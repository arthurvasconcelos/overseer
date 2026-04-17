package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/gcal"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

var gcalAccountFlag string

var gcalCmd = &cobra.Command{
	Use:   "gcal",
	Short: "Google Calendar — today, week, next event",
}

func init() {
	gcalCmd.PersistentFlags().StringVar(&gcalAccountFlag, "account", "", "Google account name (auto-selects if only one configured)")
	gcalCmd.AddCommand(gcalTodayCmd())
	gcalCmd.AddCommand(gcalWeekCmd())
	gcalCmd.AddCommand(gcalNextCmd())
	rootCmd.AddCommand(gcalCmd)
}

func resolveGoogleAccount(cfg *config.Config, name string) (config.GoogleAccount, error) {
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

func buildGCalClient(ctx context.Context, acc config.GoogleAccount) (*gcal.Client, error) {
	credsJSON, err := secrets.ReadAs(acc.CredentialsDoc, acc.OPAccount)
	if err != nil {
		return nil, fmt.Errorf("resolving credentials: %w", err)
	}
	return gcal.New(ctx, []byte(credsJSON), acc.Name)
}

func printEvents(events []gcal.Event, label string) {
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
		if label != "" {
			day := tui.StyleMuted.Render(e.Start.Format("Mon 02/01"))
			fmt.Printf("  %s  %s  %s\n", day, timeCol, title)
		} else {
			fmt.Printf("  %s  %s\n", timeCol, title)
		}
	}
}

func gcalTodayCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "today",
		Short: "Show today's calendar events",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			acc, err := resolveGoogleAccount(cfg, gcalAccountFlag)
			if err != nil {
				return err
			}
			client, err := buildGCalClient(ctx, acc)
			if err != nil {
				return err
			}
			events, err := client.TodaysEvents(ctx)
			if err != nil {
				return err
			}
			if outputFormat == "json" {
				return printJSON(events)
			}
			badge := pluralize(len(events), "event today", "events today")
			fmt.Println(tui.SectionHeader("Google Calendar / "+acc.Name, badge))
			if len(events) == 0 {
				fmt.Println("  " + tui.StyleMuted.Render("no events today"))
				return nil
			}
			printEvents(events, "")
			return nil
		},
	}
}

func gcalWeekCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "week",
		Short: "Show events for the next 7 days",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			acc, err := resolveGoogleAccount(cfg, gcalAccountFlag)
			if err != nil {
				return err
			}
			client, err := buildGCalClient(ctx, acc)
			if err != nil {
				return err
			}
			events, err := client.WeekEvents(ctx)
			if err != nil {
				return err
			}
			if outputFormat == "json" {
				return printJSON(events)
			}
			badge := pluralize(len(events), "event this week", "events this week")
			fmt.Println(tui.SectionHeader("Google Calendar / "+acc.Name, badge))
			if len(events) == 0 {
				fmt.Println("  " + tui.StyleMuted.Render("no events this week"))
				return nil
			}
			printEvents(events, "week")
			return nil
		},
	}
}

func gcalNextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "next",
		Short: "Show the next upcoming event",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			acc, err := resolveGoogleAccount(cfg, gcalAccountFlag)
			if err != nil {
				return err
			}
			client, err := buildGCalClient(ctx, acc)
			if err != nil {
				return err
			}
			event, err := client.NextEvent(ctx)
			if err != nil {
				return err
			}
			if outputFormat == "json" {
				if event == nil {
					return printJSON([]any{})
				}
				return printJSON(event)
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
					timeCol += "  " + tui.StyleMuted.Render("(in "+formatDurationGCal(until)+")")
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

func formatDurationGCal(d time.Duration) string {
	d = d.Round(time.Minute)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
