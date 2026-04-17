package cmd

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/arthurvasconcelos/overseer/internal/output"
	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/github"
	"github.com/arthurvasconcelos/overseer/internal/gitlab"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var prsCmd = &cobra.Command{
	Use:   "prs",
	Short: "Open pull requests and merge requests across GitHub and GitLab",
	RunE:  runPRs,
}

var prsCopy bool

func init() {
	prsCmd.Flags().BoolVar(&prsCopy, "copy", false, "Copy output to clipboard (macOS)")
	rootCmd.AddCommand(prsCmd)
}

func runPRs(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if len(cfg.Integrations.GitHub) == 0 && len(cfg.Integrations.GitLab) == 0 {
		fmt.Println(tui.StyleMuted.Render("no GitHub or GitLab instances configured"))
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	type task struct {
		label string
		run   func() (string, error)
	}

	var tasks []task

	for _, inst := range cfg.Integrations.GitHub {
		inst := inst
		tasks = append(tasks, task{
			label: "github/" + inst.Name,
			run: func() (string, error) {
				var b bytes.Buffer
				if err := printGitHubPRs(ctx, inst, &b); err != nil {
					return "", err
				}
				return b.String(), nil
			},
		})
	}

	for _, inst := range cfg.Integrations.GitLab {
		inst := inst
		tasks = append(tasks, task{
			label: "gitlab/" + inst.Name,
			run: func() (string, error) {
				var b bytes.Buffer
				if err := printGitLabMRs(ctx, inst, &b); err != nil {
					return "", err
				}
				return b.String(), nil
			},
		})
	}

	if output.Format == "json" {
		return runPRsJSON(ctx, cfg)
	}

	stopSpinner := tui.StartSpinner("fetching pull requests…")
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
	stopSpinner()

	var total bytes.Buffer
	for i, t := range tasks {
		if results[i].err != nil {
			line := tui.WarnLine(t.label, results[i].err.Error()) + "\n\n"
			total.WriteString(line)
		} else {
			total.Write(results[i].buf.Bytes())
		}
	}
	fmt.Print(total.String())

	if prsCopy {
		if err := copyToClipboard(total.String()); err != nil {
			fmt.Println(tui.WarnLine("copy", err.Error()))
		}
	}
	return nil
}

type prsSectionJSON struct {
	Source   string `json:"source"`
	Instance string `json:"instance"`
	Error    string `json:"error,omitempty"`
}

type githubSectionJSON struct {
	prsSectionJSON
	Items []github.PR `json:"items"`
}

type gitlabSectionJSON struct {
	prsSectionJSON
	Items []gitlab.MR `json:"items"`
}

func runPRsJSON(ctx context.Context, cfg *config.Config) error {
	type ghResult struct {
		inst config.GitHubInstance
		prs  []github.PR
		err  error
	}
	type glResult struct {
		inst config.GitLabInstance
		mrs  []gitlab.MR
		err  error
	}

	ghResults := make([]ghResult, len(cfg.Integrations.GitHub))
	glResults := make([]glResult, len(cfg.Integrations.GitLab))

	var wg sync.WaitGroup
	for i, inst := range cfg.Integrations.GitHub {
		i, inst := i, inst
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, err := secrets.ReadAs(inst.Token, inst.OPAccount)
			if err != nil {
				ghResults[i] = ghResult{inst: inst, err: err}
				return
			}
			prs, err := github.New(token).MyPRs(ctx)
			ghResults[i] = ghResult{inst: inst, prs: prs, err: err}
		}()
	}
	for i, inst := range cfg.Integrations.GitLab {
		i, inst := i, inst
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, err := secrets.ReadAs(inst.Token, inst.OPAccount)
			if err != nil {
				glResults[i] = glResult{inst: inst, err: err}
				return
			}
			mrs, err := gitlab.New(inst.BaseURL, token).MyMRs(ctx)
			glResults[i] = glResult{inst: inst, mrs: mrs, err: err}
		}()
	}
	wg.Wait()

	var out []any
	for _, r := range ghResults {
		s := githubSectionJSON{
			prsSectionJSON: prsSectionJSON{Source: "github", Instance: r.inst.Name},
			Items:          r.prs,
		}
		if r.err != nil {
			s.Error = r.err.Error()
		}
		if s.Items == nil {
			s.Items = []github.PR{}
		}
		out = append(out, s)
	}
	for _, r := range glResults {
		s := gitlabSectionJSON{
			prsSectionJSON: prsSectionJSON{Source: "gitlab", Instance: r.inst.Name},
			Items:          r.mrs,
		}
		if r.err != nil {
			s.Error = r.err.Error()
		}
		if s.Items == nil {
			s.Items = []gitlab.MR{}
		}
		out = append(out, s)
	}
	if out == nil {
		out = []any{}
	}
	return output.PrintJSON(out)
}

func printGitHubPRs(ctx context.Context, inst config.GitHubInstance, w *bytes.Buffer) error {
	token, err := secrets.ReadAs(inst.Token, inst.OPAccount)
	if err != nil {
		return err
	}

	prs, err := github.New(token).MyPRs(ctx)
	if err != nil {
		return err
	}

	badge := fmt.Sprintf("%d open", len(prs))
	fmt.Fprintln(w, tui.SectionHeader("GitHub / "+inst.Name, badge))

	if len(prs) == 0 {
		fmt.Fprintln(w, "  "+tui.StyleMuted.Render("no open pull requests"))
	}
	for _, pr := range prs {
		fmt.Fprintf(w, "  %s  %s  %s  %s\n",
			tui.StyleDim.Render(fmt.Sprintf("%-35s", pr.Repo)),
			prBadge(pr.Draft, ""),
			ciBadge(string(pr.CI)),
			tui.StyleNormal.Render(fmt.Sprintf("#%-4d %s", pr.Number, pr.Title)),
		)
	}
	fmt.Fprintln(w)
	return nil
}

func printGitLabMRs(ctx context.Context, inst config.GitLabInstance, w *bytes.Buffer) error {
	token, err := secrets.ReadAs(inst.Token, inst.OPAccount)
	if err != nil {
		return err
	}

	mrs, err := gitlab.New(inst.BaseURL, token).MyMRs(ctx)
	if err != nil {
		return err
	}

	badge := fmt.Sprintf("%d open", len(mrs))
	fmt.Fprintln(w, tui.SectionHeader("GitLab / "+inst.Name, badge))

	if len(mrs) == 0 {
		fmt.Fprintln(w, "  "+tui.StyleMuted.Render("no open merge requests"))
	}
	for _, mr := range mrs {
		fmt.Fprintf(w, "  %s  %s  %s  %s\n",
			tui.StyleDim.Render(fmt.Sprintf("%-35s", mr.Project)),
			prBadge(mr.Draft, mr.Status),
			ciBadge(string(mr.CI)),
			tui.StyleNormal.Render(fmt.Sprintf("!%-4d %s", mr.IID, mr.Title)),
		)
	}
	fmt.Fprintln(w)
	return nil
}

// ciBadge returns a coloured CI status badge.
func ciBadge(status string) string {
	switch status {
	case "pass":
		return tui.StyleOK.Render("✓ ci   ")
	case "fail":
		return tui.StyleError.Render("✗ ci   ")
	case "running":
		return tui.StyleWarn.Render("⟳ ci   ")
	default:
		return tui.StyleMuted.Render("— ci   ")
	}
}

// prBadge returns a coloured status badge for a PR/MR.
func prBadge(draft bool, mergeStatus string) string {
	if draft {
		return tui.StyleMuted.Render("draft    ")
	}
	switch strings.ToLower(mergeStatus) {
	case "cannot_be_merged":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("conflicts")
	case "can_be_merged":
		return tui.StyleOK.Render("ready    ")
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render("open     ")
	}
}
