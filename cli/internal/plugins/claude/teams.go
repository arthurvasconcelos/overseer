package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	claudeaiclient "github.com/arthurvasconcelos/overseer/internal/claudeai"
	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

// teamMeta is the parsed content of brain/claude/teams/<slug>/team.yaml.
type teamMeta struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Lead        string   `yaml:"lead,omitempty"` // who the personas are advising, e.g. "Arthur, Tech Lead"
	Personas    []string `yaml:"personas"`
}

type persona struct {
	slug   string // file base name, e.g. "senior-be"
	prompt string // content of <slug>.md
}

type loadedTeam struct {
	meta     teamMeta
	personas []persona
}

// teamsRoot returns the absolute path to brain/claude/teams/.
func teamsRoot(cfg *config.Config) string {
	return filepath.Join(brainClaudeDir(cfg), "teams")
}

// listTeamMetas scans teamsRoot for subdirectories containing a team.yaml.
func listTeamMetas(cfg *config.Config) ([]teamMeta, error) {
	root := teamsRoot(cfg)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []teamMeta
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, e.Name(), "team.yaml"))
		if err != nil {
			continue
		}
		var m teamMeta
		if err := yaml.Unmarshal(data, &m); err != nil {
			continue
		}
		if m.Name == "" {
			m.Name = e.Name()
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// loadTeam loads a team by slug (the directory name under teamsRoot).
func loadTeam(cfg *config.Config, slug string) (*loadedTeam, error) {
	dir := filepath.Join(teamsRoot(cfg), slug)
	data, err := os.ReadFile(filepath.Join(dir, "team.yaml"))
	if err != nil {
		return nil, fmt.Errorf("team %q: %w", slug, err)
	}
	var meta teamMeta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("team %q: parsing team.yaml: %w", slug, err)
	}
	if meta.Name == "" {
		meta.Name = slug
	}
	var personas []persona
	for _, p := range meta.Personas {
		content, err := os.ReadFile(filepath.Join(dir, p+".md"))
		if err != nil {
			return nil, fmt.Errorf("team %q: persona %q: %w", slug, p, err)
		}
		personas = append(personas, persona{slug: p, prompt: string(content)})
	}
	return &loadedTeam{meta: meta, personas: personas}, nil
}

// personaDisplayName converts a slug like "senior-be" to "Senior BE".
// Words of 1-2 characters are uppercased; longer words are title-cased.
func personaDisplayName(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		runes := []rune(p)
		if len(runes) <= 2 {
			parts[i] = strings.ToUpper(p)
		} else {
			runes[0] = unicode.ToUpper(runes[0])
			parts[i] = string(runes)
		}
	}
	return strings.Join(parts, " ")
}

// teamsCmd builds the `claude teams` parent command with its subcommands.
func teamsCmd(cfg *config.Config) *cobra.Command {
	root := &cobra.Command{
		Use:   "teams",
		Short: "Manage Claude team personas",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runTeamsList(cfg)
		},
	}
	root.AddCommand(teamsUseCmd(cfg))
	root.AddCommand(teamsConsultCmd(cfg))
	return root
}

// --- teams (list) ---

func runTeamsList(cfg *config.Config) error {
	metas, err := listTeamMetas(cfg)
	if err != nil {
		return err
	}

	fmt.Println(tui.SectionHeader("claude teams", teamsRoot(cfg)))
	fmt.Println()

	if len(metas) == 0 {
		fmt.Println("  " + tui.StyleMuted.Render("no teams found — create brain/claude/teams/<name>/team.yaml"))
		return nil
	}

	maxLen := 0
	for _, m := range metas {
		if len(m.Name) > maxLen {
			maxLen = len(m.Name)
		}
	}
	for _, m := range metas {
		pad := strings.Repeat(" ", maxLen-len(m.Name)+2)
		badge := tui.StyleMuted.Render(fmt.Sprintf("%d persona(s)", len(m.Personas)))
		name := tui.StyleAccent.Render(m.Name)
		desc := tui.StyleNormal.Render(m.Description)
		fmt.Printf("  %s%s%s  %s\n", name, pad, badge, desc)
	}
	return nil
}

// --- teams use ---

type teamsUseOutput struct {
	Team     string        `json:"team"`
	Personas []personaJSON `json:"personas"`
}

type personaJSON struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	Prompt      string `json:"prompt"`
}

func teamsUseCmd(cfg *config.Config) *cobra.Command {
	var copyFlag bool
	cmd := &cobra.Command{
		Use:   "use <team>",
		Short: "Output team persona context blob (paste into Claude Code)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeamsUse(cfg, args[0], copyFlag, cmd)
		},
	}
	cmd.Flags().BoolVar(&copyFlag, "copy", false, "Copy output to clipboard (macOS)")
	return cmd
}

func runTeamsUse(cfg *config.Config, slug string, copyFlag bool, cmd *cobra.Command) error {
	team, err := loadTeam(cfg, slug)
	if err != nil {
		return err
	}

	if outputFormat(cmd) == "json" {
		out := teamsUseOutput{Team: team.meta.Name}
		for _, p := range team.personas {
			out.Personas = append(out.Personas, personaJSON{
				Slug:        p.slug,
				DisplayName: personaDisplayName(p.slug),
				Prompt:      p.prompt,
			})
		}
		raw, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		result := string(raw)
		fmt.Println(result)
		if copyFlag {
			_ = clipboardCopy(result)
		}
		return nil
	}

	var sb strings.Builder
	sb.WriteString("# Team Context\n\n")
	if team.meta.Lead != "" {
		sb.WriteString("You are supporting **" + team.meta.Lead + "** in a technical discussion.\n")
		sb.WriteString("They have final decision-making authority. Draw on the following team perspectives when advising:\n\n")
	} else {
		sb.WriteString("You are facilitating a technical session on behalf of a team.\n")
		sb.WriteString("Consider the following perspectives throughout the discussion:\n\n")
	}
	for _, p := range team.personas {
		sb.WriteString("## " + personaDisplayName(p.slug) + "\n\n")
		sb.WriteString(strings.TrimSpace(p.prompt))
		sb.WriteString("\n\n")
	}
	output := strings.TrimRight(sb.String(), "\n") + "\n"
	fmt.Print(output)

	if copyFlag {
		if err := clipboardCopy(output); err != nil {
			fmt.Println(tui.WarnLine("copy", err.Error()))
		} else {
			fmt.Println(tui.StyleOK.Render("copied to clipboard"))
		}
	}
	return nil
}

// --- teams consult ---

type consultOutput struct {
	Team      string            `json:"team"`
	Question  string            `json:"question"`
	Responses []personaResponse `json:"responses"`
}

type personaResponse struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	Response    string `json:"response,omitempty"`
	Error       string `json:"error,omitempty"`
}

func teamsConsultCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "consult <team> <question>",
		Short: "Ask each team persona a question via Claude API",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeamsConsult(cfg, args[0], args[1], cmd)
		},
	}
}

func runTeamsConsult(cfg *config.Config, slug, question string, cmd *cobra.Command) error {
	if cfg.Integrations.Claude == nil || cfg.Integrations.Claude.APIKey == "" {
		return fmt.Errorf("integrations.claude.api_key is not configured")
	}

	apiKey, err := secrets.Read(cfg.Integrations.Claude.APIKey)
	if err != nil {
		return fmt.Errorf("resolving claude api_key: %w", err)
	}

	team, err := loadTeam(cfg, slug)
	if err != nil {
		return err
	}

	client := claudeaiclient.New(apiKey)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// If a lead is defined, prepend a short framing to every persona's system prompt
	// so they understand whose authority they're operating under.
	leadPrefix := ""
	if team.meta.Lead != "" {
		leadPrefix = "You are advising " + team.meta.Lead + ", who has final decision-making authority on all matters. Answer directly and concisely from your persona's perspective.\n\n"
	}

	// Call each persona in parallel; preserve order via indexed channel.
	type indexed struct {
		i   int
		out personaResponse
	}
	ch := make(chan indexed, len(team.personas))
	var wg sync.WaitGroup
	for i, p := range team.personas {
		i, p := i, p
		wg.Add(1)
		go func() {
			defer wg.Done()
			text, err := client.Ask(ctx, leadPrefix+p.prompt, question, 0)
			r := personaResponse{
				Slug:        p.slug,
				DisplayName: personaDisplayName(p.slug),
			}
			if err != nil {
				r.Error = err.Error()
			} else {
				r.Response = text
			}
			ch <- indexed{i: i, out: r}
		}()
	}
	wg.Wait()
	close(ch)

	results := make([]personaResponse, len(team.personas))
	for idx := range ch {
		results[idx.i] = idx.out
	}

	if outputFormat(cmd) == "json" {
		raw, err := json.MarshalIndent(consultOutput{
			Team:      team.meta.Name,
			Question:  question,
			Responses: results,
		}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(raw))
		return nil
	}

	var b bytes.Buffer
	fmt.Fprintf(&b, "%s\n", tui.StyleHeader.Render("Consult: "+team.meta.Name))
	fmt.Fprintf(&b, "%s\n\n", tui.StyleMuted.Render(question))
	for _, r := range results {
		fmt.Fprintln(&b, tui.SectionHeader(r.DisplayName, ""))
		fmt.Fprintln(&b)
		if r.Error != "" {
			fmt.Fprintln(&b, "  "+tui.StyleError.Render("error: "+r.Error))
		} else {
			fmt.Fprintln(&b, r.Response)
		}
		fmt.Fprintln(&b)
	}
	fmt.Print(b.String())
	return nil
}

// outputFormat reads the --format persistent flag from the root command.
func outputFormat(cmd *cobra.Command) string {
	f := cmd.Root().PersistentFlags().Lookup("format")
	if f == nil {
		return "text"
	}
	return f.Value.String()
}

// clipboardCopy writes text to the macOS clipboard via pbcopy.
func clipboardCopy(text string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pbcopy: %w", err)
	}
	return nil
}
