package obsidian

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Vault wraps operations on a local Obsidian vault.
type Vault struct {
	Path string // absolute path to the vault root
	Name string // vault name as registered in Obsidian (used in obsidian:// URIs)
}

// New creates a Vault.
func New(path, name string) *Vault {
	return &Vault{Path: path, Name: name}
}

// Open opens a file in the Obsidian app using the obsidian:// URI scheme.
// relPath is relative to the vault root, without the .md extension.
func (v *Vault) Open(relPath string) error {
	// Normalise: strip .md suffix so Obsidian resolves it correctly.
	relPath = strings.TrimSuffix(relPath, ".md")
	uri := fmt.Sprintf("obsidian://open?vault=%s&file=%s",
		url.QueryEscape(v.Name),
		url.QueryEscape(relPath),
	)

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", uri)
	case "linux":
		cmd = exec.Command("xdg-open", uri)
	default:
		return fmt.Errorf("unsupported platform for opening Obsidian: %s", runtime.GOOS)
	}
	return cmd.Run()
}

// SearchResult is a single matching line from Search.
type SearchResult struct {
	File    string // path relative to vault root
	LineNum int
	Line    string
}

// Search returns all lines in .md files (excluding .obsidian/) matching
// query (case-insensitive substring).
func (v *Vault) Search(query string) ([]SearchResult, error) {
	lower := strings.ToLower(query)
	var results []SearchResult

	err := filepath.WalkDir(v.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			if d.Name() == ".obsidian" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(v.Path, path)
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(strings.ToLower(line), lower) {
				results = append(results, SearchResult{
					File:    rel,
					LineNum: i + 1,
					Line:    strings.TrimSpace(line),
				})
			}
		}
		return nil
	})
	return results, err
}

// ListFolders returns the top-level directories in the vault, excluding
// hidden dirs (.obsidian, .git) and the templates/meta folder.
func (v *Vault) ListFolders(exclude ...string) ([]string, error) {
	entries, err := os.ReadDir(v.Path)
	if err != nil {
		return nil, err
	}
	excludeSet := make(map[string]bool, len(exclude))
	for _, e := range exclude {
		excludeSet[e] = true
	}
	var folders []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if excludeSet[e.Name()] {
			continue
		}
		folders = append(folders, e.Name())
	}
	return folders, nil
}

// ListTemplates returns template filenames (without .md) in the given folder.
func (v *Vault) ListTemplates(folder string) ([]string, error) {
	dir := filepath.Join(v.Path, folder)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".md" {
			names = append(names, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	return names, nil
}

// ReadTemplate reads a template file and returns its content.
func (v *Vault) ReadTemplate(folder, name string) (string, error) {
	path := filepath.Join(v.Path, folder, name+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// CreateNote writes content to folder/filename.md (creates folder if needed).
// Returns the absolute path.
func (v *Vault) CreateNote(folder, filename, content string) (string, error) {
	dir := v.Path
	if folder != "" {
		dir = filepath.Join(dir, folder)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, filename+".md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}

// NoteExists returns true if folder/filename.md already exists.
func (v *Vault) NoteExists(folder, filename string) bool {
	path := filepath.Join(v.Path, folder, filename+".md")
	_, err := os.Stat(path)
	return err == nil
}

// RelPath returns the path of folder/filename relative to the vault root.
func (v *Vault) RelPath(folder, filename string) string {
	if folder == "" {
		return filename
	}
	return filepath.Join(folder, filename)
}

// --- Daily note generation ---

// DailyFilename returns today's daily note filename (without .md).
func DailyFilename() string {
	return time.Now().Format("2006-01-02")
}

// GenerateDailyContent produces daily note content matching the vault's
// established format (frontmatter + section structure).
func GenerateDailyContent() string {
	now := time.Now()
	date := now.Format("2006-01-02") + "T" + now.Format("15:04")
	weekday := strings.ToLower(now.Weekday().String())
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	tomorrow := now.AddDate(0, 0, 1).Format("2006-01-02")
	heading := fmt.Sprintf("%s, %s %s, %d",
		now.Weekday().String(),
		now.Month().String(),
		ordinal(now.Day()),
		now.Year(),
	)

	return fmt.Sprintf(`---
date: %s
tags:
  - Daily
cssclasses:
  - daily
  - %s
yesterday: "[[%s]]"
tomorrow: "[[%s]]"
---
# DAILY NOTE
## %s
***
### Journal

***
### Tasks

***
### Learning
#### TIL
-

#### Notes
-

***
### Review
`+"```"+"dataview\nLIST\nFROM \"03 - Resources\" OR \"02 - Areas/02 - Learning\"\nWHERE type = \"concept\"\n  AND date(last-reviewed) + dur(review-interval + \" days\") <= date(today)\nSORT last-reviewed ASC\n```",
		date, weekday, yesterday, tomorrow, heading,
	)
}

// --- Template rendering ---

var templateValueRe = regexp.MustCompile(`\{\{VALUE:([^}]+)\}\}`)
var templaterDateRe = regexp.MustCompile(`<%[^%]*tp\.date[^%]*%>`)

// ExtractTemplateValues returns the unique {{VALUE:key}} keys in a template.
func ExtractTemplateValues(content string) []string {
	seen := make(map[string]bool)
	var keys []string
	for _, match := range templateValueRe.FindAllStringSubmatch(content, -1) {
		key := match[1]
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	return keys
}

// RenderTemplate substitutes {{VALUE:key}} with provided values and
// replaces Templater date expressions with today's date.
func RenderTemplate(content string, values map[string]string) string {
	// Replace Templater date expressions with today's date.
	today := time.Now().Format("2006-01-02")
	content = templaterDateRe.ReplaceAllStringFunc(content, func(match string) string {
		if strings.Contains(match, "yesterday") {
			return time.Now().AddDate(0, 0, -1).Format("2006-01-02")
		}
		if strings.Contains(match, "tomorrow") {
			return time.Now().AddDate(0, 0, 1).Format("2006-01-02")
		}
		return today
	})
	// Replace {{VALUE:key}} with user-provided values.
	return templateValueRe.ReplaceAllStringFunc(content, func(match string) string {
		sub := templateValueRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		if v, ok := values[sub[1]]; ok {
			return v
		}
		return ""
	})
}

func ordinal(n int) string {
	suffix := "th"
	switch {
	case n%10 == 1 && n != 11:
		suffix = "st"
	case n%10 == 2 && n != 12:
		suffix = "nd"
	case n%10 == 3 && n != 13:
		suffix = "rd"
	}
	return strconv.Itoa(n) + suffix
}
