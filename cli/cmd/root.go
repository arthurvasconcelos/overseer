package cmd

import (
	"github.com/arthurvasconcelos/overseer/internal/output"
	"github.com/spf13/cobra"
)

// Version is injected at build time via -ldflags.
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "overseer",
	Short: "Personal CLI for environment setup and daily automation",
	Long: `overseer is a personal CLI tool for bootstrapping machines, managing
dotfiles, and integrating with daily tools like Slack, Google Calendar,
and 1Password.`,
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd.Version = Version
	rootCmd.SetVersionTemplate("overseer {{.Version}}\n")
	rootCmd.PersistentFlags().StringVar(&output.Format, "format", "text", "Output format: text or json")
	rootCmd.InitDefaultCompletionCmd()
	registerPlugins()
	registerNativePluginCommands()
}
