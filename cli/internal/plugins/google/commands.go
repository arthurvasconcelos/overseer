package google

import (
	"context"
	"fmt"
	"time"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/gcal"
	"github.com/arthurvasconcelos/overseer/internal/output"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

var accountFlag string

func commands(cfg *config.Config) []*cobra.Command {
	root := &cobra.Command{
		Use:         "gcal",
		Short:       "Google Calendar — today, week, next event",
		Annotations: map[string]string{"overseer/group": "Daily"},
	}
	root.PersistentFlags().StringVar(&accountFlag, "account", "", "Google account name (auto-selects if only one configured)")
	root.AddCommand(todayCmd())
	root.AddCommand(weekCmd())
	root.AddCommand(nextCmd())
	return []*cobra.Command{root}
}

func resolveAccount(cfg *config.Config, name string) (config.GoogleAccount, error) {
	if len(cfg.Integrations.Google) == 0 {
		return config.GoogleAccount{}, fmt.Errorf("no Google accounts configured")
	}
	if name != "" {
		for _, acc := range cfg.Integrations.Google {
			if acc.Name == name {
				return acc, nil
			}
		}
		return config.GoogleAccount{}, fmt.Errorf("Google account %q not found", name)
	}
	if len(cfg.Integrations.Google) == 1 {
		return cfg.Integrations.Google[0], nil
	}
	items := make([]tui.SelectItem, len(cfg.Integrations.Google))
	for i, acc := range cfg.Integrations.Google {
		items[i] = tui.SelectItem{Title: acc.Name}
	}
	idx, err := tui.Select("Select Google account", items)
	if err != nil {
		return config.GoogleAccount{}, err
	}
	if idx < 0 {
		return config.GoogleAccount{}, fmt.Errorf("no account selected")
	}
	return cfg.Integrations.Google[idx], nil
}

func buildCalClient(ctx context.Context, acc config.GoogleAccount) (*gcal.Client, error) {
	credsJSON, err := secrets.ReadAs(acc.CredentialsDoc, acc.OPAccount)
	if err != nil {
		return nil, fmt.Errorf("resolving credentials: %w", err)
	}
	return gcal.New(ctx, []byte(credsJSON), acc.Name)
}

func printEvents(events []gcal.Event, showDate bool) {
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
		if showDate {
			day := tui.StyleMuted.Render(e.Start.Format("Mon 02/01"))
			fmt.Printf("  %s  %s  %s\n", day, timeCol, title)
		} else {
			fmt.Printf("  %s  %s\n", timeCol, title)
		}
	}
}

func todayCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "today",
		Short: "Show today's calendar events",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			acc, err := resolveAccount(cfg, accountFlag)
			if err != nil {
				return err
			}
			client, err := buildCalClient(ctx, acc)
			if err != nil {
				return err
			}
			events, err := client.TodaysEvents(ctx)
			if err != nil {
				return err
			}
			if output.Format == "json" {
				return output.PrintJSON(events)
			}
			badge := pluralize(len(events), "event today", "events today")
			fmt.Println(tui.SectionHeader("Google Calendar / "+acc.Name, badge))
			if len(events) == 0 {
				fmt.Println("  " + tui.StyleMuted.Render("no events today"))
				return nil
			}
			printEvents(events, false)
			return nil
		},
	}
}

func weekCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "week",
		Short: "Show events for the next 7 days",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			acc, err := resolveAccount(cfg, accountFlag)
			if err != nil {
				return err
			}
			client, err := buildCalClient(ctx, acc)
			if err != nil {
				return err
			}
			events, err := client.WeekEvents(ctx)
			if err != nil {
				return err
			}
			if output.Format == "json" {
				return output.PrintJSON(events)
			}
			badge := pluralize(len(events), "event this week", "events this week")
			fmt.Println(tui.SectionHeader("Google Calendar / "+acc.Name, badge))
			if len(events) == 0 {
				fmt.Println("  " + tui.StyleMuted.Render("no events this week"))
				return nil
			}
			printEvents(events, true)
			return nil
		},
	}
}

func nextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "next",
		Short: "Show the next upcoming event",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			acc, err := resolveAccount(cfg, accountFlag)
			if err != nil {
				return err
			}
			client, err := buildCalClient(ctx, acc)
			if err != nil {
				return err
			}
			event, err := client.NextEvent(ctx)
			if err != nil {
				return err
			}
			if output.Format == "json" {
				if event == nil {
					return output.PrintJSON([]any{})
				}
				return output.PrintJSON(event)
			}
			fmt.Println(tui.SectionHeader("Google Calendar / "+acc.Name, "next event"))
			if event == nil {
				fmt.Println("  " + tui.StyleMuted.Render("no upcoming events in the next 24h"))
				return nil
			}
			var timeCol string
			if event.AllDay {
				timeCol = tui.StyleMuted.Render("all day      ")
			} else {
				timeCol = tui.StyleAccent.Render(event.Start.Format("15:04") + " – " + event.End.Format("15:04"))
				until := time.Until(event.Start)
				if until > 0 {
					timeCol += "  " + tui.StyleMuted.Render("(in "+formatDuration(until)+")")
				}
			}
			title := tui.StyleNormal.Render(event.Title)
			if event.JoinURL != "" {
				title += "  " + tui.StyleAccent.Render(tui.Hyperlink(event.JoinURL, "[Join]"))
			}
			fmt.Printf("  %s  %s\n", timeCol, title)
			return nil
		},
	}
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

