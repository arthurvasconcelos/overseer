package cmd

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/arthurvasconcelos/overseer/internal/claudeai"
	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/nativeplugin"
	"github.com/arthurvasconcelos/overseer/internal/notify"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

var dailyCmd = &cobra.Command{
	Use:   "daily",
	Short: "Morning briefing — issues, calendar, messages",
	RunE:  runDaily,
}

var dailyCopy bool

func init() {
	dailyCmd.Flags().BoolVar(&dailyCopy, "copy", false, "Copy output to clipboard (macOS)")
	rootCmd.AddCommand(dailyCmd)
}

// section holds the buffered output for one integration so results can be
// collected in parallel and printed in a deterministic order.
type section struct {
	buf bytes.Buffer
	err error
}

func runDaily(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println(tui.StyleHeader.Render("overseer daily") + "  " + tui.StyleMuted.Render(time.Now().Format("Monday, 02 Jan 2006")))
	fmt.Println()

	// Build one task per contribution from enabled native plugins.
	type task struct {
		label string
		run   func() (string, error)
	}

	var tasks []task

	for _, p := range nativeplugin.Enabled(cfg) {
		if p.DailyItems == nil {
			continue
		}
		for _, dt := range p.DailyItems(cfg) {
			dt := dt
			tasks = append(tasks, task{
				label: dt.Label,
				run:   func() (string, error) { return dt.Run(ctx, cfg) },
			})
		}
	}

	// Append tasks from external plugins that declared the "daily" hook.
	for _, ep := range ExternalPluginsWithHook("daily") {
		ep := ep
		tasks = append(tasks, task{
			label: ep.name,
			run:   func() (string, error) { return runHook(ep, "daily") },
		})
	}

	// Run all tasks in parallel, preserving order in results.
	stopSpinner := tui.StartSpinner("loading daily briefing…")
	results := make([]section, len(tasks))
	var wg sync.WaitGroup
	for i, t := range tasks {
		i, t := i, t
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := t.run()
			results[i].buf.WriteString(out)
			results[i].err = err
		}()
	}
	wg.Wait()
	stopSpinner()

	// Collect output in order, then print (and optionally copy).
	var body bytes.Buffer
	for i, t := range tasks {
		if results[i].err != nil {
			body.WriteString(tui.WarnLine(t.label, results[i].err.Error()) + "\n\n")
		} else {
			body.Write(results[i].buf.Bytes())
		}
	}
	fmt.Print(body.String())

	// Optional AI summary — opt-in via integrations.claude.daily_ai_summary: true.
	if cfg.Integrations.Claude != nil && cfg.Integrations.Claude.DailyAISummary && cfg.Integrations.Claude.APIKey != "" {
		aiCtx, aiCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer aiCancel()
		apiKey, err := secrets.ReadAs(cfg.Integrations.Claude.APIKey, "")
		if err == nil {
			summary, err := claudeai.New(apiKey).Ask(aiCtx,
				"You are a terse assistant. Given a morning briefing, produce exactly 3 sentences: first, the top priority for today; second, one notable update or upcoming event; third, any blockers or risks. No lists, no headers.",
				stripANSI(body.String()),
				256,
			)
			if err == nil && summary != "" {
				fmt.Println()
				fmt.Println(tui.StyleHeader.Render("AI Summary"))
				fmt.Println(summary)
			}
		}
	}

	if dailyCopy {
		if err := copyToClipboard(body.String()); err != nil {
			fmt.Println(tui.WarnLine("copy", err.Error()))
		}
	}

	if cfg.System.Notifications {
		_ = notify.Send("overseer daily", "Morning briefing ready", "")
	}

	return nil
}
