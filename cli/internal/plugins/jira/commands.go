package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	claudeaiclient "github.com/arthurvasconcelos/overseer/internal/claudeai"
	"github.com/arthurvasconcelos/overseer/internal/config"
	jiraclient "github.com/arthurvasconcelos/overseer/internal/jira"
	"github.com/arthurvasconcelos/overseer/internal/output"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var instanceFlag string

func commands(cfg *config.Config) []*cobra.Command {
	root := &cobra.Command{
		Use:         "jira",
		Short:       "Jira issue interactions — get, create, update, transition, worklog, breakdown",
		Annotations: map[string]string{"overseer/group": "Dev"},
	}
	root.PersistentFlags().StringVar(&instanceFlag, "instance", "", "Jira instance name (auto-selects if only one configured)")
	root.AddCommand(getCmd())
	root.AddCommand(createCmd())
	root.AddCommand(updateCmd())
	root.AddCommand(transitionCmd())
	root.AddCommand(syncCmd())
	root.AddCommand(worklogCmd())
	root.AddCommand(breakdownCmd())
	return []*cobra.Command{root}
}

func resolveInstance(cfg *config.Config, name string) (config.JiraInstance, error) {
	if len(cfg.Integrations.Jira) == 0 {
		return config.JiraInstance{}, fmt.Errorf("no Jira instances configured")
	}
	if name != "" {
		for _, inst := range cfg.Integrations.Jira {
			if inst.Name == name {
				return inst, nil
			}
		}
		return config.JiraInstance{}, fmt.Errorf("Jira instance %q not found", name)
	}
	if len(cfg.Integrations.Jira) == 1 {
		return cfg.Integrations.Jira[0], nil
	}
	items := make([]tui.SelectItem, len(cfg.Integrations.Jira))
	for i, inst := range cfg.Integrations.Jira {
		items[i] = tui.SelectItem{Title: inst.Name, Subtitle: tui.StyleMuted.Render(inst.BaseURL)}
	}
	idx, err := tui.Select("Select Jira instance", items)
	if err != nil {
		return config.JiraInstance{}, err
	}
	if idx < 0 {
		return config.JiraInstance{}, fmt.Errorf("no instance selected")
	}
	return cfg.Integrations.Jira[idx], nil
}

func buildClient(inst config.JiraInstance) (*jiraclient.Client, error) {
	email, err := secrets.ReadAs(inst.Email, inst.OPAccount)
	if err != nil {
		return nil, fmt.Errorf("resolving email: %w", err)
	}
	token, err := secrets.ReadAs(inst.Token, inst.OPAccount)
	if err != nil {
		return nil, fmt.Errorf("resolving token: %w", err)
	}
	return jiraclient.New(inst.BaseURL, email, token), nil
}

// ParseWorkDuration parses "1h30m", "45m", or a plain number (minutes) into seconds.
// Exported so focus.go can call it without duplicating the logic.
func ParseWorkDuration(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	total := 0
	if idx := strings.Index(s, "h"); idx >= 0 {
		h, err := strconv.Atoi(strings.TrimSpace(s[:idx]))
		if err != nil {
			return 0, fmt.Errorf("invalid hours in %q", s)
		}
		total += h * 3600
		s = strings.TrimSpace(s[idx+1:])
	}
	if idx := strings.Index(s, "m"); idx >= 0 {
		m, err := strconv.Atoi(strings.TrimSpace(s[:idx]))
		if err != nil {
			return 0, fmt.Errorf("invalid minutes in %q", s)
		}
		total += m * 60
		s = strings.TrimSpace(s[idx+1:])
	} else if s != "" {
		m, err := strconv.Atoi(s)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q — use formats like 1h30m, 45m, 90", s)
		}
		total += m * 60
	}
	if total <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}
	return total, nil
}

func apiCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 15*time.Second)
}

func statusCell(status string, width int) string {
	padded := fmt.Sprintf("%-*s", width, status)
	switch strings.ToLower(status) {
	case "in progress":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(padded)
	case "in review", "code review":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render(padded)
	case "done", "closed", "resolved":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82")).Render(padded)
	default:
		return tui.StyleDim.Render(padded)
	}
}

func priorityCell(priority string, width int) string {
	padded := fmt.Sprintf("%-*s", width, priority)
	switch strings.ToLower(priority) {
	case "critical", "highest":
		return tui.StyleError.Render(padded)
	case "high":
		return tui.StyleWarn.Render(padded)
	case "medium":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("228")).Render(padded)
	default:
		return tui.StyleMuted.Render(padded)
	}
}

func statusBadge(status string) string   { return statusCell(status, len(status)) }
func priorityBadge(priority string) string { return priorityCell(priority, len(priority)) }

func getCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <KEY>",
		Short: "Show full details of a Jira issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
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
			ctx, cancel := apiCtx()
			defer cancel()
			issue, err := client.GetIssue(ctx, args[0])
			if err != nil {
				return err
			}
			if output.Format == "json" {
				return output.PrintJSON(issue)
			}
			printFullIssue(issue, inst.BaseURL)
			return nil
		},
	}
}

func printFullIssue(issue *jiraclient.FullIssue, baseURL string) {
	issueURL := baseURL + "/browse/" + issue.Key
	fmt.Printf("\n  %s  %s\n\n", tui.StyleAccent.Render(tui.Hyperlink(issueURL, issue.Key)), tui.StyleNormal.Render(issue.Summary))

	row := func(label, value string) {
		if value == "" {
			value = tui.StyleMuted.Render("—")
		}
		fmt.Printf("  %-14s%s\n", tui.StyleDim.Render(label), value)
	}

	row("Status", statusBadge(issue.Status))
	row("Priority", priorityBadge(issue.Priority))
	row("Type", tui.StyleNormal.Render(issue.IssueType))
	row("Project", tui.StyleNormal.Render(issue.ProjectKey+" · "+issue.ProjectName))
	row("Assignee", tui.StyleNormal.Render(issue.Assignee))
	row("Reporter", tui.StyleNormal.Render(issue.Reporter))
	if issue.Parent != "" {
		row("Parent", tui.StyleAccent.Render(issue.Parent))
	}
	if len(issue.Labels) > 0 {
		row("Labels", tui.StyleNormal.Render(strings.Join(issue.Labels, ", ")))
	}

	if desc := strings.TrimSpace(issue.Description); desc != "" {
		fmt.Println()
		fmt.Println("  " + tui.StyleDim.Render("Description"))
		for _, line := range strings.Split(desc, "\n") {
			fmt.Println("  " + tui.StyleNormal.Render(line))
		}
	}
	fmt.Println()
}

func createCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create a new Jira issue interactively",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
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

			ctx, cancel := apiCtx()
			defer cancel()
			stop := tui.StartSpinner("Fetching projects…")
			projects, err := client.GetProjects(ctx)
			stop()
			if err != nil {
				return fmt.Errorf("fetching projects: %w", err)
			}
			if len(projects) == 0 {
				return fmt.Errorf("no projects found")
			}

			projItems := make([]tui.SelectItem, len(projects))
			for i, p := range projects {
				projItems[i] = tui.SelectItem{Title: p.Key, Subtitle: tui.StyleMuted.Render(p.Name)}
			}
			pidx, err := tui.Select("Select project", projItems)
			if err != nil || pidx < 0 {
				return nil
			}
			project := projects[pidx]

			ctx2, cancel2 := apiCtx()
			defer cancel2()
			stop = tui.StartSpinner("Fetching issue types…")
			issueTypes, err := client.GetIssueTypes(ctx2, project.Key)
			stop()
			if err != nil {
				return fmt.Errorf("fetching issue types: %w", err)
			}
			var topLevel []jiraclient.IssueType
			for _, it := range issueTypes {
				if !it.Subtask {
					topLevel = append(topLevel, it)
				}
			}
			if len(topLevel) == 0 {
				return fmt.Errorf("no issue types available for project %s", project.Key)
			}

			itItems := make([]tui.SelectItem, len(topLevel))
			for i, it := range topLevel {
				itItems[i] = tui.SelectItem{Title: it.Name}
			}
			itidx, err := tui.Select("Issue type", itItems)
			if err != nil || itidx < 0 {
				return nil
			}
			issueType := topLevel[itidx]

			summary, err := tui.Prompt("Summary", "", "Brief description of the issue")
			if err != nil || summary == "" {
				return nil
			}

			priorities := []string{"Highest", "High", "Medium", "Low", "Lowest"}
			priItems := make([]tui.SelectItem, len(priorities))
			for i, p := range priorities {
				priItems[i] = tui.SelectItem{Title: p}
			}
			priidx, err := tui.Select("Priority", priItems)
			if err != nil || priidx < 0 {
				return nil
			}

			description, err := tui.Prompt("Description (optional)", "", "")
			if err != nil {
				return nil
			}

			ctx3, cancel3 := apiCtx()
			defer cancel3()
			stop = tui.StartSpinner("Creating issue…")
			key, err := client.CreateIssueSimple(ctx3, project.Key, issueType.Name, summary, priorities[priidx], description)
			stop()
			if err != nil {
				return err
			}

			issueURL := inst.BaseURL + "/browse/" + key
			fmt.Printf("\n  %s  %s\n\n", tui.StyleOK.Render("✓ created"), tui.Hyperlink(issueURL, key))
			return nil
		},
	}
}

func updateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <KEY>",
		Short: "Update summary, priority, or description of a Jira issue interactively",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
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

			ctx, cancel := apiCtx()
			defer cancel()
			issue, err := client.GetIssue(ctx, args[0])
			if err != nil {
				return err
			}

			fmt.Printf("\n  %s  %s\n\n", tui.StyleAccent.Render(issue.Key), tui.StyleNormal.Render(issue.Summary))

			newSummary, err := tui.Prompt("Summary", issue.Summary, "")
			if err != nil {
				return nil
			}

			priorities := []string{"Highest", "High", "Medium", "Low", "Lowest"}
			currentPriIdx := 2
			for i, p := range priorities {
				if strings.EqualFold(p, issue.Priority) {
					currentPriIdx = i
					break
				}
			}
			priItems := make([]tui.SelectItem, len(priorities))
			for i, p := range priorities {
				label := p
				if i == currentPriIdx {
					label += "  " + tui.StyleMuted.Render("← current")
				}
				priItems[i] = tui.SelectItem{Title: label}
			}
			priidx, err := tui.Select("Priority", priItems)
			if err != nil || priidx < 0 {
				return nil
			}

			newDesc, err := tui.Prompt("Description (leave blank to keep current)", "", "")
			if err != nil {
				return nil
			}

			fields := map[string]any{}
			if newSummary != "" && newSummary != issue.Summary {
				fields["summary"] = newSummary
			}
			if priorities[priidx] != issue.Priority {
				fields["priority"] = map[string]any{"name": priorities[priidx]}
			}
			if newDesc != "" {
				fields["description"] = jiraclient.TextToADF(newDesc)
			}

			if len(fields) == 0 {
				fmt.Println("  " + tui.StyleMuted.Render("no changes"))
				return nil
			}

			ctx2, cancel2 := apiCtx()
			defer cancel2()
			if err := client.UpdateIssue(ctx2, issue.Key, fields); err != nil {
				return err
			}

			fmt.Printf("\n  %s  %s updated\n\n", tui.StyleOK.Render("✓"), tui.StyleAccent.Render(issue.Key))
			return nil
		},
	}
}

func transitionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "transition <KEY>",
		Short: "Transition a Jira issue to a new status",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
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

			ctx, cancel := apiCtx()
			defer cancel()
			issue, err := client.GetIssue(ctx, args[0])
			if err != nil {
				return err
			}

			fmt.Printf("\n  %s  %s\n  %s  %s\n\n",
				tui.StyleAccent.Render(issue.Key), tui.StyleNormal.Render(issue.Summary),
				tui.StyleDim.Render("Current:       "), statusBadge(issue.Status))

			ctx2, cancel2 := apiCtx()
			defer cancel2()
			transitions, err := client.GetTransitions(ctx2, args[0])
			if err != nil {
				return err
			}
			if len(transitions) == 0 {
				fmt.Println("  " + tui.StyleMuted.Render("no transitions available"))
				return nil
			}

			items := make([]tui.SelectItem, len(transitions))
			for i, t := range transitions {
				items[i] = tui.SelectItem{Title: t.Name}
			}
			idx, err := tui.Select("Transition to", items)
			if err != nil || idx < 0 {
				return nil
			}

			ctx3, cancel3 := apiCtx()
			defer cancel3()
			if err := client.TransitionIssue(ctx3, args[0], transitions[idx].ID); err != nil {
				return err
			}

			fmt.Printf("  %s  %s → %s\n\n",
				tui.StyleOK.Render("✓"),
				tui.StyleDim.Render(issue.Status),
				statusBadge(transitions[idx].Name))
			return nil
		},
	}
}

func syncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "List all your open Jira issues across configured instances",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if len(cfg.Integrations.Jira) == 0 {
				fmt.Println(tui.StyleMuted.Render("no Jira instances configured"))
				return nil
			}

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			type result struct {
				Instance string             `json:"instance"`
				Issues   []jiraclient.Issue `json:"issues"`
				Error    string             `json:"error,omitempty"`
			}

			results := make([]result, len(cfg.Integrations.Jira))
			var wg sync.WaitGroup
			for i, inst := range cfg.Integrations.Jira {
				i, inst := i, inst
				wg.Add(1)
				go func() {
					defer wg.Done()
					client, err := buildClient(inst)
					if err != nil {
						results[i] = result{Instance: inst.Name, Error: err.Error()}
						return
					}
					issues, err := client.MyIssues(ctx)
					if err != nil {
						results[i] = result{Instance: inst.Name, Error: err.Error()}
						return
					}
					results[i] = result{Instance: inst.Name, Issues: issues}
				}()
			}
			wg.Wait()

			if output.Format == "json" {
				return output.PrintJSON(results)
			}

			for _, r := range results {
				badge := fmt.Sprintf("%d open", len(r.Issues))
				fmt.Println(tui.SectionHeader("Jira / "+r.Instance, badge))
				if r.Error != "" {
					fmt.Println("  " + tui.WarnLine("error", r.Error))
				} else if len(r.Issues) == 0 {
					fmt.Println("  " + tui.StyleMuted.Render("no open issues"))
				}
				for _, iss := range r.Issues {
					key := tui.StyleAccent.Render(fmt.Sprintf("%-12s", iss.Key))
					status := statusCell(iss.Status, 14)
					priority := priorityCell(iss.Priority, 10)
					fmt.Printf("  %s  %s  %s  %s\n", key, status, priority, tui.StyleNormal.Render(iss.Summary))
				}
				fmt.Println()
			}
			return nil
		},
	}
}

func worklogCmd() *cobra.Command {
	var comment string
	cmd := &cobra.Command{
		Use:   "worklog <KEY> [duration]",
		Short: "Log time spent on a Jira issue (e.g. 1h30m, 45m, 90)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
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

			key := args[0]
			durStr := ""
			if len(args) >= 2 {
				durStr = args[1]
			} else {
				durStr, err = tui.Prompt("Duration", "", "e.g. 1h30m, 45m, 90")
				if err != nil || durStr == "" {
					return nil
				}
			}

			seconds, err := ParseWorkDuration(durStr)
			if err != nil {
				return err
			}

			if comment == "" {
				comment, err = tui.Prompt("Comment (optional)", "", "")
				if err != nil {
					return nil
				}
			}

			ctx, cancel := apiCtx()
			defer cancel()
			if err := client.AddWorklog(ctx, key, seconds, comment); err != nil {
				return err
			}

			h := seconds / 3600
			m := (seconds % 3600) / 60
			var durDisplay string
			switch {
			case h > 0 && m > 0:
				durDisplay = fmt.Sprintf("%dh%dm", h, m)
			case h > 0:
				durDisplay = fmt.Sprintf("%dh", h)
			default:
				durDisplay = fmt.Sprintf("%dm", m)
			}

			fmt.Printf("\n  %s  logged %s to %s\n\n", tui.StyleOK.Render("✓"), durDisplay, tui.StyleAccent.Render(key))
			return nil
		},
	}
	cmd.Flags().StringVar(&comment, "comment", "", "Worklog comment")
	return cmd
}

func breakdownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "breakdown <KEY>",
		Short: "Use Claude AI to suggest and create subtasks for an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if cfg.Integrations.Claude == nil || cfg.Integrations.Claude.APIKey == "" {
				return fmt.Errorf("integrations.claude.api_key is not configured")
			}
			inst, err := resolveInstance(cfg, instanceFlag)
			if err != nil {
				return err
			}
			client, err := buildClient(inst)
			if err != nil {
				return err
			}

			ctx, cancel := apiCtx()
			defer cancel()
			stop := tui.StartSpinner("Fetching issue…")
			issue, err := client.GetIssue(ctx, args[0])
			stop()
			if err != nil {
				return err
			}

			fmt.Printf("\n  %s  %s\n\n", tui.StyleAccent.Render(issue.Key), tui.StyleNormal.Render(issue.Summary))

			apiKey, err := secrets.Read(cfg.Integrations.Claude.APIKey)
			if err != nil {
				return fmt.Errorf("resolving claude api_key: %w", err)
			}

			claudeClient := claudeaiclient.New(apiKey)
			ctx2, cancel2 := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel2()

			systemPrompt := `You are a software project assistant. Given a Jira issue, suggest a breakdown into implementation subtasks.
Return ONLY a valid JSON array of strings, each being a concise subtask title starting with an action verb.
Maximum 8 subtasks. No explanation, no markdown fences — just the raw JSON array.`
			question := fmt.Sprintf("Break down this issue into subtasks:\n\nTitle: %s\n\nDescription: %s",
				issue.Summary, issue.Description)

			stop = tui.StartSpinner("Generating breakdown with Claude…")
			response, err := claudeClient.Ask(ctx2, systemPrompt, question, 512)
			stop()
			if err != nil {
				return fmt.Errorf("claude: %w", err)
			}

			start := strings.Index(response, "[")
			end := strings.LastIndex(response, "]")
			if start < 0 || end < 0 || end <= start {
				return fmt.Errorf("unexpected Claude response format: %s", response)
			}
			var subtasks []string
			if err := json.Unmarshal([]byte(response[start:end+1]), &subtasks); err != nil {
				return fmt.Errorf("parsing Claude response: %w", err)
			}
			if len(subtasks) == 0 {
				fmt.Println(tui.StyleMuted.Render("  no subtasks suggested"))
				return nil
			}

			fmt.Println(tui.SectionHeader("Proposed subtasks", fmt.Sprintf("%d", len(subtasks))))
			fmt.Println()
			for i, t := range subtasks {
				fmt.Printf("  %s  %s\n", tui.StyleAccent.Render(fmt.Sprintf("%2d.", i+1)), tui.StyleNormal.Render(t))
			}
			fmt.Println()

			ok, err := tui.Confirm(fmt.Sprintf("Create %d subtasks under %s?", len(subtasks), issue.Key))
			if err != nil || !ok {
				fmt.Println(tui.StyleMuted.Render("  cancelled"))
				return nil
			}

			ctx3, cancel3 := apiCtx()
			defer cancel3()
			issueTypes, err := client.GetIssueTypes(ctx3, issue.ProjectKey)
			if err != nil {
				return fmt.Errorf("fetching issue types: %w", err)
			}
			subtaskTypeName := "Subtask"
			for _, it := range issueTypes {
				if it.Subtask {
					subtaskTypeName = it.Name
					break
				}
			}

			fmt.Println()
			for i, title := range subtasks {
				ctx4, cancel4 := apiCtx()
				stop = tui.StartSpinner(fmt.Sprintf("Creating %d/%d…", i+1, len(subtasks)))
				key, err := client.CreateSubtask(ctx4, issue.ProjectKey, issue.Key, subtaskTypeName, title)
				cancel4()
				stop()
				if err != nil {
					fmt.Printf("  %s  %s\n", tui.StyleError.Render("✗"), tui.StyleNormal.Render(title))
					fmt.Printf("       %s\n", tui.StyleMuted.Render(err.Error()))
				} else {
					issueURL := inst.BaseURL + "/browse/" + key
					fmt.Printf("  %s  %s  %s\n", tui.StyleOK.Render("✓"), tui.Hyperlink(issueURL, key), tui.StyleNormal.Render(title))
				}
			}
			fmt.Println()
			return nil
		},
	}
}
