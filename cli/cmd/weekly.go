package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/arthurvasconcelos/overseer/internal/output"
	"github.com/arthurvasconcelos/overseer/internal/config"
	githubclient "github.com/arthurvasconcelos/overseer/internal/github"
	gitlabclient "github.com/arthurvasconcelos/overseer/internal/gitlab"
	jiraclient "github.com/arthurvasconcelos/overseer/internal/jira"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

var weeklyCmd = &cobra.Command{
	Use:   "weekly",
	Short: "Activity summary for the past 7 days",
	RunE:  runWeekly,
}

var weeklyCopy bool

func init() {
	weeklyCmd.Flags().BoolVar(&weeklyCopy, "copy", false, "Copy output to clipboard (macOS)")
	rootCmd.AddCommand(weeklyCmd)
}

// weeklyReport is the JSON-serializable output for --format json.
type weeklyReport struct {
	Period  string          `json:"period"`
	GitHub  []weeklyPR      `json:"github"`
	GitLab  []weeklyMR      `json:"gitlab"`
	Jira    []weeklyIssue   `json:"jira"`
	Commits []weeklyRepo    `json:"commits"`
}

type weeklyPR struct {
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
}

type weeklyMR struct {
	Project string `json:"project"`
	IID     int    `json:"iid"`
	Title   string `json:"title"`
	URL     string `json:"url"`
}

type weeklyIssue struct {
	Instance string `json:"instance"`
	Key      string `json:"key"`
	Summary  string `json:"summary"`
	Status   string `json:"status"`
}

type weeklyRepo struct {
	Name    string   `json:"name"`
	Commits []string `json:"commits"`
}

func runWeekly(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)
	since := time.Date(weekAgo.Year(), weekAgo.Month(), weekAgo.Day(), 0, 0, 0, 0, weekAgo.Location())

	if output.Format == "json" {
		return runWeeklyJSON(ctx, cfg, since, now)
	}

	fmt.Println(tui.StyleHeader.Render("Weekly Summary") + "  " +
		tui.StyleMuted.Render(since.Format("Jan 02")+" – "+now.Format("Jan 02, 2006")))
	fmt.Println()

	type result struct {
		output string
		err    error
	}

	type task struct {
		label string
		run   func() (string, error)
	}

	var tasks []task

	// GitHub merged PRs — one task per instance.
	for _, inst := range cfg.Integrations.GitHub {
		inst := inst
		tasks = append(tasks, task{
			label: "github/" + inst.Name,
			run: func() (string, error) {
				token, err := secrets.ReadAs(inst.Token, inst.OPAccount)
				if err != nil {
					return "", err
				}
				prs, err := githubclient.New(token).MergedPRs(ctx, since)
				if err != nil {
					return "", err
				}
				return formatWeeklyGitHubPRs(inst.Name, prs), nil
			},
		})
	}

	// GitLab merged MRs — one task per instance.
	for _, inst := range cfg.Integrations.GitLab {
		inst := inst
		tasks = append(tasks, task{
			label: "gitlab/" + inst.Name,
			run: func() (string, error) {
				token, err := secrets.ReadAs(inst.Token, inst.OPAccount)
				if err != nil {
					return "", err
				}
				mrs, err := gitlabclient.New(inst.BaseURL, token).MergedMRs(ctx, since)
				if err != nil {
					return "", err
				}
				return formatWeeklyGitLabMRs(inst.Name, mrs), nil
			},
		})
	}

	// Jira closed issues — one task per instance.
	doneStatuses := []string{"Done", "In Review", "Code Review", "Resolved", "Closed"}
	for _, inst := range cfg.Integrations.Jira {
		inst := inst
		tasks = append(tasks, task{
			label: "jira/" + inst.Name,
			run: func() (string, error) {
				email, err := secrets.ReadAs(inst.Email, inst.OPAccount)
				if err != nil {
					return "", err
				}
				token, err := secrets.ReadAs(inst.Token, inst.OPAccount)
				if err != nil {
					return "", err
				}
				issues, err := jiraclient.New(inst.BaseURL, email, token).RecentlyUpdated(ctx, doneStatuses, since)
				if err != nil {
					return "", err
				}
				return formatWeeklyJiraIssues(inst.Name, inst.BaseURL, issues), nil
			},
		})
	}

	// Git commits by repo — single task covering all repos.
	tasks = append(tasks, task{
		label: "git",
		run: func() (string, error) {
			profileEmails := collectProfileEmails(cfg)
			home, err := resolveReposPath(cfg)
			if err != nil {
				return "", err
			}
			return formatWeeklyCommits(cfg, home, since, profileEmails), nil
		},
	})

	stopSpinner := tui.StartSpinner("loading weekly summary…")
	results := make([]result, len(tasks))
	var wg sync.WaitGroup
	for i, t := range tasks {
		i, t := i, t
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := t.run()
			results[i] = result{output: out, err: err}
		}()
	}
	wg.Wait()
	stopSpinner()

	var body strings.Builder
	for i, t := range tasks {
		if results[i].err != nil {
			body.WriteString(tui.WarnLine(t.label, results[i].err.Error()) + "\n\n")
		} else if results[i].output != "" {
			body.WriteString(results[i].output)
		}
	}

	output := body.String()
	fmt.Print(output)

	if weeklyCopy {
		if err := copyToClipboard(stripANSI(output)); err != nil {
			fmt.Println(tui.WarnLine("copy", err.Error()))
		}
	}

	return nil
}

func runWeeklyJSON(ctx context.Context, cfg *config.Config, since, now time.Time) error {
	report := weeklyReport{
		Period:  since.Format("2006-01-02") + "/" + now.Format("2006-01-02"),
		GitHub:  []weeklyPR{},
		GitLab:  []weeklyMR{},
		Jira:    []weeklyIssue{},
		Commits: []weeklyRepo{},
	}

	for _, inst := range cfg.Integrations.GitHub {
		token, err := secrets.ReadAs(inst.Token, inst.OPAccount)
		if err != nil {
			continue
		}
		prs, err := githubclient.New(token).MergedPRs(ctx, since)
		if err != nil {
			continue
		}
		for _, pr := range prs {
			report.GitHub = append(report.GitHub, weeklyPR{
				Repo:   pr.Repo,
				Number: pr.Number,
				Title:  pr.Title,
				URL:    pr.URL,
			})
		}
	}

	for _, inst := range cfg.Integrations.GitLab {
		token, err := secrets.ReadAs(inst.Token, inst.OPAccount)
		if err != nil {
			continue
		}
		mrs, err := gitlabclient.New(inst.BaseURL, token).MergedMRs(ctx, since)
		if err != nil {
			continue
		}
		for _, mr := range mrs {
			report.GitLab = append(report.GitLab, weeklyMR{
				Project: mr.Project,
				IID:     mr.IID,
				Title:   mr.Title,
				URL:     mr.URL,
			})
		}
	}

	doneStatuses := []string{"Done", "In Review", "Code Review", "Resolved", "Closed"}
	for _, inst := range cfg.Integrations.Jira {
		email, err := secrets.ReadAs(inst.Email, inst.OPAccount)
		if err != nil {
			continue
		}
		token, err := secrets.ReadAs(inst.Token, inst.OPAccount)
		if err != nil {
			continue
		}
		issues, err := jiraclient.New(inst.BaseURL, email, token).RecentlyUpdated(ctx, doneStatuses, since)
		if err != nil {
			continue
		}
		for _, i := range issues {
			report.Jira = append(report.Jira, weeklyIssue{
				Instance: inst.Name,
				Key:      i.Key,
				Summary:  i.Summary,
				Status:   i.Status,
			})
		}
	}

	profileEmails := collectProfileEmails(cfg)
	home, _ := resolveReposPath(cfg)
	for _, repo := range cfg.Repos {
		path := repoRoot(home, repo)
		commits := gitCommitsSince(path, since, profileEmails)
		if len(commits) > 0 {
			report.Commits = append(report.Commits, weeklyRepo{
				Name:    repo.Name,
				Commits: commits,
			})
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func formatWeeklyGitHubPRs(name string, prs []githubclient.PR) string {
	if len(prs) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(tui.SectionHeader("GitHub / "+name, fmt.Sprintf("%d PR(s) merged", len(prs))) + "\n")
	for _, pr := range prs {
		title := tui.Hyperlink(pr.URL, pr.Title)
		sb.WriteString(fmt.Sprintf("  %s  %s\n",
			tui.StyleMuted.Render(fmt.Sprintf("#%d", pr.Number)),
			title,
		))
		sb.WriteString(fmt.Sprintf("       %s\n", tui.StyleMuted.Render(pr.Repo)))
	}
	sb.WriteString("\n")
	return sb.String()
}

func formatWeeklyGitLabMRs(name string, mrs []gitlabclient.MR) string {
	if len(mrs) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(tui.SectionHeader("GitLab / "+name, fmt.Sprintf("%d MR(s) merged", len(mrs))) + "\n")
	for _, mr := range mrs {
		title := tui.Hyperlink(mr.URL, mr.Title)
		sb.WriteString(fmt.Sprintf("  %s  %s\n",
			tui.StyleMuted.Render(fmt.Sprintf("!%d", mr.IID)),
			title,
		))
		sb.WriteString(fmt.Sprintf("       %s\n", tui.StyleMuted.Render(mr.Project)))
	}
	sb.WriteString("\n")
	return sb.String()
}

func formatWeeklyJiraIssues(name, baseURL string, issues []jiraclient.Issue) string {
	if len(issues) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(tui.SectionHeader("Jira / "+name, fmt.Sprintf("%d issue(s) closed", len(issues))) + "\n")
	for _, i := range issues {
		key := tui.Hyperlink(baseURL+"/browse/"+i.Key, tui.StyleAccent.Render(i.Key))
		sb.WriteString(fmt.Sprintf("  %s  %s  %s\n",
			key,
			i.Summary,
			tui.StyleMuted.Render("("+i.Status+")"),
		))
	}
	sb.WriteString("\n")
	return sb.String()
}

func formatWeeklyCommits(cfg *config.Config, home string, since time.Time, emails []string) string {
	type repoCommits struct {
		name    string
		commits []string
	}
	var repos []repoCommits
	for _, repo := range cfg.Repos {
		path := repoRoot(home, repo)
		commits := gitCommitsSince(path, since, emails)
		if len(commits) > 0 {
			repos = append(repos, repoCommits{name: repo.Name, commits: commits})
		}
	}
	if len(repos) == 0 {
		return ""
	}
	var sb strings.Builder
	total := 0
	for _, r := range repos {
		total += len(r.commits)
	}
	sb.WriteString(tui.SectionHeader("Commits", fmt.Sprintf("%d commit(s) across %d repo(s)", total, len(repos))))
	for _, r := range repos {
		sb.WriteString(fmt.Sprintf("  %s  %s\n",
			tui.StyleAccent.Render(r.name),
			tui.StyleMuted.Render(fmt.Sprintf("%d commit(s)", len(r.commits))),
		))
		for _, c := range r.commits {
			sb.WriteString(fmt.Sprintf("    %s\n", c))
		}
	}
	sb.WriteString("\n")
	return sb.String()
}
