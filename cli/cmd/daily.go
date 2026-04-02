package cmd

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/gcal"
	"github.com/arthurvasconcelos/overseer/internal/jira"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	overseerslack "github.com/arthurvasconcelos/overseer/internal/slack"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var dailyCmd = &cobra.Command{
	Use:   "daily",
	Short: "Morning briefing — issues, calendar, messages",
	RunE:  runDaily,
}

func init() {
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

	// Build one task per configured integration instance.
	type task struct {
		label string
		run   func() (string, error)
	}

	var tasks []task
	for _, instance := range cfg.Integrations.Jira {
		instance := instance
		tasks = append(tasks, task{
			label: "jira/" + instance.Name,
			run: func() (string, error) {
				var b bytes.Buffer
				if err := printJira(ctx, instance, &b); err != nil {
					return "", err
				}
				return b.String(), nil
			},
		})
	}
	for _, ws := range cfg.Integrations.Slack {
		ws := ws
		tasks = append(tasks, task{
			label: "slack/" + ws.Name,
			run: func() (string, error) {
				var b bytes.Buffer
				if err := printSlack(ws, &b); err != nil {
					return "", err
				}
				return b.String(), nil
			},
		})
	}
	for _, account := range cfg.Integrations.Google {
		account := account
		tasks = append(tasks, task{
			label: "google/" + account.Name,
			run: func() (string, error) {
				var b bytes.Buffer
				if err := printGCal(ctx, account, &b); err != nil {
					return "", err
				}
				return b.String(), nil
			},
		})
	}

	// Run all tasks in parallel, preserving order in results.
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

	// Print in original order.
	for i, t := range tasks {
		if results[i].err != nil {
			fmt.Println(tui.WarnLine(t.label, results[i].err.Error()))
			fmt.Println()
		} else {
			fmt.Print(results[i].buf.String())
		}
	}

	return nil
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
		fmt.Fprintf(w, "  %s  %s\n", timeCol, tui.StyleNormal.Render(e.Title))
	}
	fmt.Fprintln(w)

	return nil
}

func printSlack(ws config.SlackWorkspace, w *bytes.Buffer) error {
	token, err := secrets.ReadAs(ws.Token, ws.OPAccount)
	if err != nil {
		return err
	}

	client := overseerslack.New(token)

	mentions, err := client.Mentions()
	if err != nil {
		return err
	}

	fmt.Fprintln(w, tui.SectionHeader("Slack / "+ws.Name, ""))
	if len(mentions) == 0 {
		fmt.Fprintln(w, "  "+tui.StyleMuted.Render("no recent mentions"))
	} else {
		for _, m := range mentions {
			channel := tui.StyleAccent.Render("#" + m.Channel)
			fmt.Fprintf(w, "  %-30s  %s\n", channel, tui.StyleNormal.Render(m.Text))
		}
	}
	fmt.Fprintln(w)

	return nil
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

	client := jira.New(instance.BaseURL, email, token)
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
		status := jiraStatusStyle(i.Status).Render(fmt.Sprintf("%-14s", i.Status))
		priority := jiraPriorityStyle(i.Priority).Render(fmt.Sprintf("%-10s", i.Priority))
		fmt.Fprintf(w, "  %s  %s  %s  %s\n", key, status, priority, tui.StyleNormal.Render(i.Summary))
	}
	fmt.Fprintln(w)

	return nil
}

func jiraStatusStyle(status string) lipgloss.Style {
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

func jiraPriorityStyle(priority string) lipgloss.Style {
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
