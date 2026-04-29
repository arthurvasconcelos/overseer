package slack

import (
	"bytes"
	"context"
	"fmt"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/nativeplugin"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	slackclient "github.com/arthurvasconcelos/overseer/internal/slack"
	"github.com/arthurvasconcelos/overseer/internal/tui"
)

func init() {
	nativeplugin.Register(&nativeplugin.Plugin{
		Name:         "slack",
		Description:  "Slack mentions",
		IsEnabled:    isEnabled,
		Commands:     commands,
		DailyItems:   dailyItems,
		StatusChecks: statusChecks,
	})
}

func isEnabled(cfg *config.Config) bool {
	if s, ok := cfg.Plugins.Settings["slack"]; ok {
		return s.Enabled
	}
	return len(cfg.Integrations.Slack) > 0
}

func dailyItems(cfg *config.Config) []nativeplugin.DailyTask {
	tasks := make([]nativeplugin.DailyTask, len(cfg.Integrations.Slack))
	for i, ws := range cfg.Integrations.Slack {
		ws := ws
		tasks[i] = nativeplugin.DailyTask{
			Label: "slack/" + ws.Name,
			Run: func(_ context.Context, _ *config.Config) (string, error) {
				var b bytes.Buffer
				if err := printSlack(ws, &b); err != nil {
					return "", err
				}
				return b.String(), nil
			},
		}
	}
	return tasks
}

func statusChecks(cfg *config.Config) []nativeplugin.StatusCheckFn {
	checks := make([]nativeplugin.StatusCheckFn, len(cfg.Integrations.Slack))
	for i, ws := range cfg.Integrations.Slack {
		ws := ws
		checks[i] = nativeplugin.StatusCheckFn{
			Name: "slack/" + ws.Name,
			Run: func(_ context.Context) (bool, string) {
				return checkSlack(ws)
			},
		}
	}
	return checks
}

func printSlack(ws config.SlackWorkspace, w *bytes.Buffer) error {
	client, err := buildClient(ws)
	if err != nil {
		return err
	}

	mentions, err := client.Mentions(ws.GroupHandles)
	if err != nil {
		return err
	}

	fmt.Fprintln(w, tui.SectionHeader("Slack / "+ws.Name, ""))
	if len(mentions) == 0 {
		fmt.Fprintln(w, "  "+tui.StyleMuted.Render("no recent mentions"))
	} else {
		shown := mentions
		if len(shown) > 5 {
			shown = shown[:5]
		}
		for _, m := range shown {
			channel := tui.StyleAccent.Render("#" + m.Channel)
			fmt.Fprintf(w, "  %-30s  %s\n", channel, tui.StyleNormal.Render(m.Text))
		}
		if len(mentions) > 5 {
			fmt.Fprintln(w, "  "+tui.StyleMuted.Render(fmt.Sprintf("…and %d more — run: overseer slack mentions", len(mentions)-5)))
		}
	}
	fmt.Fprintln(w)

	return nil
}

func checkSlack(ws config.SlackWorkspace) (bool, string) {
	token, err := secrets.ReadAs(ws.Token, ws.OPAccount)
	if err != nil {
		return false, "could not read token: " + err.Error()
	}
	username, err := slackclient.New(token).Ping()
	if err != nil {
		return false, err.Error()
	}
	return true, "@" + username
}
