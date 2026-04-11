package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/arthurvasconcelos/overseer/internal/config"
	jiraclient "github.com/arthurvasconcelos/overseer/internal/jira"
	"github.com/arthurvasconcelos/overseer/internal/notify"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	slackclient "github.com/arthurvasconcelos/overseer/internal/slack"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

var standupCmd = &cobra.Command{
	Use:   "standup",
	Short: "Synthesize yesterday's activity into a standup message",
	RunE:  runStandup,
}

var (
	standupPostWorkspace string
	standupPostChannel   string
	standupCopy          bool
)

func init() {
	standupCmd.Flags().StringVar(&standupPostWorkspace, "post-workspace", "", "Slack workspace name to post standup to")
	standupCmd.Flags().StringVar(&standupPostChannel, "post-channel", "", "Slack channel to post standup to (requires --post-workspace)")
	standupCmd.Flags().BoolVar(&standupCopy, "copy", false, "Copy output to clipboard (macOS)")
	rootCmd.AddCommand(standupCmd)
}

func runStandup(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	yesterday := time.Now().AddDate(0, 0, -1)
	// Start of yesterday.
	since := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())

	var sb strings.Builder

	sb.WriteString(tui.StyleHeader.Render("Standup") + "  " + tui.StyleMuted.Render(time.Now().Format("Monday, 02 Jan 2006")))
	sb.WriteString("\n\n")

	// --- What I did ---
	sb.WriteString(tui.StyleAccent.Render("What I did") + "\n")

	// Jira: issues moved to Done or In Review since yesterday.
	doneStatuses := []string{"Done", "In Review", "Code Review", "Resolved", "Closed"}
	anyJira := false
	for _, inst := range cfg.Integrations.Jira {
		email, err := secrets.ReadAs(inst.Email, inst.OPAccount)
		if err != nil {
			sb.WriteString(tui.WarnLine("jira/"+inst.Name, err.Error()) + "\n")
			continue
		}
		token, err := secrets.ReadAs(inst.Token, inst.OPAccount)
		if err != nil {
			sb.WriteString(tui.WarnLine("jira/"+inst.Name, err.Error()) + "\n")
			continue
		}
		issues, err := jiraclient.New(inst.BaseURL, email, token).RecentlyUpdated(ctx, doneStatuses, since)
		if err != nil {
			sb.WriteString(tui.WarnLine("jira/"+inst.Name, err.Error()) + "\n")
			continue
		}
		for _, i := range issues {
			sb.WriteString(fmt.Sprintf("  • [%s] %s (%s)\n", i.Key, i.Summary, i.Status))
			anyJira = true
		}
	}

	// Git: commits across managed repos since yesterday authored by configured profiles.
	profileEmails := collectProfileEmails(cfg)
	home := resolveReposPath(cfg)
	anyCommit := false
	for _, repo := range cfg.Repos {
		path := repoRoot(home, repo)
		commits := gitCommitsSince(path, since, profileEmails)
		for _, c := range commits {
			sb.WriteString(fmt.Sprintf("  • [%s] %s\n", repo.Name, c))
			anyCommit = true
		}
	}

	if !anyJira && !anyCommit {
		sb.WriteString(tui.StyleMuted.Render("  (nothing found)") + "\n")
	}
	sb.WriteString("\n")

	// --- What I'm doing ---
	sb.WriteString(tui.StyleAccent.Render("What I'm doing") + "\n")
	sb.WriteString(tui.StyleMuted.Render("  (fill in manually)") + "\n\n")

	// --- Blockers ---
	sb.WriteString(tui.StyleAccent.Render("Blockers") + "\n")
	sb.WriteString(tui.StyleMuted.Render("  (none)") + "\n")

	output := sb.String()
	fmt.Print(output)

	if standupCopy {
		if err := copyToClipboard(stripANSI(output)); err != nil {
			fmt.Println(tui.WarnLine("copy", err.Error()))
		}
	}

	if standupPostWorkspace != "" && standupPostChannel != "" {
		if err := postStandupToSlack(cfg, standupPostWorkspace, standupPostChannel, stripANSI(output)); err != nil {
			fmt.Println(tui.WarnLine("slack", err.Error()))
		} else {
			fmt.Println(tui.StyleOK.Render("posted to #" + standupPostChannel))
		}
	}

	if cfg.System.Notifications {
		_ = notify.Send("overseer standup", "Standup ready", "")
	}

	return nil
}

// collectProfileEmails returns all email addresses from configured git profiles.
func collectProfileEmails(cfg *config.Config) []string {
	seen := map[string]bool{}
	var emails []string
	for _, p := range cfg.Git.Profiles {
		if p.Email != "" && !seen[p.Email] {
			seen[p.Email] = true
			emails = append(emails, p.Email)
		}
	}
	return emails
}

// gitCommitsSince returns one-line commit summaries in a repo since the given
// time, filtered to commits authored by any of the given emails.
func gitCommitsSince(repoPath string, since time.Time, emails []string) []string {
	if len(emails) == 0 {
		return nil
	}
	sinceStr := since.Format("2006-01-02")
	args := []string{
		"log",
		"--oneline",
		"--since=" + sinceStr,
		"--all",
	}
	for _, e := range emails {
		args = append(args, "--author="+e)
	}
	out, err := gitIn(repoPath, args...)
	if err != nil || strings.TrimSpace(out) == "" {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var commits []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			commits = append(commits, l)
		}
	}
	return commits
}

func postStandupToSlack(cfg *config.Config, workspaceName, channel, text string) error {
	for _, ws := range cfg.Integrations.Slack {
		if ws.Name != workspaceName {
			continue
		}
		token, err := secrets.ReadAs(ws.Token, ws.OPAccount)
		if err != nil {
			return err
		}
		return slackclient.New(token).Send(channel, text)
	}
	return fmt.Errorf("slack workspace %q not found in config", workspaceName)
}

// stripANSI removes ANSI escape sequences from a string (for plain-text copy/post).
func stripANSI(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++ // skip 'm'
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

