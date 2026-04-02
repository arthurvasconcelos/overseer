package cmd

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/gcal"
	"github.com/arthurvasconcelos/overseer/internal/jira"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	overseerslack "github.com/arthurvasconcelos/overseer/internal/slack"
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

	fmt.Printf("overseer daily — %s\n\n", time.Now().Format("Monday, 02 Jan 2006"))

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
			fmt.Printf("  [warn] %s: %v\n\n", t.label, results[i].err)
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

	fmt.Fprintf(w, "Google Calendar — %s (%d events today)\n", account.Name, len(events))
	if len(events) == 0 {
		fmt.Fprintf(w, "  no events today\n")
	}
	for _, e := range events {
		if e.AllDay {
			fmt.Fprintf(w, "  all day       %s\n", e.Title)
		} else {
			fmt.Fprintf(w, "  %s – %s  %s\n", e.Start.Format("15:04"), e.End.Format("15:04"), e.Title)
		}
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

	fmt.Fprintf(w, "Slack — %s\n", ws.Name)
	if len(mentions) == 0 {
		fmt.Fprintf(w, "  no recent mentions\n")
	} else {
		fmt.Fprintf(w, "  mentions:\n")
		for _, m := range mentions {
			fmt.Fprintf(w, "    #%-20s  %s\n", m.Channel, m.Text)
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

	fmt.Fprintf(w, "Jira — %s (%d open)\n", instance.Name, len(issues))
	if len(issues) == 0 {
		fmt.Fprintf(w, "  no open issues\n")
	}
	for _, i := range issues {
		fmt.Fprintf(w, "  %-12s  %-14s  %-10s  %s\n", i.Key, i.Status, i.Priority, i.Summary)
	}
	fmt.Fprintln(w)

	return nil
}
