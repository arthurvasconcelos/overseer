package nativeplugin

import (
	"context"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/spf13/cobra"
)

// DailyTask is one section contributed by a plugin to the daily briefing.
// Label is used for error reporting; Run produces the rendered output.
type DailyTask struct {
	Label string
	Run   func(ctx context.Context, cfg *config.Config) (string, error)
}

// StatusCheckFn is a single named health check contributed to overseer status.
type StatusCheckFn struct {
	Name string
	Run  func(ctx context.Context) (ok bool, msg string)
}

// Plugin declares a native (compiled-in) plugin.
// All func fields are optional — nil means the plugin does not implement that
// extension point. Only Name and IsEnabled are required.
type Plugin struct {
	// Name is the unique identifier, e.g. "jira", "slack", "claude".
	Name string

	// Description is the short human-readable summary shown in plugins list.
	Description string

	// IsEnabled reports whether this plugin is active for the given config.
	IsEnabled func(cfg *config.Config) bool

	// Commands returns cobra subcommands contributed by this plugin.
	// Called once at startup if the plugin is enabled. May be nil.
	Commands func(cfg *config.Config) []*cobra.Command

	// DailyItems returns the tasks this plugin contributes to overseer daily.
	// Each task runs in its own goroutine. May be nil.
	DailyItems func(cfg *config.Config) []DailyTask

	// StatusChecks returns the health checks this plugin contributes to overseer status.
	// Each check runs in its own goroutine. May be nil.
	StatusChecks func(cfg *config.Config) []StatusCheckFn
}
