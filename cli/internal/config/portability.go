package config

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// PortabilityIssue is a value in the shared brain that only works on some machines.
type PortabilityIssue struct {
	Key   string
	Value string
	Line  int
}

// platformPathMarkers are fragments that tie a path to one OS or one user's home.
var platformPathMarkers = []string{"/Users/", "/Applications/", "/Library/", "/opt/homebrew/", "/home/", "/mnt/c/"}

// NonPortablePaths returns the absolute paths in a shared config file. Those
// differ between macOS, Linux and WSL, so they belong in a config.<platform>.yaml
// overlay or should be written relative to ~.
func NonPortablePaths(path string) ([]PortabilityIssue, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	var issues []PortabilityIssue
	walkScalars(&root, "", func(key string, n *yaml.Node) {
		if strings.HasPrefix(n.Value, "/") || (strings.HasSuffix(key, "gpg_ssh_program") && signerPlatform(n.Value) != "") {
			issues = append(issues, PortabilityIssue{Key: key, Value: n.Value, Line: n.Line})
		}
	})
	return issues, nil
}

// PlatformSpecificLines returns the uncommented lines of a text file that
// mention a platform-specific path. Binary and very large files are skipped.
func PlatformSpecificLines(path string) []int {
	info, err := os.Stat(path)
	if err != nil || info.Size() > 1<<20 {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil || bytes.IndexByte(data[:min(len(data), 8000)], 0) != -1 {
		return nil
	}
	var lines []int
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for n := 1; scanner.Scan(); n++ {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		for _, marker := range platformPathMarkers {
			if strings.Contains(line, marker) {
				lines = append(lines, n)
				break
			}
		}
	}
	return lines
}

func walkScalars(n *yaml.Node, key string, fn func(string, *yaml.Node)) {
	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			walkScalars(c, key, fn)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			child := n.Content[i].Value
			if key != "" {
				child = key + "." + child
			}
			walkScalars(n.Content[i+1], child, fn)
		}
	case yaml.SequenceNode:
		for i, c := range n.Content {
			walkScalars(c, fmt.Sprintf("%s[%d]", key, i), fn)
		}
	case yaml.ScalarNode:
		fn(key, n)
	}
}

// PortabilityFix is one change made by FixPortability.
type PortabilityFix struct {
	Key  string
	From string
	To   string // the new value, or the overlay file the key moved to
}

// FixPortability rewrites the shared brain config.yaml so it works across
// platforms: absolute paths inside the home folder become ~/..., and
// git.defaults.gpg_ssh_program moves to the config.<platform>.yaml overlay its
// value belongs to. config.yaml is edited line by line so comments and
// formatting survive. Whatever can't be fixed is left for NonPortablePaths.
func FixPortability(cfg *Config) ([]PortabilityFix, error) {
	dir := BrainOverseerPath(cfg)
	path := filepath.Join(dir, "config.yaml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	data, moved, err := moveSharedSigner(data, dir)
	if err != nil {
		return nil, err
	}
	home, _ := os.UserHomeDir()
	data, contracted, err := contractHomePaths(data, homePrefixes(home))
	if err != nil {
		return nil, err
	}

	fixes := append(moved, contracted...)
	if len(fixes) == 0 {
		return nil, nil
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return nil, fmt.Errorf("writing brain config: %w", err)
	}
	return fixes, nil
}

// homePrefixes returns the home folder plus the macOS and Linux homes of the
// same user name, so a path written on another platform is recognised too.
func homePrefixes(home string) []string {
	if home == "" {
		return nil
	}
	user := filepath.Base(home)
	prefixes := []string{home}
	for _, p := range []string{"/Users/" + user, "/home/" + user} {
		if p != home {
			prefixes = append(prefixes, p)
		}
	}
	return prefixes
}

// contractHomePaths rewrites every scalar under one of the home prefixes as ~/...
func contractHomePaths(data []byte, prefixes []string) ([]byte, []PortabilityFix, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, nil, fmt.Errorf("parsing brain config: %w", err)
	}

	type edit struct {
		line, col int
		fix       PortabilityFix
	}
	var edits []edit
	walkScalars(&root, "", func(key string, n *yaml.Node) {
		for _, prefix := range prefixes {
			rest, ok := strings.CutPrefix(n.Value, prefix)
			if !ok || (rest != "" && !strings.HasPrefix(rest, "/")) {
				continue
			}
			to := "~" + rest
			edits = append(edits, edit{n.Line, n.Column, PortabilityFix{Key: key, From: n.Value, To: to}})
			return
		}
	})

	// Several values can share a line (flow sequences), so edit right to left.
	sort.Slice(edits, func(i, j int) bool {
		if edits[i].line != edits[j].line {
			return edits[i].line < edits[j].line
		}
		return edits[i].col > edits[j].col
	})

	lines := strings.Split(string(data), "\n")
	var fixes []PortabilityFix
	for _, e := range edits {
		line := lines[e.line-1]
		start := min(e.col-1, len(line))
		idx := strings.Index(line[start:], e.fix.From)
		if idx == -1 {
			continue
		}
		at := start + idx
		lines[e.line-1] = line[:at] + e.fix.To + line[at+len(e.fix.From):]
		fixes = append(fixes, e.fix)
	}
	sort.Slice(fixes, func(i, j int) bool { return fixes[i].Key < fixes[j].Key })
	return []byte(strings.Join(lines, "\n")), fixes, nil
}

// moveSharedSigner moves git.defaults.gpg_ssh_program out of the shared config
// into config.<platform>.yaml in dir. An overlay that already sets it keeps
// its own value. Profile-level signers can't move: overlays replace the whole
// profiles list.
func moveSharedSigner(data []byte, dir string) ([]byte, []PortabilityFix, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, nil, fmt.Errorf("parsing brain config: %w", err)
	}
	defaults := mappingFind(mappingFind(docRoot(&root), "git"), "defaults")
	if defaults == nil || defaults.Kind != yaml.MappingNode {
		return data, nil, nil
	}
	var keyNode, valNode *yaml.Node
	for i := 0; i+1 < len(defaults.Content); i += 2 {
		if defaults.Content[i].Value == "gpg_ssh_program" {
			keyNode, valNode = defaults.Content[i], defaults.Content[i+1]
		}
	}
	if keyNode == nil || valNode.Line != keyNode.Line {
		return data, nil, nil
	}
	platform := signerPlatform(valNode.Value)
	if platform == "" {
		return data, nil, nil
	}

	lines := strings.Split(string(data), "\n")
	if !strings.HasPrefix(strings.TrimSpace(lines[keyNode.Line-1]), "gpg_ssh_program:") {
		return data, nil, nil
	}

	overlay := filepath.Join(dir, "config."+platform+".yaml")
	doc, err := ReadConfigNode(overlay)
	if err != nil {
		return nil, nil, err
	}
	overlayDefaults := getOrCreateMapping(getOrCreateMapping(docRoot(doc), "git"), "defaults")
	if mappingFind(overlayDefaults, "gpg_ssh_program") == nil {
		mappingSet(overlayDefaults, "gpg_ssh_program", scalarStr(valNode.Value))
		if err := WriteConfigNode(overlay, doc); err != nil {
			return nil, nil, fmt.Errorf("writing %s: %w", filepath.Base(overlay), err)
		}
	}

	lines = append(lines[:keyNode.Line-1], lines[keyNode.Line:]...)
	fix := PortabilityFix{Key: "git.defaults.gpg_ssh_program", From: valNode.Value, To: filepath.Base(overlay)}
	return []byte(strings.Join(lines, "\n")), []PortabilityFix{fix}, nil
}

// signerPlatform guesses which platform a commit signer path belongs to.
// It returns "" when the value could work anywhere.
func signerPlatform(value string) string {
	switch {
	case strings.HasPrefix(value, "/mnt/") || strings.HasSuffix(value, ".exe") || strings.HasSuffix(value, "op-ssh-sign-wsl"):
		return "wsl"
	case strings.HasPrefix(value, "/Applications/") || strings.HasPrefix(value, "/Users/") || strings.HasPrefix(value, "/opt/homebrew/"):
		return "darwin"
	case strings.HasPrefix(value, "/home/") || strings.HasPrefix(value, "/usr/") || strings.HasPrefix(value, "/opt/"):
		return "linux"
	}
	return ""
}
