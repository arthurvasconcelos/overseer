package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/jira"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
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
