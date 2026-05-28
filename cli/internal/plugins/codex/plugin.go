package codex

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
		Name:         "codex",
		Description:  "Codex config management",
		IsEnabled:    isEnabled,
		Commands:     commands,
		StatusChecks: statusChecks,
	})
}

func isEnabled(cfg *config.Config) bool {
	s, ok := cfg.Plugins.Settings["codex"]
	return ok && s.Enabled
}

// brainCodexDir returns the absolute path to <brain>/codex/.
func brainCodexDir(cfg *config.Config) string {
	return filepath.Join(config.ResolveBrainPath(cfg), "codex")
}

func commands(cfg *config.Config) []*cobra.Command {
	root := &cobra.Command{
		Use:         "codex",
		Short:       "Manage Codex configuration",
		Annotations: map[string]string{"overseer/group": "AI"},
	}
	root.AddCommand(setupCmd(cfg))
	root.AddCommand(listCmd(cfg))
	root.AddCommand(skillsCmd(cfg))
	root.AddCommand(codexMCPCmd())
	return []*cobra.Command{root}
}

func statusChecks(cfg *config.Config) []nativeplugin.StatusCheckFn {
	codexDir := brainCodexDir(cfg)
	targets := wellKnownTargets(codexDir)

	return []nativeplugin.StatusCheckFn{
		{
			Name: "codex",
			Run: func(_ context.Context) (bool, string) {
				return checkLinks(targets, codexDir)
			},
		},
	}
}

func checkLinks(targets []managedTarget, codexDir string) (bool, string) {
	ok := true
	var issues []string

	for _, t := range targets {
		linkOK, msg := linkStatus(t, codexDir)
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
