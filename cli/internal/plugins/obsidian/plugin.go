package obsidian

import (
	"context"
	"os"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/nativeplugin"
)

func init() {
	nativeplugin.Register(&nativeplugin.Plugin{
		Name:         "obsidian",
		Description:  "Obsidian vault",
		IsEnabled:    isEnabled,
		StatusChecks: statusChecks,
	})
}

func isEnabled(cfg *config.Config) bool {
	if s, ok := cfg.Plugins.Settings["obsidian"]; ok {
		return s.Enabled
	}
	return cfg.Obsidian.VaultPath != ""
}

func statusChecks(cfg *config.Config) []nativeplugin.StatusCheckFn {
	if cfg.Obsidian.VaultPath == "" {
		return nil
	}
	vaultPath := cfg.Obsidian.VaultPath
	return []nativeplugin.StatusCheckFn{
		{
			Name: "obsidian",
			Run: func(_ context.Context) (bool, string) {
				if _, err := os.Stat(vaultPath); err != nil {
					return false, "vault path not found: " + vaultPath
				}
				return true, vaultPath
			},
		},
	}
}
