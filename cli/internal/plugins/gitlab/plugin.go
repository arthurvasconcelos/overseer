package gitlab

import (
	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/nativeplugin"
)

func init() {
	nativeplugin.Register(&nativeplugin.Plugin{
		Name:        "gitlab",
		Description: "GitLab merge requests",
		IsEnabled:   isEnabled,
	})
}

func isEnabled(cfg *config.Config) bool {
	if s, ok := cfg.Plugins.Settings["gitlab"]; ok {
		return s.Enabled
	}
	return len(cfg.Integrations.GitLab) > 0
}
