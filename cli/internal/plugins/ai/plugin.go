package ai

import (
	"path/filepath"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/nativeplugin"
	"github.com/spf13/cobra"
)

func init() {
	nativeplugin.Register(&nativeplugin.Plugin{
		Name:        "ai",
		Description: "AI team personas",
		IsEnabled:   isEnabled,
		Commands:    commands,
	})
}

func isEnabled(cfg *config.Config) bool {
	s, ok := cfg.Plugins.Settings["claude"]
	return ok && s.Enabled
}

func brainAIDir(cfg *config.Config) string {
	return filepath.Join(config.ResolveBrainPath(cfg), "ai")
}

func commands(cfg *config.Config) []*cobra.Command {
	return []*cobra.Command{teamsCmd(cfg)}
}
