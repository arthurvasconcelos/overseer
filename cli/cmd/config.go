package cmd

import (
	"fmt"
	"sort"

	"github.com/arthurvasconcelos/overseer/internal/config"
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

func runConfig(_ *cobra.Command, _ []string) error {
	path, err := config.Path()
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	fmt.Printf("config file: %s\n\n", fileLink(path))
	fmt.Printf("secrets:\n")
	fmt.Printf("  vault: %s\n", cfg.Secrets.Vault)

	if len(cfg.Secrets.Environments) > 0 {
		fmt.Printf("  environments:\n")
		keys := make([]string, 0, len(cfg.Secrets.Environments))
		for k := range cfg.Secrets.Environments {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("    %s: %s\n", k, cfg.Secrets.Environments[k])
		}
	}

	return nil
}
