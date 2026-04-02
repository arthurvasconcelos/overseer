package cmd

import (
	"context"
	"fmt"
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

func runDaily(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Printf("overseer daily — %s\n\n", time.Now().Format("Monday, 02 Jan 2006"))

	for _, instance := range cfg.Integrations.Jira {
		if err := printJira(ctx, instance); err != nil {
			fmt.Printf("  [warn] jira/%s: %v\n", instance.Name, err)
		}
	}

	for _, ws := range cfg.Integrations.Slack {
		if err := printSlack(ws); err != nil {
			fmt.Printf("  [warn] slack/%s: %v\n", ws.Name, err)
		}
	}

	for _, account := range cfg.Integrations.Google {
		if err := printGCal(ctx, account); err != nil {
			fmt.Printf("  [warn] google/%s: %v\n", account.Name, err)
		}
	}

	return nil
}

func printGCal(ctx context.Context, account config.GoogleAccount) error {
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

	fmt.Printf("Google Calendar — %s (%d events today)\n", account.Name, len(events))
	if len(events) == 0 {
		fmt.Printf("  no events today\n")
	}
	for _, e := range events {
		if e.AllDay {
			fmt.Printf("  all day       %s\n", e.Title)
		} else {
			fmt.Printf("  %s – %s  %s\n",
				e.Start.Format("15:04"),
				e.End.Format("15:04"),
				e.Title,
			)
		}
	}
	fmt.Println()

	return nil
}

func printSlack(ws config.SlackWorkspace) error {
	token, err := secrets.Read(ws.Token)
	if err != nil {
		return err
	}

	client := overseerslack.New(token)

	mentions, err := client.Mentions()
	if err != nil {
		return err
	}

	fmt.Printf("Slack — %s\n", ws.Name)

	if len(mentions) == 0 {
		fmt.Printf("  no recent mentions\n")
	} else {
		fmt.Printf("  mentions:\n")
		for _, m := range mentions {
			fmt.Printf("    #%-20s  %s\n", m.Channel, m.Text)
		}
	}
	fmt.Println()

	return nil
}

func printJira(ctx context.Context, instance config.JiraInstance) error {
	email, err := secrets.Read(instance.Email)
	if err != nil {
		return err
	}
	token, err := secrets.Read(instance.Token)
	if err != nil {
		return err
	}

	client := jira.New(instance.BaseURL, email, token)
	issues, err := client.MyIssues(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Jira — %s (%d open)\n", instance.Name, len(issues))
	if len(issues) == 0 {
		fmt.Printf("  no open issues\n")
	}
	for _, i := range issues {
		fmt.Printf("  %-12s  %-14s  %-10s  %s\n", i.Key, i.Status, i.Priority, i.Summary)
	}
	fmt.Println()

	return nil
}
