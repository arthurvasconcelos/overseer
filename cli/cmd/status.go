package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/nativeplugin"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Health-check all configured integrations",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

type checkResult struct {
	name string
	ok   bool
	msg  string
}

type checkResultJSON struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

func runStatus(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Build the list of checks to run in parallel.
	type checkFn struct {
		name string
		fn   func() checkResult
	}

	var checks []checkFn

	checks = append(checks, checkFn{"1password", checkOnePassword})

	for _, p := range nativeplugin.Enabled(cfg) {
		if p.StatusChecks == nil {
			continue
		}
		for _, sc := range p.StatusChecks(cfg) {
			sc := sc
			checks = append(checks, checkFn{
				name: sc.Name,
				fn: func() checkResult {
					ok, msg := sc.Run(ctx)
					return checkResult{name: sc.Name, ok: ok, msg: msg}
				},
			})
		}
	}

	// Append checks from external plugins that declared the "status" hook.
	for _, ep := range ExternalPluginsWithHook("status") {
		ep := ep
		checks = append(checks, checkFn{
			name: ep.name,
			fn: func() checkResult {
				return runExternalStatusHook(ep)
			},
		})
	}

	results := make([]checkResult, len(checks))
	var wg sync.WaitGroup
	for i, c := range checks {
		i, c := i, c
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = c.fn()
			results[i].name = c.name
		}()
	}
	wg.Wait()

	if outputFormat == "json" {
		out := make([]checkResultJSON, len(results))
		for i, r := range results {
			out[i] = checkResultJSON{Name: r.name, OK: r.ok, Message: r.msg}
		}
		return printJSON(out)
	}

	// Calculate column width for alignment.
	maxLen := 0
	for _, r := range results {
		if len(r.name) > maxLen {
			maxLen = len(r.name)
		}
	}

	for _, r := range results {
		icon := tui.StyleOK.Render("✓")
		if !r.ok {
			icon = tui.StyleError.Render("✗")
		}
		padding := strings.Repeat(" ", maxLen-len(r.name)+2)
		fmt.Printf("  %s%s%s  %s\n",
			tui.StyleNormal.Render(r.name),
			padding,
			icon,
			tui.StyleDim.Render(r.msg),
		)
	}
	return nil
}

func checkOnePassword() checkResult {
	out, err := exec.Command("op", "account", "list", "--format=json").Output()
	if err != nil {
		return checkResult{ok: false, msg: "op CLI not available or not signed in"}
	}
	var accounts []struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(out, &accounts); err != nil || len(accounts) == 0 {
		return checkResult{ok: false, msg: "no accounts found"}
	}
	noun := "account"
	if len(accounts) > 1 {
		noun = "accounts"
	}
	return checkResult{ok: true, msg: fmt.Sprintf("signed in (%d %s)", len(accounts), noun)}
}

// runExternalStatusHook calls an external plugin with the "status" hook and
// parses its JSON output into checkResult entries. If the plugin outputs
// multiple items, the first is returned with its name as the label; the rest
// are silently merged into the result message. For full multi-item support,
// native plugins are the recommended path.
func runExternalStatusHook(ep externalPlugin) checkResult {
	out, err := runHook(ep, "status")
	if err != nil {
		return checkResult{name: ep.name, ok: false, msg: err.Error()}
	}
	var items []checkResultJSON
	if err := json.Unmarshal([]byte(out), &items); err != nil || len(items) == 0 {
		return checkResult{name: ep.name, ok: true, msg: strings.TrimSpace(out)}
	}
	// Return the first item; additional items are appended to its message.
	r := checkResult{name: items[0].Name, ok: items[0].OK, msg: items[0].Message}
	for _, extra := range items[1:] {
		sign := "✓"
		if !extra.OK {
			sign = "✗"
			r.ok = false
		}
		r.msg += fmt.Sprintf(" | %s %s: %s", sign, extra.Name, extra.Message)
	}
	return r
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
