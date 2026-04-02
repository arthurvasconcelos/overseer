package cmd

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update overseer to the latest release",
	RunE:  runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

const githubRepo = "arthurvasconcelos/overseer"

// updateCheckResult holds the result of the background version check.
var updateCheckResult = make(chan string, 1)

// startUpdateCheck kicks off a background goroutine that fetches the latest
// release tag. Called from root PersistentPreRun so it runs alongside every
// command. The result is consumed in PersistentPostRun.
func startUpdateCheck() {
	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		tag, err := fetchLatestTag(client)
		if err != nil {
			updateCheckResult <- ""
			return
		}
		latest := strings.TrimPrefix(tag, "v")
		current := strings.TrimPrefix(Version, "v")
		if current != "dev" && latest != current {
			updateCheckResult <- tag
		} else {
			updateCheckResult <- ""
		}
	}()
}

// printUpdateNotice reads the background check result and prints a notice if
// a newer version is available. Called from root PersistentPostRun.
func printUpdateNotice() {
	select {
	case tag := <-updateCheckResult:
		if tag != "" {
			fmt.Printf("\n\033[33mA new version is available: %s → %s\033[0m\n", Version, tag)
			fmt.Printf("\033[33mRun \033[1moverseer update\033[0m\033[33m to upgrade.\033[0m\n")
		}
	case <-time.After(100 * time.Millisecond):
		// Check didn't finish in time — skip silently.
	}
}

func runUpdate(_ *cobra.Command, _ []string) error {
	client := &http.Client{Timeout: 30 * time.Second}

	fmt.Println("checking for updates...")

	latestTag, err := fetchLatestTag(client)
	if err != nil {
		return err
	}
	latestVersion := strings.TrimPrefix(latestTag, "v")
	currentVersion := strings.TrimPrefix(Version, "v")

	if currentVersion == latestVersion {
		fmt.Printf("already up to date (%s)\n", Version)
		return nil
	}
	if currentVersion == "dev" {
		fmt.Printf("running dev build — installing %s\n", latestTag)
	} else {
		fmt.Printf("update available: %s → %s\n", Version, latestTag)
	}

	installDir := filepath.Join(os.Getenv("HOME"), "bin")
	if err := downloadAndInstall(client, latestTag, latestVersion, installDir); err != nil {
		return err
	}

	fmt.Printf("updated to %s\n", latestTag)
	return nil
}

func fetchLatestTag(client *http.Client) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %s", resp.Status)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("parsing release: %w", err)
	}
	return release.TagName, nil
}

func downloadAndInstall(client *http.Client, tag, version, installDir string) error {
	os_ := runtime.GOOS
	arch := runtime.GOARCH

	archive := fmt.Sprintf("overseer_%s_%s_%s.tar.gz", version, os_, arch)
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", githubRepo, tag, archive)

	fmt.Printf("downloading %s...\n", archive)

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("downloading binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	binary, err := extractBinary(resp.Body)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp("", "overseer-update-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(binary); err != nil {
		return err
	}
	tmp.Close()

	dest := filepath.Join(installDir, "overseer")
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0755); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), dest); err != nil {
		return copyFile(tmp.Name(), dest)
	}

	return nil
}

func extractBinary(r io.Reader) ([]byte, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("reading gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading tar: %w", err)
		}
		if hdr.Name == "overseer" || strings.HasSuffix(hdr.Name, "/overseer") {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("overseer binary not found in archive")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
