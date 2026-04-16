package claude

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/nativeplugin"
	"github.com/spf13/cobra"
)

func init() {
	nativeplugin.Register(&nativeplugin.Plugin{
		Name:         "claude",
		Description:  "Claude AI config management",
		IsEnabled:    isEnabled,
		Commands:     commands,
		StatusChecks: statusChecks,
	})
}

func isEnabled(cfg *config.Config) bool {
	s, ok := cfg.Plugins.Settings["claude"]
	return ok && s.Enabled
}

// brainClaudeDir returns the absolute path to <brain>/claude/.
func brainClaudeDir(cfg *config.Config) string {
	return filepath.Join(config.ResolveBrainPath(cfg), "claude")
}

func commands(cfg *config.Config) []*cobra.Command {
	root := &cobra.Command{
		Use:         "claude",
		Short:       "Manage Claude AI configuration and team personas",
		Annotations: map[string]string{"overseer/group": "AI"},
	}
	root.AddCommand(setupCmd(cfg))
	root.AddCommand(listCmd(cfg))
	root.AddCommand(teamsCmd(cfg))
	return []*cobra.Command{root}
}

func statusChecks(cfg *config.Config) []nativeplugin.StatusCheckFn {
	claudeDir := brainClaudeDir(cfg)
	targets := wellKnownTargets(claudeDir)

	return []nativeplugin.StatusCheckFn{
		{
			Name: "claude",
			Run: func(_ context.Context) (bool, string) {
				return checkLinks(targets, claudeDir)
			},
		},
	}
}

func checkLinks(targets []managedTarget, claudeDir string) (bool, string) {
	ok := true
	var issues []string

	for _, t := range targets {
		linkOK, msg := linkStatus(t, claudeDir)
		if !linkOK {
			ok = false
			issues = append(issues, msg)
		}
	}

	if ok {
		return true, fmt.Sprintf("all links healthy (%d targets)", len(targets))
	}
	if len(issues) == 1 {
		return false, issues[0]
	}
	return false, fmt.Sprintf("%d issues: %s", len(issues), strings.Join(issues, "; "))
}

