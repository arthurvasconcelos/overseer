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

	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/charmbracelet/bubbles/progress"
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

func trimV(v string) string { return strings.TrimPrefix(v, "v") }

func runUpdate(_ *cobra.Command, _ []string) error {
	client := &http.Client{Timeout: 30 * time.Second}

	fmt.Println(tui.StyleMuted.Render("checking for updates..."))

	latestTag, err := fetchLatestTag(client, strings.Contains(Version, "-"))
	if err != nil {
		return err
	}
	latestVersion := trimV(latestTag)
	currentVersion := trimV(Version)

	if currentVersion == latestVersion {
		fmt.Println(tui.StyleOK.Render("✓") + "  already up to date " + tui.StyleMuted.Render("("+Version+")"))
		return nil
	}
	if currentVersion == "dev" {
		fmt.Println(tui.StyleDim.Render("running dev build — installing " + latestTag))
	} else {
		fmt.Println(tui.StyleWarn.Render("update available: "+Version+" → "+latestTag))
	}

	installDir := filepath.Join(os.Getenv("HOME"), "bin")
	if err := downloadAndInstall(client, latestTag, latestVersion, installDir); err != nil {
		return err
	}

	fmt.Println(tui.StyleOK.Render("✓") + "  updated to " + tui.StyleAccent.Render(latestTag))
	return nil
}

func fetchLatestTag(client *http.Client, prerelease bool) (string, error) {
	if !prerelease {
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

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=10", githubRepo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %s", resp.Status)
	}

	var releases []struct {
		TagName    string `json:"tag_name"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return "", fmt.Errorf("parsing releases: %w", err)
	}
	for _, r := range releases {
		if r.Prerelease {
			return r.TagName, nil
		}
	}
	return "", fmt.Errorf("no beta release found on GitHub")
}

// progressReader wraps an io.Reader and calls onRead after every read with the
// cumulative bytes read and the total size (may be -1 if unknown).
type progressReader struct {
	r      io.Reader
	total  int64
	read   int64
	onRead func(read, total int64)
}

func (pr *progressReader) Read(p []byte) (n int, err error) {
	n, err = pr.r.Read(p)
	pr.read += int64(n)
	if pr.onRead != nil {
		pr.onRead(pr.read, pr.total)
	}
	return
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func downloadAndInstall(client *http.Client, tag, version, installDir string) error {
	os_ := runtime.GOOS
	arch := runtime.GOARCH

	archive := fmt.Sprintf("overseer_%s_%s_%s.tar.gz", version, os_, arch)
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", githubRepo, tag, archive)

	fmt.Printf("downloading %s\n", archive)

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("downloading binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	prog := progress.New(progress.WithDefaultGradient(), progress.WithWidth(50))
	pr := &progressReader{
		r:     resp.Body,
		total: resp.ContentLength,
		onRead: func(read, total int64) {
			if total > 0 {
				bar := prog.ViewAs(float64(read) / float64(total))
				fmt.Printf("\r%s  %s / %s", bar, formatBytes(read), formatBytes(total))
			} else {
				fmt.Printf("\r%s downloaded", formatBytes(read))
			}
		},
	}

	binary, err := extractBinary(pr)
	fmt.Println()
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
