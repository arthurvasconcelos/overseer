package github

import (
	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/nativeplugin"
)

func init() {
	nativeplugin.Register(&nativeplugin.Plugin{
		Name:        "github",
		Description: "GitHub pull requests",
		IsEnabled:   isEnabled,
	})
}

func isEnabled(cfg *config.Config) bool {
	if s, ok := cfg.Plugins.Settings["github"]; ok {
		return s.Enabled
	}
	return len(cfg.Integrations.GitHub) > 0
}
