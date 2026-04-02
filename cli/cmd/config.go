package cmd

import (
	"fmt"

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

func runConfig(_ *cobra.Command, _ []string) error {
	path, err := config.Path()
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	fmt.Printf("config file: %s\n\n", path)
	fmt.Printf("secrets:\n")
	fmt.Printf("  vault: %s\n", cfg.Secrets.Vault)

	return nil
}
