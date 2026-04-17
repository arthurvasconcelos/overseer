package slack

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/output"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	slackclient "github.com/arthurvasconcelos/overseer/internal/slack"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

var instanceFlag string

func commands(cfg *config.Config) []*cobra.Command {
	root := &cobra.Command{
		Use:         "slack",
		Short:       "Slack interactions — mentions, channels, send",
		Annotations: map[string]string{"overseer/group": "Dev"},
	}
	root.PersistentFlags().StringVar(&instanceFlag, "instance", "", "Slack workspace name (auto-selects if only one configured)")
	root.AddCommand(mentionsCmd())
	root.AddCommand(channelsCmd())
	root.AddCommand(sendCmd())
	return []*cobra.Command{root}
}

func resolveWorkspace(cfg *config.Config, name string) (config.SlackWorkspace, error) {
	if len(cfg.Integrations.Slack) == 0 {
		return config.SlackWorkspace{}, fmt.Errorf("no Slack workspaces configured")
	}
	if name != "" {
		for _, ws := range cfg.Integrations.Slack {
			if ws.Name == name {
				return ws, nil
			}
		}
		return config.SlackWorkspace{}, fmt.Errorf("Slack workspace %q not found", name)
	}
	if len(cfg.Integrations.Slack) == 1 {
		return cfg.Integrations.Slack[0], nil
	}
	items := make([]tui.SelectItem, len(cfg.Integrations.Slack))
	for i, ws := range cfg.Integrations.Slack {
		items[i] = tui.SelectItem{Title: ws.Name}
	}
	idx, err := tui.Select("Select Slack workspace", items)
	if err != nil {
		return config.SlackWorkspace{}, err
	}
	if idx < 0 {
		return config.SlackWorkspace{}, fmt.Errorf("no workspace selected")
	}
	return cfg.Integrations.Slack[idx], nil
}

func buildClient(ws config.SlackWorkspace) (*slackclient.Client, error) {
	token, err := secrets.ReadAs(ws.Token, ws.OPAccount)
	if err != nil {
		return nil, fmt.Errorf("resolving token: %w", err)
	}
	return slackclient.New(token), nil
}

func mentionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mentions",
		Short: "Show recent messages that mention you",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			ws, err := resolveWorkspace(cfg, instanceFlag)
			if err != nil {
				return err
			}
			client, err := buildClient(ws)
			if err != nil {
				return err
			}
			mentions, err := client.Mentions()
			if err != nil {
				return err
			}
			if output.Format == "json" {
				return output.PrintJSON(mentions)
			}
			fmt.Println(tui.SectionHeader("Slack / "+ws.Name+" — Mentions", fmt.Sprintf("%d", len(mentions))))
			if len(mentions) == 0 {
				fmt.Println("  " + tui.StyleMuted.Render("no recent mentions"))
				return nil
			}
			for _, m := range mentions {
				channel := tui.StyleAccent.Render("#" + m.Channel)
				fmt.Printf("  %-30s  %s\n", channel, tui.StyleNormal.Render(m.Text))
			}
			return nil
		},
	}
}

func channelsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "channels",
		Short: "List channels the bot is a member of",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			ws, err := resolveWorkspace(cfg, instanceFlag)
			if err != nil {
				return err
			}
			client, err := buildClient(ws)
			if err != nil {
				return err
			}
			channels, err := client.Channels()
			if err != nil {
				return err
			}
			if output.Format == "json" {
				return output.PrintJSON(channels)
			}
			fmt.Println(tui.SectionHeader("Slack / "+ws.Name+" — Channels", fmt.Sprintf("%d", len(channels))))
			if len(channels) == 0 {
				fmt.Println("  " + tui.StyleMuted.Render("no channels found"))
				return nil
			}
			for _, ch := range channels {
				kind := tui.StyleMuted.Render("public ")
				if ch.Private {
					kind = tui.StyleMuted.Render("private")
				}
				fmt.Printf("  %s  %s\n", kind, tui.StyleAccent.Render("#"+ch.Name))
			}
			return nil
		},
	}
}

func sendCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "send <channel> <message>",
		Short: "Post a message to a channel",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			channel, message := args[0], args[1]
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			ws, err := resolveWorkspace(cfg, instanceFlag)
			if err != nil {
				return err
			}
			client, err := buildClient(ws)
			if err != nil {
				return err
			}
			if err := client.Send(channel, message); err != nil {
				return err
			}
			if output.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "sent", "channel": channel})
			}
			fmt.Println(tui.StyleOK.Render("✓") + "  sent to " + tui.StyleAccent.Render("#"+channel))
			return nil
		},
	}
}
