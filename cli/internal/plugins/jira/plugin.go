package jira

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/arthurvasconcelos/overseer/internal/config"
	jiraclient "github.com/arthurvasconcelos/overseer/internal/jira"
	"github.com/arthurvasconcelos/overseer/internal/nativeplugin"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/charmbracelet/lipgloss"
)

func init() {
	nativeplugin.Register(&nativeplugin.Plugin{
		Name:         "jira",
		Description:  "Jira issue tracking",
		IsEnabled:    isEnabled,
		DailyItems:   dailyItems,
		StatusChecks: statusChecks,
	})
}

func isEnabled(cfg *config.Config) bool {
	if s, ok := cfg.Plugins.Settings["jira"]; ok {
		return s.Enabled
	}
	return len(cfg.Integrations.Jira) > 0
}

func dailyItems(cfg *config.Config) []nativeplugin.DailyTask {
	tasks := make([]nativeplugin.DailyTask, len(cfg.Integrations.Jira))
	for i, instance := range cfg.Integrations.Jira {
		instance := instance
		tasks[i] = nativeplugin.DailyTask{
			Label: "jira/" + instance.Name,
			Run: func(ctx context.Context, _ *config.Config) (string, error) {
				var b bytes.Buffer
				if err := printJira(ctx, instance, &b); err != nil {
					return "", err
				}
				return b.String(), nil
			},
		}
	}
	return tasks
}

func statusChecks(cfg *config.Config) []nativeplugin.StatusCheckFn {
	checks := make([]nativeplugin.StatusCheckFn, len(cfg.Integrations.Jira))
	for i, instance := range cfg.Integrations.Jira {
		instance := instance
		checks[i] = nativeplugin.StatusCheckFn{
			Name: "jira/" + instance.Name,
			Run: func(ctx context.Context) (bool, string) {
				return checkJira(ctx, instance)
			},
		}
	}
	return checks
}

func printJira(ctx context.Context, instance config.JiraInstance, w *bytes.Buffer) error {
	email, err := secrets.ReadAs(instance.Email, instance.OPAccount)
	if err != nil {
		return err
	}
	token, err := secrets.ReadAs(instance.Token, instance.OPAccount)
	if err != nil {
		return err
	}

	client := jiraclient.New(instance.BaseURL, email, token)
	issues, err := client.MyIssues(ctx)
	if err != nil {
		return err
	}

	badge := fmt.Sprintf("%d open", len(issues))
	fmt.Fprintln(w, tui.SectionHeader("Jira / "+instance.Name, badge))
	if len(issues) == 0 {
		fmt.Fprintln(w, "  "+tui.StyleMuted.Render("no open issues"))
	}
	for _, i := range issues {
		key := tui.StyleAccent.Render(fmt.Sprintf("%-12s", i.Key))
		status := statusStyle(i.Status).Render(fmt.Sprintf("%-14s", i.Status))
		priority := priorityStyle(i.Priority).Render(fmt.Sprintf("%-10s", i.Priority))
		fmt.Fprintf(w, "  %s  %s  %s  %s\n", key, status, priority, tui.StyleNormal.Render(i.Summary))
	}
	fmt.Fprintln(w)

	return nil
}

func checkJira(ctx context.Context, inst config.JiraInstance) (bool, string) {
	email, err := secrets.ReadAs(inst.Email, inst.OPAccount)
	if err != nil {
		return false, "could not read email: " + err.Error()
	}
	token, err := secrets.ReadAs(inst.Token, inst.OPAccount)
	if err != nil {
		return false, "could not read token: " + err.Error()
	}
	authedEmail, err := jiraclient.New(inst.BaseURL, email, token).Ping(ctx)
	if err != nil {
		return false, err.Error()
	}
	return true, authedEmail
}

func statusStyle(status string) lipgloss.Style {
	switch strings.ToLower(status) {
	case "in progress":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // amber
	case "in review", "code review":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("39")) // blue
	case "done", "closed", "resolved":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("82")) // green
	default:
		return tui.StyleDim
	}
}

func priorityStyle(priority string) lipgloss.Style {
	switch strings.ToLower(priority) {
	case "critical", "highest":
		return tui.StyleError
	case "high":
		return tui.StyleWarn
	case "medium":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("228")) // soft yellow
	default:
		return tui.StyleMuted
	}
}
