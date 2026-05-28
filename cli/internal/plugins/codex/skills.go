package codex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

type skillInfo struct {
	Name        string
	Description string
	SourceDir   string
	FromBrain   bool
	Active      bool // symlink exists in ~/.codex/skills/ pointing to SourceDir
	Disabled    bool // in disabled list in state file
}

type skillsState struct {
	Disabled []string `json:"disabled,omitempty"`
}

func skillsCmd(cfg *config.Config) *cobra.Command {
	root := &cobra.Command{
		Use:   "skills",
		Short: "Manage Codex skills",
	}
	root.AddCommand(skillsListPathCmd(cfg))
	root.AddCommand(skillsAddPathCmd(cfg))
	root.AddCommand(skillsRemovePathCmd(cfg))
	root.AddCommand(skillsSyncCmd(cfg))
	root.AddCommand(skillsListCmd(cfg))
	root.AddCommand(skillsEnableCmd(cfg))
	root.AddCommand(skillsDisableCmd(cfg))
	return root
}

func skillsListCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all available skills with status",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runSkillsList(cfg)
		},
	}
}

func skillsAddPathCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "add-path <path>",
		Short: "Add a directory to skill search paths",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runSkillsAddPath(cfg, args[0])
		},
	}
}

func skillsListPathCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "list-path",
		Short: "List all registered skill search paths",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runSkillsListPath(cfg)
		},
	}
}

func skillsRemovePathCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "remove-path [path]",
		Short: "Remove a directory from skill search paths (interactive select when no arg given)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return runSkillsRemovePathInteractive(cfg)
			}
			return runSkillsRemovePath(cfg, args[0])
		},
	}
}

func skillsEnableCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "enable [name...]",
		Short: "Enable skills (interactive multi-select when no args given)",
		Args:  cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return runSkillsEnableInteractive(cfg)
			}
			for _, name := range args {
				if err := runSkillsEnable(cfg, name); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func skillsDisableCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "disable [name...]",
		Short: "Disable skills (interactive multi-select when no args given)",
		Args:  cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return runSkillsDisableInteractive(cfg)
			}
			for _, name := range args {
				if err := runSkillsDisable(cfg, name); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func skillsSyncCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Sync skill symlinks to match enabled/disabled state",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runSkillsSync(cfg)
		},
	}
}

// --- state file ---

func skillsStateFile(codexDir string) string {
	return filepath.Join(codexDir, "skills-state.json")
}

func readSkillsState(codexDir string) skillsState {
	data, err := os.ReadFile(skillsStateFile(codexDir))
	if err != nil {
		return skillsState{}
	}
	var state skillsState
	_ = json.Unmarshal(data, &state)
	return state
}

func writeSkillsState(codexDir string, state skillsState) error {
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(skillsStateFile(codexDir), append(data, '\n'), 0o644)
}

func isDisabled(state skillsState, name string) bool {
	for _, d := range state.Disabled {
		if d == name {
			return true
		}
	}
	return false
}

func setDisabled(state *skillsState, name string, disabled bool) {
	if disabled {
		if !isDisabled(*state, name) {
			state.Disabled = append(state.Disabled, name)
			sort.Strings(state.Disabled)
		}
		return
	}
	var filtered []string
	for _, d := range state.Disabled {
		if d != name {
			filtered = append(filtered, d)
		}
	}
	state.Disabled = filtered
}

// --- discovery ---

func discoverSkills(cfg *config.Config) []skillInfo {
	codexDir := brainCodexDir(cfg)
	brainSkillsDir := filepath.Join(codexDir, "skills")
	home, _ := os.UserHomeDir()
	localSkillsDir := filepath.Join(home, ".codex", "skills")
	state := readSkillsState(codexDir)

	seen := map[string]bool{}
	var skills []skillInfo

	if entries, err := os.ReadDir(brainSkillsDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			name := entry.Name()
			seen[name] = true
			srcDir := filepath.Join(brainSkillsDir, name)
			skills = append(skills, skillInfo{
				Name:        name,
				Description: readSkillDescription(srcDir),
				SourceDir:   srcDir,
				FromBrain:   true,
				Active:      isLinkedTo(filepath.Join(localSkillsDir, name), srcDir),
				Disabled:    isDisabled(state, name),
			})
		}
	}

	for _, searchPath := range skillSearchPaths(cfg) {
		expanded := expandTilde(searchPath)
		entries, err := os.ReadDir(expanded)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			name := entry.Name()
			if seen[name] {
				continue
			}
			seen[name] = true
			srcDir := filepath.Join(expanded, name)
			skills = append(skills, skillInfo{
				Name:        name,
				Description: readSkillDescription(srcDir),
				SourceDir:   srcDir,
				FromBrain:   false,
				Active:      isLinkedTo(filepath.Join(localSkillsDir, name), srcDir),
				Disabled:    isDisabled(state, name),
			})
		}
	}

	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills
}

func skillSearchPaths(cfg *config.Config) []string {
	if cfg.Integrations.Codex == nil {
		return nil
	}
	return cfg.Integrations.Codex.SkillSearchPaths
}

func isLinkedTo(linkPath, targetPath string) bool {
	current, err := os.Readlink(linkPath)
	return err == nil && current == targetPath
}

func expandTilde(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[2:])
}

func readSkillDescription(skillDir string) string {
	entries, err := os.ReadDir(skillDir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if desc := extractFrontmatterField(filepath.Join(skillDir, entry.Name()), "description"); desc != "" {
			return desc
		}
	}
	return ""
}

func extractFrontmatterField(filePath, field string) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return ""
	}
	rest := content[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return ""
	}
	prefix := field + ":"
	for _, line := range strings.Split(rest[:end], "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			val := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			return strings.Trim(val, `"'`)
		}
	}
	return ""
}

// --- runners ---

func runSkillsAddPath(cfg *config.Config, path string) error {
	abs, err := filepath.Abs(expandTilde(path))
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}
	if !pathExists(abs) {
		return fmt.Errorf("path does not exist: %s", abs)
	}

	// Auto-detect a skills/ subdirectory: if the given path has one, use it instead.
	skillsSubdir := filepath.Join(abs, "skills")
	if pathExists(skillsSubdir) {
		fmt.Printf("  %s  %s\n",
			tui.StyleDim.Render("detect"),
			tui.StyleDim.Render("found skills/ subdir — using "+skillsSubdir),
		)
		abs = skillsSubdir
	}

	added, err := config.WriteBrainCodexSkillSearchPath(cfg, abs)
	if err != nil {
		return err
	}
	if !added {
		fmt.Printf("  %s  %s\n", tui.StyleMuted.Render("skip  "), tui.StyleDim.Render(abs+" (already in search paths)"))
		return nil
	}
	fmt.Printf("  %s  %s\n", tui.StyleOK.Render("added "), tui.StyleNormal.Render(abs))

	// Preview discovered skills.
	discovered := skillsInDir(abs)
	if len(discovered) > 0 {
		fmt.Println()
		fmt.Printf("  %s\n", tui.StyleDim.Render(fmt.Sprintf("%d skill(s) available — use 'overseer codex skills enable <name>' to activate:", len(discovered))))
		for _, name := range discovered {
			fmt.Printf("    %s  %s\n", tui.StyleMuted.Render("·"), tui.StyleNormal.Render(name))
		}
	}
	return nil
}

func runSkillsListPath(cfg *config.Config) error {
	paths := skillSearchPaths(cfg)
	if len(paths) == 0 {
		fmt.Println("  " + tui.StyleMuted.Render("no search paths configured"))
		return nil
	}
	for _, p := range paths {
		fmt.Printf("  %s  %s\n", tui.StyleDim.Render("·"), tui.StyleNormal.Render(p))
	}
	return nil
}

func runSkillsRemovePathInteractive(cfg *config.Config) error {
	paths := skillSearchPaths(cfg)
	if len(paths) == 0 {
		fmt.Println("  " + tui.StyleMuted.Render("no search paths configured"))
		return nil
	}

	items := make([]tui.SelectItem, len(paths))
	for i, p := range paths {
		items[i] = tui.SelectItem{Title: p}
	}

	idx, err := tui.Select("Select search path to remove", items)
	if err != nil {
		return err
	}
	if idx < 0 {
		return nil
	}
	return runSkillsRemovePath(cfg, paths[idx])
}

func runSkillsRemovePath(cfg *config.Config, path string) error {
	abs, err := filepath.Abs(expandTilde(path))
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	removed, err := config.WriteBrainCodexRemoveSkillSearchPath(cfg, abs)
	if err != nil {
		return err
	}
	if !removed {
		return fmt.Errorf("path %q not found in skill_search_paths", abs)
	}
	fmt.Printf("  %s  %s\n", tui.StyleOK.Render("removed"), tui.StyleNormal.Render(abs))

	// Clean up any symlinks in ~/.codex/skills/ that pointed into this path.
	home, _ := os.UserHomeDir()
	localSkillsDir := filepath.Join(home, ".codex", "skills")
	entries, err := os.ReadDir(localSkillsDir)
	if err != nil {
		return nil
	}
	cleaned := 0
	for _, entry := range entries {
		dst := filepath.Join(localSkillsDir, entry.Name())
		target, err := os.Readlink(dst)
		if err != nil {
			continue
		}
		if strings.HasPrefix(target, abs+string(os.PathSeparator)) || target == abs {
			if err := os.Remove(dst); err != nil {
				fmt.Printf("  %s  %s  %s\n", tui.StyleWarn.Render("warn  "), tui.StyleNormal.Render(entry.Name()), tui.StyleDim.Render(err.Error()))
				continue
			}
			fmt.Printf("  %s  %s\n", tui.StyleMuted.Render("unlink"), tui.StyleNormal.Render(entry.Name()))
			cleaned++
		}
	}
	if cleaned == 0 {
		fmt.Println("  " + tui.StyleDim.Render("no symlinks to clean up"))
	}
	return nil
}

func runSkillsList(cfg *config.Config) error {
	skills := discoverSkills(cfg)
	codexDir := brainCodexDir(cfg)

	fmt.Println(tui.SectionHeader("codex skills", codexDir))
	fmt.Println()

	if len(skills) == 0 {
		fmt.Println("  " + tui.StyleMuted.Render("no skills found"))
		fmt.Println()
		fmt.Println("  " + tui.StyleDim.Render("Add skills to "+filepath.Join(codexDir, "skills")+" or run: overseer codex skills add <path>"))
		return nil
	}

	maxName := 0
	for _, s := range skills {
		if len(s.Name) > maxName {
			maxName = len(s.Name)
		}
	}

	maxSrc := 0
	for _, s := range skills {
		n := len(skillSourceLabel(s))
		if n > maxSrc {
			maxSrc = n
		}
	}

	for _, s := range skills {
		var icon string
		switch {
		case s.Disabled:
			icon = tui.StyleMuted.Render("○")
		case s.Active:
			icon = tui.StyleOK.Render("✓")
		case s.FromBrain:
			icon = tui.StyleWarn.Render("!")
		default:
			icon = tui.StyleDim.Render("·")
		}

		srcLabel := skillSourceLabel(s)
		var src string
		if s.FromBrain {
			src = tui.StyleAccent.Render(srcLabel)
		} else {
			src = tui.StyleDim.Render(srcLabel)
		}

		namePad := strings.Repeat(" ", maxName-len(s.Name)+2)
		srcPad := strings.Repeat(" ", maxSrc-len(srcLabel)+2)

		var desc string
		if s.Description == "" {
			desc = tui.StyleMuted.Render("(no description)")
		} else {
			desc = tui.StyleDim.Render(truncateStr(s.Description, 60))
		}

		fmt.Printf("  %s  %s%s%s%s%s\n", icon, tui.StyleNormal.Render(s.Name), namePad, src, srcPad, desc)
	}

	fmt.Println()
	active, needsSync, available := 0, 0, 0
	for _, s := range skills {
		switch {
		case s.Active:
			active++
		case s.Disabled:
			// skip
		case s.FromBrain:
			needsSync++
		default:
			available++
		}
	}
	fmt.Print("  " + tui.StyleDim.Render(fmt.Sprintf("%d/%d active", active, len(skills))))
	if needsSync > 0 {
		fmt.Print("  " + tui.StyleWarn.Render(fmt.Sprintf("%d unsynced", needsSync)) +
			tui.StyleMuted.Render(" — run: overseer codex skills sync"))
	}
	if available > 0 {
		fmt.Print("  " + tui.StyleDim.Render(fmt.Sprintf("%d external available", available)) +
			tui.StyleMuted.Render(" — use: overseer codex skills enable <name>"))
	}
	fmt.Println()

	return nil
}

func runSkillsEnableInteractive(cfg *config.Config) error {
	skills := discoverSkills(cfg)
	var candidates []skillInfo
	for _, s := range skills {
		if !s.Active && !s.Disabled {
			candidates = append(candidates, s)
		}
	}
	if len(candidates) == 0 {
		fmt.Println("  " + tui.StyleMuted.Render("no skills available to enable"))
		return nil
	}

	items := make([]tui.SelectItem, len(candidates))
	for i, s := range candidates {
		items[i] = tui.SelectItem{Title: skillSelectLabel(s, candidates)}
	}

	chosen, err := tui.MultiSelect("Select skills to enable", items)
	if err != nil {
		return err
	}
	if len(chosen) == 0 {
		return nil
	}
	for _, idx := range chosen {
		if err := runSkillsEnable(cfg, candidates[idx].Name); err != nil {
			return err
		}
	}
	return nil
}

func runSkillsDisableInteractive(cfg *config.Config) error {
	skills := discoverSkills(cfg)
	var candidates []skillInfo
	for _, s := range skills {
		if s.Active {
			candidates = append(candidates, s)
		}
	}
	if len(candidates) == 0 {
		fmt.Println("  " + tui.StyleMuted.Render("no active skills to disable"))
		return nil
	}

	items := make([]tui.SelectItem, len(candidates))
	for i, s := range candidates {
		items[i] = tui.SelectItem{Title: skillSelectLabel(s, candidates)}
	}

	chosen, err := tui.MultiSelect("Select skills to disable", items)
	if err != nil {
		return err
	}
	if len(chosen) == 0 {
		return nil
	}
	for _, idx := range chosen {
		if err := runSkillsDisable(cfg, candidates[idx].Name); err != nil {
			return err
		}
	}
	return nil
}

func runSkillsEnable(cfg *config.Config, name string) error {
	codexDir := brainCodexDir(cfg)
	state := readSkillsState(codexDir)

	skills := discoverSkills(cfg)
	var found *skillInfo
	for i := range skills {
		if skills[i].Name == name {
			found = &skills[i]
			break
		}
	}
	if found == nil {
		return fmt.Errorf("skill %q not found — run 'overseer codex skills list' to see available skills", name)
	}

	setDisabled(&state, name, false)
	if err := writeSkillsState(codexDir, state); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	home, _ := os.UserHomeDir()
	localSkillsDir := filepath.Join(home, ".codex", "skills")
	if err := os.MkdirAll(localSkillsDir, 0o755); err != nil {
		return err
	}

	return makeLink(found.SourceDir, filepath.Join(localSkillsDir, name), false, name)
}

func runSkillsDisable(cfg *config.Config, name string) error {
	codexDir := brainCodexDir(cfg)
	state := readSkillsState(codexDir)

	skills := discoverSkills(cfg)
	found := false
	for _, s := range skills {
		if s.Name == name {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("skill %q not found — run 'overseer codex skills list' to see available skills", name)
	}

	setDisabled(&state, name, true)
	if err := writeSkillsState(codexDir, state); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	home, _ := os.UserHomeDir()
	dst := filepath.Join(home, ".codex", "skills", name)
	info, err := os.Lstat(dst)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(dst); err != nil {
			return fmt.Errorf("removing symlink: %w", err)
		}
		fmt.Printf("  %s  %s\n", tui.StyleMuted.Render("unlink"), tui.StyleNormal.Render(name))
	} else {
		fmt.Printf("  %s  %s\n", tui.StyleMuted.Render("skip  "), tui.StyleDim.Render(name+" (no symlink to remove)"))
	}

	return nil
}

func runSkillsSync(cfg *config.Config) error {
	codexDir := brainCodexDir(cfg)
	skills := discoverSkills(cfg)
	state := readSkillsState(codexDir)

	home, _ := os.UserHomeDir()
	localSkillsDir := filepath.Join(home, ".codex", "skills")

	if len(skills) > 0 {
		if err := os.MkdirAll(localSkillsDir, 0o755); err != nil {
			return err
		}
	}

	externalAvailable := 0
	for _, s := range skills {
		dst := filepath.Join(localSkillsDir, s.Name)
		if isDisabled(state, s.Name) {
			info, err := os.Lstat(dst)
			if err == nil && info.Mode()&os.ModeSymlink != 0 {
				if err := os.Remove(dst); err != nil {
					return fmt.Errorf("removing symlink for %s: %w", s.Name, err)
				}
				fmt.Printf("  %s  %s\n", tui.StyleMuted.Render("unlink"), tui.StyleNormal.Render(s.Name))
			} else {
				fmt.Printf("  %s  %s\n", tui.StyleMuted.Render("skip  "), tui.StyleDim.Render(s.Name+" (disabled)"))
			}
		} else if s.FromBrain {
			if err := makeLink(s.SourceDir, dst, false, s.Name); err != nil {
				return err
			}
		} else {
			// External skills are not auto-created; only symlinks already in place are preserved.
			if !s.Active {
				externalAvailable++
			}
		}
	}

	// Warn about symlinks in ~/.codex/skills/ not managed by overseer.
	if entries, err := os.ReadDir(localSkillsDir); err == nil {
		for _, entry := range entries {
			known := false
			for _, s := range skills {
				if s.Name == entry.Name() {
					known = true
					break
				}
			}
			if known {
				continue
			}
			dst := filepath.Join(localSkillsDir, entry.Name())
			info, err := os.Lstat(dst)
			if err == nil && info.Mode()&os.ModeSymlink != 0 {
				fmt.Printf("  %s  %s  %s\n",
					tui.StyleWarn.Render("warn  "),
					tui.StyleNormal.Render(entry.Name()),
					tui.StyleDim.Render("not managed by overseer — skipping"),
				)
			}
		}
	}

	if externalAvailable > 0 {
		fmt.Println()
		fmt.Printf("  %s  %s\n",
			tui.StyleDim.Render(fmt.Sprintf("%d external skill(s) available but not enabled", externalAvailable)),
			tui.StyleMuted.Render("— use: overseer codex skills enable <name>"),
		)
	}

	return nil
}

// skillSelectLabel builds a padded, aligned plain-text label for use in multi-select
// prompts. No lipgloss renders are used inside the label: huh passes option labels
// through its own lipgloss pipeline, which treats interior spaces as word-break
// points and collapses alignment padding. Plain text avoids this entirely.
func skillSelectLabel(s skillInfo, peers []skillInfo) string {
	maxName, maxSrc := 0, 0
	for _, p := range peers {
		if len(p.Name) > maxName {
			maxName = len(p.Name)
		}
		if n := len(skillSourceLabel(p)); n > maxSrc {
			maxSrc = n
		}
	}
	srcLabel := skillSourceLabel(s)
	namePad := strings.Repeat(" ", maxName-len(s.Name)+2)
	srcPad := strings.Repeat(" ", maxSrc-len(srcLabel)+2)
	label := s.Name + namePad + srcLabel + srcPad
	if s.Description != "" {
		label += truncateStr(s.Description, 55)
	}
	return label
}

// skillSourceLabel returns a short display label for a skill's source.
func skillSourceLabel(s skillInfo) string {
	if s.FromBrain {
		return "brain"
	}
	return shortPath(filepath.Dir(s.SourceDir))
}

// shortPath returns the last two path components joined with "/", e.g.:
// /home/user/repos/aiconf/skills → aiconf/skills
func shortPath(path string) string {
	base := filepath.Base(path)
	parent := filepath.Base(filepath.Dir(path))
	if parent == "." || parent == "/" {
		return base
	}
	return parent + "/" + base
}

// truncateStr shortens s to at most n runes, appending "…" if trimmed.
func truncateStr(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}

// skillsInDir returns the names of skill directories inside dir (non-hidden dirs only).
func skillsInDir(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			names = append(names, entry.Name())
		}
	}
	return names
}
