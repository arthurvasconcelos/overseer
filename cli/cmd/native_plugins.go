package cmd

import (
	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/nativeplugin"
)

// registerNativePluginCommands registers cobra subcommands contributed by enabled native plugins.
// Built-in commands always take precedence — plugins with the same name as an existing command
// are silently skipped.
func registerNativePluginCommands() {
	cfg, err := config.Load()
	if err != nil {
		return
	}
	for _, p := range nativeplugin.Enabled(cfg) {
		if p.Commands == nil {
			continue
		}
		for _, c := range p.Commands(cfg) {
			if existing, _, _ := rootCmd.Find([]string{c.Name()}); existing == rootCmd {
				rootCmd.AddCommand(c)
			}
		}
	}
}
