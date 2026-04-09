package cmd

import (
	"fmt"
	"strings"

	"github.com/arthurvasconcelos/overseer/internal/config"
	obsidianpkg "github.com/arthurvasconcelos/overseer/internal/obsidian"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

var noteCmd = &cobra.Command{
	Use:   "note",
	Short: "Obsidian vault — daily notes, create, search",
}

var noteDailyCmd = &cobra.Command{
	Use:   "daily",
	Short: "Open or create today's daily note",
	RunE:  runNoteDaily,
}

var noteNewCmd = &cobra.Command{
	Use:   "new [title]",
	Short: "Create a new note",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runNoteNew,
}

var noteSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search the vault",
	Args:  cobra.ExactArgs(1),
	RunE:  runNoteSearch,
}

func init() {
	noteCmd.AddCommand(noteDailyCmd)
	noteCmd.AddCommand(noteNewCmd)
	noteCmd.AddCommand(noteSearchCmd)
	rootCmd.AddCommand(noteCmd)
}

// vaultFromConfig resolves the vault path and returns a Vault instance.
func vaultFromConfig(cfg *config.Config) (*obsidianpkg.Vault, error) {
	ocfg := cfg.Obsidian
	if ocfg.VaultPath == "" {
		return nil, fmt.Errorf("obsidian.vault_path not configured — add it to config.yaml")
	}
	path := repoRoot(resolveReposPath(cfg), ocfg.VaultPath)
	name := ocfg.VaultName
	if name == "" {
		// Fall back to directory basename.
		name = vaultBasename(path)
	}
	return obsidianpkg.New(path, name), nil
}

func vaultBasename(path string) string {
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	return parts[len(parts)-1]
}

// --- daily ---

func runNoteDaily(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	vault, err := vaultFromConfig(cfg)
	if err != nil {
		return err
	}

	folder := cfg.Obsidian.DailyNotesFolder
	filename := obsidianpkg.DailyFilename()

	if vault.NoteExists(folder, filename) {
		fmt.Println(tui.StyleMuted.Render("opening existing daily note: ") + tui.StyleAccent.Render(filename))
	} else {
		content := obsidianpkg.GenerateDailyContent()
		if _, err := vault.CreateNote(folder, filename, content); err != nil {
			return fmt.Errorf("creating daily note: %w", err)
		}
		fmt.Println(tui.StyleOK.Render("✓") + "  created " + tui.StyleAccent.Render(filename))
	}

	return vault.Open(vault.RelPath(folder, filename))
}

// --- new ---

func runNoteNew(_ *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	vault, err := vaultFromConfig(cfg)
	if err != nil {
		return err
	}
	ocfg := cfg.Obsidian

	// Title.
	title := ""
	if len(args) > 0 {
		title = args[0]
	} else {
		title, err = tui.Prompt("note title", "", "")
		if err != nil {
			return err
		}
		fmt.Println()
	}
	if title == "" {
		return fmt.Errorf("title cannot be empty")
	}

	// Folder picker.
	folder, err := pickNoteFolder(vault, ocfg)
	if err != nil {
		return err
	}
	fmt.Println()

	// Template picker (optional).
	content, err := pickTemplate(vault, ocfg, title)
	if err != nil {
		return err
	}
	if content != "" {
		fmt.Println()
	}

	filename := sanitizeFilename(title)
	if vault.NoteExists(folder, filename) {
		fmt.Println(tui.StyleMuted.Render("note already exists, opening: ") + tui.StyleAccent.Render(filename))
		return vault.Open(vault.RelPath(folder, filename))
	}

	if _, err := vault.CreateNote(folder, filename, content); err != nil {
		return fmt.Errorf("creating note: %w", err)
	}
	fmt.Println(tui.StyleOK.Render("✓") + "  created " + tui.StyleAccent.Render(folder+"/"+filename))
	return vault.Open(vault.RelPath(folder, filename))
}

func pickNoteFolder(vault *obsidianpkg.Vault, ocfg config.ObsidianConfig) (string, error) {
	folders, err := vault.ListFolders(ocfg.TemplatesFolder, ocfg.DailyNotesFolder)
	if err != nil || len(folders) == 0 {
		return ocfg.DefaultFolder, nil
	}

	items := make([]tui.SelectItem, 0, len(folders)+1)
	if ocfg.DefaultFolder == "" {
		items = append(items, tui.SelectItem{Title: "/ (vault root)"})
	}
	for _, f := range folders {
		items = append(items, tui.SelectItem{Title: f})
	}

	idx, err := tui.Select("select folder", items)
	if err != nil || idx == -1 {
		return ocfg.DefaultFolder, err
	}
	chosen := items[idx].Title
	if chosen == "/ (vault root)" {
		return "", nil
	}
	return chosen, nil
}

func pickTemplate(vault *obsidianpkg.Vault, ocfg config.ObsidianConfig, title string) (string, error) {
	if ocfg.TemplatesFolder == "" {
		return defaultNoteContent(title), nil
	}

	names, err := vault.ListTemplates(ocfg.TemplatesFolder)
	if err != nil || len(names) == 0 {
		return defaultNoteContent(title), nil
	}

	items := []tui.SelectItem{{Title: "none", Subtitle: "blank note with frontmatter"}}
	for _, n := range names {
		items = append(items, tui.SelectItem{Title: n})
	}

	idx, err := tui.Select("select template", items)
	if err != nil || idx == -1 {
		return defaultNoteContent(title), err
	}
	if idx == 0 {
		return defaultNoteContent(title), nil
	}

	tmplName := names[idx-1]
	raw, err := vault.ReadTemplate(ocfg.TemplatesFolder, tmplName)
	if err != nil {
		return "", fmt.Errorf("reading template %q: %w", tmplName, err)
	}

	// Prompt for any {{VALUE:key}} variables in the template.
	keys := obsidianpkg.ExtractTemplateValues(raw)
	values := make(map[string]string, len(keys))
	for _, k := range keys {
		fmt.Println()
		val, err := tui.Prompt(k, "", "")
		if err != nil {
			return "", err
		}
		values[k] = val
	}

	return obsidianpkg.RenderTemplate(raw, values), nil
}

func defaultNoteContent(title string) string {
	return fmt.Sprintf("# %s\n", title)
}

func sanitizeFilename(title string) string {
	// Replace characters that are invalid in filenames.
	r := strings.NewReplacer(
		"/", "-", "\\", "-", ":", "-", "*", "-",
		"?", "", "\"", "", "<", "", ">", "", "|", "-",
	)
	return strings.TrimSpace(r.Replace(title))
}

// --- search ---

func runNoteSearch(_ *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	vault, err := vaultFromConfig(cfg)
	if err != nil {
		return err
	}

	query := args[0]
	results, err := vault.Search(query)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	if len(results) == 0 {
		fmt.Println(tui.StyleMuted.Render("no results for: " + query))
		return nil
	}

	fmt.Println(tui.SectionHeader("results", fmt.Sprintf("%d matches", len(results))))
	fmt.Println()

	prevFile := ""
	for _, r := range results {
		if r.File != prevFile {
			fmt.Println("  " + tui.StyleAccent.Render(r.File))
			prevFile = r.File
		}
		lineNum := tui.StyleMuted.Render(fmt.Sprintf("%4d", r.LineNum))
		line := tui.StyleNormal.Render(truncateSearch(r.Line, 90))
		fmt.Printf("  %s  %s\n", lineNum, line)
	}
	return nil
}

func truncateSearch(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
