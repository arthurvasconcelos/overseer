package github

import (
	"context"
	"fmt"
	"time"

	"github.com/arthurvasconcelos/overseer/internal/config"
	githubclient "github.com/arthurvasconcelos/overseer/internal/github"
	"github.com/arthurvasconcelos/overseer/internal/output"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

var instanceFlag string

func commands(cfg *config.Config) []*cobra.Command {
	root := &cobra.Command{
		Use:         "github",
		Short:       "GitHub interactions — issues, prs, merged",
		Annotations: map[string]string{"overseer/group": "Dev"},
	}
	root.PersistentFlags().StringVar(&instanceFlag, "instance", "", "GitHub account name (auto-selects if only one configured)")
	root.AddCommand(issuesCmd())
	root.AddCommand(prsCmd())
	root.AddCommand(mergedCmd())
	return []*cobra.Command{root}
}

func resolveInstance(cfg *config.Config, name string) (config.GitHubInstance, error) {
	if len(cfg.Integrations.GitHub) == 0 {
		return config.GitHubInstance{}, fmt.Errorf("no GitHub accounts configured")
	}
	if name != "" {
		for _, inst := range cfg.Integrations.GitHub {
			if inst.Name == name {
				return inst, nil
			}
		}
		return config.GitHubInstance{}, fmt.Errorf("GitHub account %q not found", name)
	}
	if len(cfg.Integrations.GitHub) == 1 {
		return cfg.Integrations.GitHub[0], nil
	}
	items := make([]tui.SelectItem, len(cfg.Integrations.GitHub))
	for i, inst := range cfg.Integrations.GitHub {
		items[i] = tui.SelectItem{Title: inst.Name}
	}
	idx, err := tui.Select("Select GitHub account", items)
	if err != nil {
		return config.GitHubInstance{}, err
	}
	if idx < 0 {
		return config.GitHubInstance{}, fmt.Errorf("no account selected")
	}
	return cfg.Integrations.GitHub[idx], nil
}

func buildClient(inst config.GitHubInstance) (*githubclient.Client, error) {
	token, err := secrets.ReadAs(inst.Token, inst.OPAccount)
	if err != nil {
		return nil, fmt.Errorf("resolving token: %w", err)
	}
	return githubclient.New(token), nil
}

func issuesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "issues",
		Short: "List open issues assigned to you",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			inst, err := resolveInstance(cfg, instanceFlag)
			if err != nil {
				return err
			}
			client, err := buildClient(inst)
			if err != nil {
				return err
			}
			issues, err := client.MyIssues(ctx)
			if err != nil {
				return err
			}
			if output.Format == "json" {
				if issues == nil {
					issues = []githubclient.Issue{}
				}
				return output.PrintJSON(issues)
			}
			badge := pluralize(len(issues), "open issue", "open issues")
			fmt.Println(tui.SectionHeader("GitHub / "+inst.Name+" — Issues", badge))
			if len(issues) == 0 {
				fmt.Println("  " + tui.StyleMuted.Render("no open issues"))
				return nil
			}
			for _, i := range issues {
				num := tui.StyleAccent.Render(fmt.Sprintf("#%-5d", i.Number))
				repo := tui.StyleMuted.Render(i.Repo)
				fmt.Printf("  %s  %-30s  %s\n", num, repo, tui.StyleNormal.Render(i.Title))
			}
			return nil
		},
	}
}

func prsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "prs",
		Short: "List open pull requests involving you",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			inst, err := resolveInstance(cfg, instanceFlag)
			if err != nil {
				return err
			}
			client, err := buildClient(inst)
			if err != nil {
				return err
			}
			prs, err := client.MyPRs(ctx)
			if err != nil {
				return err
			}
			if output.Format == "json" {
				if prs == nil {
					prs = []githubclient.PR{}
				}
				return output.PrintJSON(prs)
			}
			badge := pluralize(len(prs), "open PR", "open PRs")
			fmt.Println(tui.SectionHeader("GitHub / "+inst.Name+" — Pull Requests", badge))
			if len(prs) == 0 {
				fmt.Println("  " + tui.StyleMuted.Render("no open pull requests"))
				return nil
			}
			for _, pr := range prs {
				num := tui.StyleAccent.Render(fmt.Sprintf("#%-5d", pr.Number))
				repo := tui.StyleMuted.Render(pr.Repo)
				ci := ciIndicator(pr.CI)
				draft := ""
				if pr.Draft {
					draft = tui.StyleMuted.Render(" [draft]")
				}
				fmt.Printf("  %s  %-30s  %s%s%s\n", num, repo, tui.StyleNormal.Render(pr.Title), draft, ci)
			}
			return nil
		},
	}
}

func mergedCmd() *cobra.Command {
	var sinceDays int
	cmd := &cobra.Command{
		Use:   "merged",
		Short: "List recently merged pull requests",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			inst, err := resolveInstance(cfg, instanceFlag)
			if err != nil {
				return err
			}
			client, err := buildClient(inst)
			if err != nil {
				return err
			}
			since := time.Now().AddDate(0, 0, -sinceDays)
			prs, err := client.MergedPRs(ctx, since)
			if err != nil {
				return err
			}
			if output.Format == "json" {
				if prs == nil {
					prs = []githubclient.PR{}
				}
				return output.PrintJSON(prs)
			}
			badge := pluralize(len(prs), "merged PR", "merged PRs")
			fmt.Println(tui.SectionHeader("GitHub / "+inst.Name+" — Merged", badge))
			if len(prs) == 0 {
				fmt.Println("  " + tui.StyleMuted.Render("no recently merged PRs"))
				return nil
			}
			for _, pr := range prs {
				num := tui.StyleAccent.Render(fmt.Sprintf("#%-5d", pr.Number))
				repo := tui.StyleMuted.Render(pr.Repo)
				fmt.Printf("  %s  %-30s  %s\n", num, repo, tui.StyleNormal.Render(pr.Title))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&sinceDays, "days", 7, "how many days back to look")
	return cmd
}

func ciIndicator(ci githubclient.CIStatus) string {
	switch ci {
	case githubclient.CIPass:
		return "  " + tui.StyleOK.Render("✓")
	case githubclient.CIFail:
		return "  " + tui.StyleError.Render("✗")
	case githubclient.CIRunning:
		return "  " + tui.StyleMuted.Render("…")
	default:
		return ""
	}
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}
