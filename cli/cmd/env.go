package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage environment variable profiles",
}

func init() {
	rootCmd.AddCommand(envCmd)
	envCmd.AddCommand(envListCmd)
	envCmd.AddCommand(envShowCmd)
	envCmd.AddCommand(envUseCmd)
}

var envListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all env profiles",
	Args:  cobra.NoArgs,
	RunE:  runEnvList,
}

var envShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show variables for an env profile",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runEnvShow,
}

var envUseCmd = &cobra.Command{
	Use:   "use [name]",
	Short: "Print export statements for an env profile",
	Long:  "Prints export KEY=VALUE statements to stdout.\nUsage: eval $(overseer env use <name>)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runEnvUse,
}

func runEnvList(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	profiles := cfg.Env.Profiles
	if outputFormat == "json" {
		type item struct {
			Name string   `json:"name"`
			Vars []string `json:"vars"`
		}
		out := make([]item, len(profiles))
		for i, p := range profiles {
			keys := envSortedKeys(p.Vars)
			out[i] = item{Name: p.Name, Vars: keys}
		}
		return printJSON(out)
	}
	if len(profiles) == 0 {
		fmt.Println(tui.StyleMuted.Render("no env profiles configured — add env.profiles to config.yaml"))
		return nil
	}
	maxLen := 0
	for _, p := range profiles {
		if len(p.Name) > maxLen {
			maxLen = len(p.Name)
		}
	}
	for _, p := range profiles {
		pad := strings.Repeat(" ", maxLen-len(p.Name)+2)
		badge := tui.StyleMuted.Render(fmt.Sprintf("%d var(s)", len(p.Vars)))
		fmt.Printf("  %s%s%s\n", tui.StyleAccent.Render(p.Name), pad, badge)
	}
	return nil
}

func runEnvShow(_ *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	profile, err := pickEnvProfile(cfg, args)
	if err != nil {
		return err
	}
	keys := envSortedKeys(profile.Vars)
	if outputFormat == "json" {
		type kv struct {
			Key    string `json:"key"`
			Value  string `json:"value"`
			Secret bool   `json:"secret"`
		}
		out := make([]kv, 0, len(keys))
		for _, k := range keys {
			v := profile.Vars[k]
			isSecret := strings.HasPrefix(v, "op://")
			display := v
			if isSecret {
				display = "<secret>"
			}
			out = append(out, kv{Key: k, Value: display, Secret: isSecret})
		}
		return printJSON(out)
	}
	fmt.Println(tui.SectionHeader("env: "+profile.Name, ""))
	fmt.Println()
	maxLen := 0
	for _, k := range keys {
		if len(k) > maxLen {
			maxLen = len(k)
		}
	}
	for _, k := range keys {
		v := profile.Vars[k]
		pad := strings.Repeat(" ", maxLen-len(k)+2)
		display := tui.StyleNormal.Render(v)
		if strings.HasPrefix(v, "op://") {
			display = tui.StyleMuted.Render("<secret>")
		}
		fmt.Printf("  %s%s%s\n", tui.StyleAccent.Render(k), pad, display)
	}
	return nil
}

func runEnvUse(_ *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	profile, err := pickEnvProfile(cfg, args)
	if err != nil {
		return err
	}
	for _, k := range envSortedKeys(profile.Vars) {
		v := profile.Vars[k]
		if strings.HasPrefix(v, "op://") {
			resolved, resolveErr := secrets.ReadAs(v, profile.OPAccount)
			if resolveErr != nil {
				return fmt.Errorf("resolving %s: %w", k, resolveErr)
			}
			v = resolved
		}
		fmt.Printf("export %s=%s\n", k, envShellQuote(v))
	}
	return nil
}

func pickEnvProfile(cfg *config.Config, args []string) (*config.EnvProfile, error) {
	profiles := cfg.Env.Profiles
	if len(profiles) == 0 {
		return nil, fmt.Errorf("no env profiles configured")
	}
	if len(args) == 1 {
		for i := range profiles {
			if profiles[i].Name == args[0] {
				return &profiles[i], nil
			}
		}
		return nil, fmt.Errorf("env profile %q not found", args[0])
	}
	items := make([]tui.SelectItem, len(profiles))
	for i, p := range profiles {
		items[i] = tui.SelectItem{
			Title:    p.Name,
			Subtitle: tui.StyleMuted.Render(fmt.Sprintf("%d var(s)", len(p.Vars))),
		}
	}
	idx, err := tui.Select("Select env profile", items)
	if err != nil {
		return nil, err
	}
	if idx < 0 {
		return nil, fmt.Errorf("cancelled")
	}
	return &profiles[idx], nil
}

func envSortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// envShellQuote wraps v in single quotes, escaping any single quotes within.
func envShellQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'"
}
