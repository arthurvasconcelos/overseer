package github

import (
	"bytes"
	"context"
	"fmt"

	"github.com/arthurvasconcelos/overseer/internal/config"
	githubclient "github.com/arthurvasconcelos/overseer/internal/github"
	"github.com/arthurvasconcelos/overseer/internal/nativeplugin"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	"github.com/arthurvasconcelos/overseer/internal/tui"
)

func init() {
	nativeplugin.Register(&nativeplugin.Plugin{
		Name:        "github",
		Description: "GitHub pull requests",
		IsEnabled:   isEnabled,
		DailyItems:  dailyItems,
	})
}

func isEnabled(cfg *config.Config) bool {
	if s, ok := cfg.Plugins.Settings["github"]; ok {
		return s.Enabled
	}
	return len(cfg.Integrations.GitHub) > 0
}

func dailyItems(cfg *config.Config) []nativeplugin.DailyTask {
	var tasks []nativeplugin.DailyTask
	for _, inst := range cfg.Integrations.GitHub {
		if !inst.ShowIssues {
			continue
		}
		inst := inst
		tasks = append(tasks, nativeplugin.DailyTask{
			Label: "github/" + inst.Name + "/issues",
			Run: func(ctx context.Context, _ *config.Config) (string, error) {
				var b bytes.Buffer
				if err := printGitHubIssues(ctx, inst, &b); err != nil {
					return "", err
				}
				return b.String(), nil
			},
		})
	}
	return tasks
}

func printGitHubIssues(ctx context.Context, inst config.GitHubInstance, w *bytes.Buffer) error {
	token, err := secrets.ReadAs(inst.Token, inst.OPAccount)
	if err != nil {
		return err
	}

	client := githubclient.New(token)
	issues, err := client.MyIssues(ctx)
	if err != nil {
		return err
	}

	badge := fmt.Sprintf("%d open", len(issues))
	fmt.Fprintln(w, tui.SectionHeader("GitHub Issues / "+inst.Name, badge))
	if len(issues) == 0 {
		fmt.Fprintln(w, "  "+tui.StyleMuted.Render("no open issues"))
	}
	for _, i := range issues {
		num := tui.StyleAccent.Render(fmt.Sprintf("#%-5d", i.Number))
		repo := tui.StyleMuted.Render(i.Repo)
		fmt.Fprintf(w, "  %s  %s  %s\n", num, repo, tui.StyleNormal.Render(i.Title))
	}
	fmt.Fprintln(w)

	return nil
}
