package google

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/gcal"
	"github.com/arthurvasconcelos/overseer/internal/nativeplugin"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	"github.com/arthurvasconcelos/overseer/internal/tui"
)

func init() {
	nativeplugin.Register(&nativeplugin.Plugin{
		Name:         "google",
		Description:  "Google Calendar events",
		IsEnabled:    isEnabled,
		DailyItems:   dailyItems,
		StatusChecks: statusChecks,
	})
}

func isEnabled(cfg *config.Config) bool {
	if s, ok := cfg.Plugins.Settings["google"]; ok {
		return s.Enabled
	}
	return len(cfg.Integrations.Google) > 0
}

func dailyItems(cfg *config.Config) []nativeplugin.DailyTask {
	tasks := make([]nativeplugin.DailyTask, len(cfg.Integrations.Google))
	for i, account := range cfg.Integrations.Google {
		account := account
		tasks[i] = nativeplugin.DailyTask{
			Label: "google/" + account.Name,
			Run: func(ctx context.Context, _ *config.Config) (string, error) {
				var b bytes.Buffer
				if err := printGCal(ctx, account, &b); err != nil {
					return "", err
				}
				return b.String(), nil
			},
		}
	}
	return tasks
}

func statusChecks(cfg *config.Config) []nativeplugin.StatusCheckFn {
	checks := make([]nativeplugin.StatusCheckFn, len(cfg.Integrations.Google))
	for i, account := range cfg.Integrations.Google {
		account := account
		checks[i] = nativeplugin.StatusCheckFn{
			Name: "google/" + account.Name,
			Run: func(_ context.Context) (bool, string) {
				return checkGoogle(account)
			},
		}
	}
	return checks
}

func printGCal(ctx context.Context, account config.GoogleAccount, w *bytes.Buffer) error {
	credsJSON, err := secrets.ReadAs(account.CredentialsDoc, account.OPAccount)
	if err != nil {
		return err
	}

	client, err := gcal.New(ctx, []byte(credsJSON), account.Name)
	if err != nil {
		return err
	}

	events, err := client.TodaysEvents(ctx)
	if err != nil {
		return err
	}

	badge := fmt.Sprintf("%d event today", len(events))
	if len(events) != 1 {
		badge = fmt.Sprintf("%d events today", len(events))
	}
	fmt.Fprintln(w, tui.SectionHeader("Google Calendar / "+account.Name, badge))
	if len(events) == 0 {
		fmt.Fprintln(w, "  "+tui.StyleMuted.Render("no events today"))
	}
	for _, e := range events {
		var timeCol string
		if e.AllDay {
			timeCol = tui.StyleMuted.Render("all day      ")
		} else {
			timeCol = tui.StyleAccent.Render(e.Start.Format("15:04") + " – " + e.End.Format("15:04"))
		}
		title := tui.StyleNormal.Render(e.Title)
		if e.JoinURL != "" {
			title += "  " + tui.StyleAccent.Render(tui.Hyperlink(e.JoinURL, "[Join]"))
		}
		fmt.Fprintf(w, "  %s  %s\n", timeCol, title)
	}
	fmt.Fprintln(w)

	return nil
}

func checkGoogle(acc config.GoogleAccount) (bool, string) {
	info, err := gcal.TokenStatus(acc.Name)
	if err != nil {
		return false, err.Error()
	}
	if !info.Present {
		return false, "no token — run: overseer daily to authorize"
	}
	if !info.Valid {
		return false, "token expired — run: overseer daily to refresh"
	}
	until := time.Until(info.Expiry)
	if until > 0 {
		return true, fmt.Sprintf("token valid (expires in %s)", formatDuration(until))
	}
	return true, "token valid"
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
