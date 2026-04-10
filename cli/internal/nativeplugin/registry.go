package nativeplugin

import "github.com/arthurvasconcelos/overseer/internal/config"

// registry holds all registered native plugins in registration order.
// Written only by Register; read by All and Enabled.
var registry []*Plugin

// Register adds a plugin to the native registry.
// Call this from an init() function in the plugin's own package.
func Register(p *Plugin) {
	registry = append(registry, p)
}

// All returns all registered plugins regardless of enabled state.
func All() []*Plugin {
	out := make([]*Plugin, len(registry))
	copy(out, registry)
	return out
}

// Enabled returns only the plugins that report IsEnabled(cfg) == true.
// If cfg is nil, no plugins are considered enabled.
func Enabled(cfg *config.Config) []*Plugin {
	if cfg == nil {
		return nil
	}
	var out []*Plugin
	for _, p := range registry {
		if p.IsEnabled != nil && p.IsEnabled(cfg) {
			out = append(out, p)
		}
	}
	return out
}
