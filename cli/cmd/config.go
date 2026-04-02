package cmd

import (
	"fmt"
	"sort"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show active configuration",
	RunE:  runConfig,
}

func init() {
	rootCmd.AddCommand(configCmd)
}

// fileLink returns an OSC 8 hyperlink pointing to a local file path.
// Terminals that support OSC 8 (iTerm2, WezTerm, VS Code, etc.) render
// this as a clickable link; others print the plain path as fallback.
func fileLink(path string) string {
	return fmt.Sprintf("\033]8;;file://%s\033\\%s\033]8;;\033\\", path, path)
}

func kv(key, value string) string {
	return "  " + tui.StyleDim.Render(key+":") + " " + tui.StyleNormal.Render(value)
}

func runConfig(_ *cobra.Command, _ []string) error {
	path, err := config.Path()
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	localPath, err := config.LocalPath()
	if err != nil {
		return err
	}

	fmt.Println(kv("config", fileLink(path)))
	fmt.Println(kv("config (local)", fileLink(localPath)))

	if len(cfg.Secrets.Environments) > 0 {
		fmt.Println()
		fmt.Println(tui.StyleHeader.Render("▸ secrets"))
		keys := make([]string, 0, len(cfg.Secrets.Environments))
		for k := range cfg.Secrets.Environments {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Println(kv("  "+k, cfg.Secrets.Environments[k]))
		}
	}

	if len(cfg.Integrations.Jira) > 0 {
		fmt.Println()
		fmt.Println(tui.StyleHeader.Render("▸ jira"))
		for _, j := range cfg.Integrations.Jira {
			fmt.Println(kv("  "+j.Name, j.BaseURL))
		}
	}
	if len(cfg.Integrations.Slack) > 0 {
		fmt.Println()
		fmt.Println(tui.StyleHeader.Render("▸ slack"))
		for _, s := range cfg.Integrations.Slack {
			fmt.Println("  " + tui.StyleNormal.Render(s.Name))
		}
	}
	if len(cfg.Integrations.Google) > 0 {
		fmt.Println()
		fmt.Println(tui.StyleHeader.Render("▸ google"))
		for _, g := range cfg.Integrations.Google {
			fmt.Println(kv("  "+g.Name, g.CredentialsDoc))
		}
	}

	return nil
}
