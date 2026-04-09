package cmd

import (
	"github.com/spf13/cobra"
)

// Version is injected at build time via -ldflags.
var Version = "dev"

// outputFormat holds the value of the --format flag ("text" or "json").
var outputFormat string

var rootCmd = &cobra.Command{
	Use:   "overseer",
	Short: "Personal CLI for environment setup and daily automation",
	Long: `overseer is a personal CLI tool for bootstrapping machines, managing
dotfiles, and integrating with daily tools like Slack, Google Calendar,
and 1Password.`,
	PersistentPreRun: func(_ *cobra.Command, _ []string) {
		startUpdateCheck()
	},
	PersistentPostRun: func(_ *cobra.Command, _ []string) {
		printUpdateNotice()
	},
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd.Version = Version
	rootCmd.SetVersionTemplate("overseer {{.Version}}\n")
	rootCmd.PersistentFlags().StringVar(&outputFormat, "format", "text", "Output format: text or json")
	registerPlugins()
}
