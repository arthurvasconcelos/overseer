package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/arthurvasconcelos/overseer/internal/config"
	githubclient "github.com/arthurvasconcelos/overseer/internal/github"
	gitlabclient "github.com/arthurvasconcelos/overseer/internal/gitlab"
	jiraclient "github.com/arthurvasconcelos/overseer/internal/jira"
	"github.com/arthurvasconcelos/overseer/internal/learning"
	"github.com/arthurvasconcelos/overseer/internal/output"
	"github.com/arthurvasconcelos/overseer/internal/plugins/claude"
	"github.com/arthurvasconcelos/overseer/internal/plugins/google"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

const journalDateFormat = "2006-01-02"

// warningSink collects non-fatal problems from concurrent gatherers. A source
// that fails degrades its own section rather than the whole day.
type warningSink struct {
	mu       sync.Mutex
	messages []string
}

func (w *warningSink) add(message string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.messages = append(w.messages, message)
}

// remoteContext bounds one remote source. Each gets its own budget so a slow
// host cannot starve the others.
func remoteContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// JournalContext is everything known about one day, gathered from the sources
// overseer already talks to. It is the input an assistant turns into a daily
// note — this command deliberately does no writing and no prose.
type JournalContext struct {
	Date      string                 `json:"date"`
	NotePath  string                 `json:"note_path"`
	NoteFound bool                   `json:"note_exists"`
	Calendar  []JournalEvent         `json:"calendar"`
	Commits   []JournalCommit        `json:"commits"`
	MRs       []JournalMR            `json:"mrs"`
	Jira      []JournalIssue         `json:"jira"`
	Worklog   []claude.WorklogRecord `json:"worklog"`
	Learning  []JournalLearning      `json:"learning"`
	Warnings  []string               `json:"warnings"`
}

type JournalEvent struct {
	Title   string `json:"title"`
	Start   string `json:"start"`
	End     string `json:"end,omitempty"`
	AllDay  bool   `json:"all_day"`
	JoinURL string `json:"join_url,omitempty"`
}

type JournalCommit struct {
	Repo    string `json:"repo"`
	SHA     string `json:"sha"`
	Subject string `json:"subject"`
	Time    string `json:"time"`
}

type JournalMR struct {
	Project string `json:"project"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Host    string `json:"host"`
	State   string `json:"state"` // "merged" or "opened"
	Draft   bool   `json:"draft,omitempty"`
}

type JournalIssue struct {
	Key     string `json:"key"`
	Summary string `json:"summary"`
	Status  string `json:"status"`
	URL     string `json:"url"`
}

type JournalLearning struct {
	Topic       string `json:"topic"`
	Description string `json:"description"`
}

var journalCmd = &cobra.Command{
	Use:         "journal",
	Short:       "Gather a day's activity for the daily note",
	Annotations: map[string]string{"overseer/group": "Daily"},
}

var journalContextCmd = &cobra.Command{
	Use:   "context",
	Short: "Collect calendar, git, MR, Jira and session activity for a date",
	RunE:  runJournalContext,
}

var journalDate string

func init() {
	journalContextCmd.Flags().StringVar(&journalDate, "date", "", "date to gather (YYYY-MM-DD, default today)")
	journalCmd.AddCommand(journalContextCmd)
	rootCmd.AddCommand(journalCmd)
}

func runJournalContext(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	day := time.Now()
	if journalDate != "" {
		day, err = time.ParseInLocation(journalDateFormat, journalDate, time.Local)
		if err != nil {
			return fmt.Errorf("invalid --date: %w", err)
		}
	}
	date := day.Format(journalDateFormat)
	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	dayEnd := dayStart.Add(24 * time.Hour)

	journal := JournalContext{
		Date:     date,
		Calendar: []JournalEvent{},
		Commits:  []JournalCommit{},
		MRs:      []JournalMR{},
		Jira:     []JournalIssue{},
		Worklog:  []claude.WorklogRecord{},
		Learning: []JournalLearning{},
		Warnings: []string{},
	}

	journal.NotePath, journal.NoteFound = dailyNotePath(cfg, date)

	records, err := claude.ReadWorklog(date)
	if err != nil {
		journal.Warnings = append(journal.Warnings, "worklog: "+err.Error())
	} else {
		journal.Worklog = records
	}

	commits, activeRepos := gatherCommits(cfg, dayStart, dayEnd)
	journal.Commits = commits
	for _, record := range journal.Worklog {
		for _, workdir := range record.Workdirs {
			activeRepos = append(activeRepos, workdir.Root)
		}
	}

	// Remote sources run concurrently, each under its own deadline: one slow
	// host must not eat the budget of the others.
	warnings := &warningSink{}
	group := sync.WaitGroup{}

	group.Add(1)
	go func() {
		defer group.Done()
		ctx, cancel := remoteContext()
		defer cancel()
		events, issues := google.EventsOn(ctx, cfg, day)
		for _, warning := range issues {
			warnings.add(warning)
		}
		for _, event := range events {
			// All-day events carry only a date; gcal leaves End zero for them.
			entry := JournalEvent{Title: strings.TrimSpace(event.Title), AllDay: event.AllDay, JoinURL: event.JoinURL}
			if event.AllDay {
				entry.Start = event.Start.Format(journalDateFormat)
			} else {
				entry.Start = event.Start.Local().Format(time.RFC3339)
				entry.End = event.End.Local().Format(time.RFC3339)
			}
			journal.Calendar = append(journal.Calendar, entry)
		}
	}()

	group.Add(1)
	go func() {
		defer group.Done()
		ctx, cancel := remoteContext()
		defer cancel()
		journal.MRs = gatherMRs(ctx, cfg, activeRepos, dayStart, dayEnd, warnings)
	}()

	group.Add(1)
	go func() {
		defer group.Done()
		ctx, cancel := remoteContext()
		defer cancel()
		journal.Jira = gatherJira(ctx, cfg, dayStart, dayEnd, warnings)
	}()

	group.Add(1)
	go func() {
		defer group.Done()
		ctx, cancel := remoteContext()
		defer cancel()
		journal.Learning = gatherLearning(ctx, cfg, day, warnings)
	}()

	group.Wait()
	journal.Warnings = append(journal.Warnings, warnings.messages...)
	sort.Strings(journal.Warnings)

	if output.Format == "json" {
		return output.PrintJSON(journal)
	}
	printJournalContext(journal)
	return nil
}

// dailyNotePath resolves the vault path of a date's daily note.
func dailyNotePath(cfg *config.Config, date string) (string, bool) {
	if cfg.Obsidian.VaultPath == "" {
		return "", false
	}
	home, err := resolveReposPath(cfg)
	if err != nil {
		return "", false
	}
	vault := resolvePath(home, cfg.Obsidian.VaultPath)
	path := filepath.Join(vault, cfg.Obsidian.DailyNotesFolder, date+".md")
	_, statErr := os.Stat(path)
	return path, statErr == nil
}

// gatherCommits collects commits authored by the configured git identities in
// every managed repo. Commits are gathered day-wide rather than per session:
// work often lands in a repo that was never a session working directory.
func gatherCommits(cfg *config.Config, dayStart, dayEnd time.Time) ([]JournalCommit, []string) {
	emails := collectProfileEmails(cfg)
	if len(emails) == 0 {
		return []JournalCommit{}, nil
	}

	commits := []JournalCommit{}
	active := []string{}
	home, _ := resolveReposPath(cfg)

	record := func(path, name string) {
		found := commitsIn(path, name, dayStart, dayEnd, emails)
		if len(found) == 0 {
			return
		}
		commits = append(commits, found...)
		active = append(active, path)
	}

	for _, repo := range cfg.Repos {
		record(repoRoot(home, repo), repo.Name)
	}
	for _, dir := range cfg.RepoDirs {
		dir = expandHome(dir)
		// The directory itself is often a repo too (a mono-index of clones);
		// discoverRepos only walks its children.
		if isGitRepo(dir) {
			record(dir, filepath.Base(dir))
		}
		for _, path := range discoverRepos(dir) {
			name, err := filepath.Rel(dir, path)
			if err != nil {
				name = filepath.Base(path)
			}
			record(path, name)
		}
	}

	sort.Slice(commits, func(i, j int) bool { return commits[i].Time < commits[j].Time })
	return commits, active
}

func commitsIn(repoPath, repoName string, dayStart, dayEnd time.Time, emails []string) []JournalCommit {
	args := []string{
		"log",
		"--all",
		"--no-merges",
		"--since=" + dayStart.Format(time.RFC3339),
		"--until=" + dayEnd.Format(time.RFC3339),
		"--pretty=%h\x1f%cI\x1f%s",
	}
	for _, email := range emails {
		args = append(args, "--author="+email)
	}

	out, err := gitIn(repoPath, args...)
	if err != nil || strings.TrimSpace(out) == "" {
		return nil
	}

	commits := []JournalCommit{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.Split(line, "\x1f")
		if len(parts) != 3 {
			continue
		}
		commits = append(commits, JournalCommit{
			Repo:    repoName,
			SHA:     parts[0],
			Time:    parts[1],
			Subject: parts[2],
		})
	}
	return commits
}

func gatherMRs(ctx context.Context, cfg *config.Config, activeRepos []string, dayStart, dayEnd time.Time, warnings *warningSink) []JournalMR {
	mrs := []JournalMR{}

	for _, instance := range cfg.Integrations.GitLab {
		token, err := secrets.ReadAs(instance.Token, instance.OPAccount)
		if err != nil {
			warnings.add("gitlab/" + instance.Name + ": " + err.Error())
			continue
		}
		client := gitlabclient.NewWithTimeout(instance.BaseURL, token, 25*time.Second)

		projects := gitlabProjectsFor(activeRepos, instance.BaseURL)
		if len(projects) == 0 {
			// Nothing local maps to this instance — fall back to the
			// instance-wide search.
			merged, err := client.MergedBetween(ctx, dayStart, dayEnd)
			if err != nil {
				warnings.add("gitlab/" + instance.Name + ": " + err.Error())
				continue
			}
			for _, mr := range merged {
				mrs = append(mrs, JournalMR{Project: mr.Project, Title: mr.Title, URL: mr.URL, Host: instance.Name, State: "merged"})
			}
			continue
		}

		author, err := client.CurrentUsername(ctx)
		if err != nil {
			warnings.add("gitlab/" + instance.Name + ": " + err.Error())
			continue
		}
		for _, project := range projects {
			merged, err := client.MergedInProject(ctx, project, author, dayStart, dayEnd)
			if err != nil {
				warnings.add("gitlab/" + instance.Name + "/" + project + ": " + err.Error())
			}
			for _, mr := range merged {
				mrs = append(mrs, JournalMR{Project: mr.Project, Title: mr.Title, URL: mr.URL, Host: instance.Name, State: "merged"})
			}

			opened, err := client.OpenedInProject(ctx, project, author, dayStart, dayEnd)
			if err != nil {
				warnings.add("gitlab/" + instance.Name + "/" + project + ": " + err.Error())
				continue
			}
			for _, mr := range opened {
				mrs = append(mrs, JournalMR{Project: mr.Project, Title: mr.Title, URL: mr.URL, Host: instance.Name, State: "opened", Draft: mr.Draft})
			}
		}
	}

	for _, instance := range cfg.Integrations.GitHub {
		token, err := secrets.ReadAs(instance.Token, instance.OPAccount)
		if err != nil {
			warnings.add("github/" + instance.Name + ": " + err.Error())
			continue
		}
		merged, err := githubclient.New(token).MergedPRs(ctx, dayStart)
		if err != nil {
			warnings.add("github/" + instance.Name + ": " + err.Error())
			continue
		}
		for _, pr := range merged {
			mrs = append(mrs, JournalMR{Project: pr.Repo, Title: pr.Title, URL: pr.URL, Host: instance.Name, State: "merged"})
		}
	}

	return mrs
}

func gatherJira(ctx context.Context, cfg *config.Config, dayStart, dayEnd time.Time, warnings *warningSink) []JournalIssue {
	issues := []JournalIssue{}

	for _, instance := range cfg.Integrations.Jira {
		email, err := secrets.ReadAs(instance.Email, instance.OPAccount)
		if err != nil {
			warnings.add("jira/" + instance.Name + ": " + err.Error())
			continue
		}
		token, err := secrets.ReadAs(instance.Token, instance.OPAccount)
		if err != nil {
			warnings.add("jira/" + instance.Name + ": " + err.Error())
			continue
		}
		found, err := jiraclient.New(instance.BaseURL, email, token).UpdatedBetween(ctx, dayStart, dayEnd)
		if err != nil {
			warnings.add("jira/" + instance.Name + ": " + err.Error())
			continue
		}
		for _, issue := range found {
			issues = append(issues, JournalIssue{
				Key:     issue.Key,
				Summary: issue.Summary,
				Status:  issue.Status,
				URL:     strings.TrimSuffix(instance.BaseURL, "/") + "/browse/" + issue.Key,
			})
		}
	}

	return issues
}

func gatherLearning(ctx context.Context, cfg *config.Config, day time.Time, warnings *warningSink) []JournalLearning {
	entries := []JournalLearning{}

	dbPath := learning.DBPath(config.BrainOverseerPath(cfg))
	if _, err := os.Stat(dbPath); err != nil {
		return entries
	}

	service, err := learning.Open(ctx, dbPath)
	if err != nil {
		warnings.add("learning: " + err.Error())
		return entries
	}
	defer service.Close()

	added, err := service.AddedOn(ctx, day)
	if err != nil {
		warnings.add("learning: " + err.Error())
		return entries
	}
	for _, entry := range added {
		entries = append(entries, JournalLearning{Topic: entry.Topic, Description: entry.Description})
	}
	return entries
}

func printJournalContext(journal JournalContext) {
	fmt.Println(tui.SectionHeader("journal context", journal.Date))
	fmt.Println()

	note := journal.NotePath
	if note == "" {
		note = "(no vault configured)"
	} else if !journal.NoteFound {
		note += "  (not created yet)"
	}
	fmt.Printf("  %s  %s\n\n", tui.StyleDim.Render("note  "), tui.StyleNormal.Render(note))

	printJournalSection("calendar", len(journal.Calendar), func() {
		for _, event := range journal.Calendar {
			when := "all day"
			if !event.AllDay {
				when = journalClock(event.Start) + "-" + journalClock(event.End)
			}
			fmt.Printf("      %s  %s\n", tui.StyleAccent.Render(when), tui.StyleNormal.Render(event.Title))
		}
	})

	printJournalSection("commits", len(journal.Commits), func() {
		for _, commit := range journal.Commits {
			fmt.Printf("      %s  %s  %s\n",
				tui.StyleAccent.Render(journalClock(commit.Time)),
				tui.StyleDim.Render(commit.Repo),
				tui.StyleNormal.Render(commit.Subject))
		}
	})

	printJournalSection("mrs", len(journal.MRs), func() {
		for _, mr := range journal.MRs {
			fmt.Printf("      %s  %s  %s\n",
				tui.StyleAccent.Render(mr.State),
				tui.StyleDim.Render(mr.Project),
				tui.StyleNormal.Render(mr.Title))
		}
	})

	printJournalSection("jira", len(journal.Jira), func() {
		for _, issue := range journal.Jira {
			fmt.Printf("      %s  %s  %s\n",
				tui.StyleAccent.Render(issue.Key),
				tui.StyleNormal.Render(issue.Summary),
				tui.StyleDim.Render("("+issue.Status+")"))
		}
	})

	printJournalSection("sessions", len(journal.Worklog), func() {
		for _, record := range journal.Worklog {
			roots := []string{}
			for _, workdir := range record.Workdirs {
				roots = append(roots, filepath.Base(workdir.Root))
			}
			fmt.Printf("      %s  %s  %s\n",
				tui.StyleAccent.Render(journalClock(record.StartedAt)+"-"+journalClock(record.EndedAt)),
				tui.StyleNormal.Render(strings.Join(roots, ", ")),
				tui.StyleDim.Render(fmt.Sprintf("%d prompts", len(record.Prompts))))
		}
	})

	printJournalSection("learning", len(journal.Learning), func() {
		for _, entry := range journal.Learning {
			fmt.Printf("      %s\n", tui.StyleNormal.Render(entry.Topic))
		}
	})

	for _, warning := range journal.Warnings {
		fmt.Println("  " + tui.WarnLine("skip", warning))
	}
}

func printJournalSection(label string, count int, body func()) {
	if count == 0 {
		fmt.Printf("  %s  %s\n", tui.StyleDim.Render(label), tui.StyleMuted.Render("(none)"))
		return
	}
	fmt.Printf("  %s  %s\n", tui.StyleDim.Render(label), tui.StyleMuted.Render(fmt.Sprintf("(%d)", count)))
	body()
}

func journalClock(timestamp string) string {
	parsed, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return "--:--"
	}
	return parsed.Local().Format("15:04")
}

// gitlabProjectsFor returns the GitLab project paths of the repos that saw
// activity today and whose origin points at the given instance. Querying those
// directly avoids the instance-wide merged-MR search, which gitlab.com answers
// with 408 for accounts with many projects — and scoping to active repos keeps
// the fan-out at a handful of requests rather than one per managed repo.
func gitlabProjectsFor(repoPaths []string, baseURL string) []string {
	host := gitlabHost(baseURL)
	if host == "" {
		return nil
	}

	seen := map[string]bool{}
	projects := []string{}
	for _, path := range repoPaths {
		remote, err := gitIn(path, "remote", "get-url", "origin")
		if err != nil {
			continue
		}
		project := gitlabProjectFromRemote(strings.TrimSpace(remote), host)
		if project == "" || seen[project] {
			continue
		}
		seen[project] = true
		projects = append(projects, project)
	}
	sort.Strings(projects)
	return projects
}

func gitlabHost(baseURL string) string {
	if baseURL == "" {
		return "gitlab.com"
	}
	trimmed := strings.TrimSuffix(baseURL, "/")
	trimmed = strings.TrimPrefix(trimmed, "https://")
	trimmed = strings.TrimPrefix(trimmed, "http://")
	return trimmed
}

// gitlabProjectFromRemote extracts "group/subgroup/project" from an origin URL
// on the given host, in either SSH or HTTPS form.
func gitlabProjectFromRemote(remote, host string) string {
	project := ""
	switch {
	case strings.HasPrefix(remote, "git@"+host+":"):
		project = strings.TrimPrefix(remote, "git@"+host+":")
	case strings.HasPrefix(remote, "ssh://git@"+host+"/"):
		project = strings.TrimPrefix(remote, "ssh://git@"+host+"/")
	case strings.HasPrefix(remote, "https://"+host+"/"):
		project = strings.TrimPrefix(remote, "https://"+host+"/")
	case strings.HasPrefix(remote, "http://"+host+"/"):
		project = strings.TrimPrefix(remote, "http://"+host+"/")
	default:
		return ""
	}
	return strings.TrimSuffix(strings.Trim(project, "/"), ".git")
}
