package gitlab

import (
	"context"
	"fmt"
	"time"

	"github.com/arthurvasconcelos/overseer/internal/config"
	gitlabclient "github.com/arthurvasconcelos/overseer/internal/gitlab"
	"github.com/arthurvasconcelos/overseer/internal/output"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

var instanceFlag string

func commands(cfg *config.Config) []*cobra.Command {
	root := &cobra.Command{
		Use:         "gitlab",
		Short:       "GitLab interactions — issues, mrs, merged",
		Annotations: map[string]string{"overseer/group": "Dev"},
	}
	root.PersistentFlags().StringVar(&instanceFlag, "instance", "", "GitLab instance name (auto-selects if only one configured)")
	root.AddCommand(issuesCmd())
	root.AddCommand(mrsCmd())
	root.AddCommand(mergedCmd())
	return []*cobra.Command{root}
}

func resolveInstance(cfg *config.Config, name string) (config.GitLabInstance, error) {
	if len(cfg.Integrations.GitLab) == 0 {
		return config.GitLabInstance{}, fmt.Errorf("no GitLab instances configured")
	}
	if name != "" {
		for _, inst := range cfg.Integrations.GitLab {
			if inst.Name == name {
				return inst, nil
			}
		}
		return config.GitLabInstance{}, fmt.Errorf("GitLab instance %q not found", name)
	}
	if len(cfg.Integrations.GitLab) == 1 {
		return cfg.Integrations.GitLab[0], nil
	}
	items := make([]tui.SelectItem, len(cfg.Integrations.GitLab))
	for i, inst := range cfg.Integrations.GitLab {
		items[i] = tui.SelectItem{Title: inst.Name, Subtitle: tui.StyleMuted.Render(inst.BaseURL)}
	}
	idx, err := tui.Select("Select GitLab instance", items)
	if err != nil {
		return config.GitLabInstance{}, err
	}
	if idx < 0 {
		return config.GitLabInstance{}, fmt.Errorf("no instance selected")
	}
	return cfg.Integrations.GitLab[idx], nil
}

func buildClient(inst config.GitLabInstance) (*gitlabclient.Client, error) {
	token, err := secrets.ReadAs(inst.Token, inst.OPAccount)
	if err != nil {
		return nil, fmt.Errorf("resolving token: %w", err)
	}
	return gitlabclient.New(inst.BaseURL, token), nil
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
					issues = []gitlabclient.Issue{}
				}
				return output.PrintJSON(issues)
			}
			badge := pluralize(len(issues), "open issue", "open issues")
			fmt.Println(tui.SectionHeader("GitLab / "+inst.Name+" — Issues", badge))
			if len(issues) == 0 {
				fmt.Println("  " + tui.StyleMuted.Render("no open issues"))
				return nil
			}
			for _, i := range issues {
				num := tui.StyleAccent.Render(fmt.Sprintf("#%-5d", i.IID))
				proj := tui.StyleMuted.Render(i.Project)
				fmt.Printf("  %s  %-40s  %s\n", num, proj, tui.StyleNormal.Render(i.Title))
			}
			return nil
		},
	}
}

func mrsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mrs",
		Short: "List open merge requests assigned to or created by you",
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
			mrs, err := client.MyMRs(ctx)
			if err != nil {
				return err
			}
			if output.Format == "json" {
				if mrs == nil {
					mrs = []gitlabclient.MR{}
				}
				return output.PrintJSON(mrs)
			}
			badge := pluralize(len(mrs), "open MR", "open MRs")
			fmt.Println(tui.SectionHeader("GitLab / "+inst.Name+" — Merge Requests", badge))
			if len(mrs) == 0 {
				fmt.Println("  " + tui.StyleMuted.Render("no open merge requests"))
				return nil
			}
			for _, mr := range mrs {
				num := tui.StyleAccent.Render(fmt.Sprintf("!%-5d", mr.IID))
				proj := tui.StyleMuted.Render(mr.Project)
				ci := ciIndicator(mr.CI)
				draft := ""
				if mr.Draft {
					draft = tui.StyleMuted.Render(" [draft]")
				}
				fmt.Printf("  %s  %-40s  %s%s%s\n", num, proj, tui.StyleNormal.Render(mr.Title), draft, ci)
			}
			return nil
		},
	}
}

func mergedCmd() *cobra.Command {
	var sinceDays int
	cmd := &cobra.Command{
		Use:   "merged",
		Short: "List recently merged merge requests",
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
			mrs, err := client.MergedMRs(ctx, since)
			if err != nil {
				return err
			}
			if output.Format == "json" {
				if mrs == nil {
					mrs = []gitlabclient.MR{}
				}
				return output.PrintJSON(mrs)
			}
			badge := pluralize(len(mrs), "merged MR", "merged MRs")
			fmt.Println(tui.SectionHeader("GitLab / "+inst.Name+" — Merged", badge))
			if len(mrs) == 0 {
				fmt.Println("  " + tui.StyleMuted.Render("no recently merged MRs"))
				return nil
			}
			for _, mr := range mrs {
				num := tui.StyleAccent.Render(fmt.Sprintf("!%-5d", mr.IID))
				proj := tui.StyleMuted.Render(mr.Project)
				fmt.Printf("  %s  %-40s  %s\n", num, proj, tui.StyleNormal.Render(mr.Title))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&sinceDays, "days", 7, "how many days back to look")
	return cmd
}

func ciIndicator(ci gitlabclient.CIStatus) string {
	switch ci {
	case gitlabclient.CIPass:
		return "  " + tui.StyleOK.Render("✓")
	case gitlabclient.CIFail:
		return "  " + tui.StyleError.Render("✗")
	case gitlabclient.CIRunning:
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
