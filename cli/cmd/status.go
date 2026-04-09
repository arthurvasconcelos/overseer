package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/gcal"
	"github.com/arthurvasconcelos/overseer/internal/jira"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	overseerslack "github.com/arthurvasconcelos/overseer/internal/slack"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Health-check all configured integrations",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

type checkResult struct {
	name string
	ok   bool
	msg  string
}

type checkResultJSON struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

func runStatus(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Build the list of checks to run in parallel.
	type checkFn struct {
		name string
		fn   func() checkResult
	}

	var checks []checkFn

	checks = append(checks, checkFn{"1password", checkOnePassword})

	for _, j := range cfg.Integrations.Jira {
		j := j
		checks = append(checks, checkFn{
			name: "jira/" + j.Name,
			fn:   func() checkResult { return checkJira(ctx, j) },
		})
	}

	for _, s := range cfg.Integrations.Slack {
		s := s
		checks = append(checks, checkFn{
			name: "slack/" + s.Name,
			fn:   func() checkResult { return checkSlack(s) },
		})
	}

	for _, g := range cfg.Integrations.Google {
		g := g
		checks = append(checks, checkFn{
			name: "google/" + g.Name,
			fn:   func() checkResult { return checkGoogle(g) },
		})
	}

	results := make([]checkResult, len(checks))
	var wg sync.WaitGroup
	for i, c := range checks {
		i, c := i, c
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = c.fn()
			results[i].name = c.name
		}()
	}
	wg.Wait()

	if outputFormat == "json" {
		out := make([]checkResultJSON, len(results))
		for i, r := range results {
			out[i] = checkResultJSON{Name: r.name, OK: r.ok, Message: r.msg}
		}
		return printJSON(out)
	}

	// Calculate column width for alignment.
	maxLen := 0
	for _, r := range results {
		if len(r.name) > maxLen {
			maxLen = len(r.name)
		}
	}

	for _, r := range results {
		icon := tui.StyleOK.Render("✓")
		if !r.ok {
			icon = tui.StyleError.Render("✗")
		}
		padding := strings.Repeat(" ", maxLen-len(r.name)+2)
		fmt.Printf("  %s%s%s  %s\n",
			tui.StyleNormal.Render(r.name),
			padding,
			icon,
			tui.StyleDim.Render(r.msg),
		)
	}
	return nil
}

func checkOnePassword() checkResult {
	out, err := exec.Command("op", "account", "list", "--format=json").Output()
	if err != nil {
		return checkResult{ok: false, msg: "op CLI not available or not signed in"}
	}
	var accounts []struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(out, &accounts); err != nil || len(accounts) == 0 {
		return checkResult{ok: false, msg: "no accounts found"}
	}
	noun := "account"
	if len(accounts) > 1 {
		noun = "accounts"
	}
	return checkResult{ok: true, msg: fmt.Sprintf("signed in (%d %s)", len(accounts), noun)}
}

func checkJira(ctx context.Context, inst config.JiraInstance) checkResult {
	email, err := secrets.ReadAs(inst.Email, inst.OPAccount)
	if err != nil {
		return checkResult{ok: false, msg: "could not read email: " + err.Error()}
	}
	token, err := secrets.ReadAs(inst.Token, inst.OPAccount)
	if err != nil {
		return checkResult{ok: false, msg: "could not read token: " + err.Error()}
	}
	authedEmail, err := jira.New(inst.BaseURL, email, token).Ping(ctx)
	if err != nil {
		return checkResult{ok: false, msg: err.Error()}
	}
	return checkResult{ok: true, msg: authedEmail}
}

func checkSlack(ws config.SlackWorkspace) checkResult {
	token, err := secrets.ReadAs(ws.Token, ws.OPAccount)
	if err != nil {
		return checkResult{ok: false, msg: "could not read token: " + err.Error()}
	}
	username, err := overseerslack.New(token).Ping()
	if err != nil {
		return checkResult{ok: false, msg: err.Error()}
	}
	return checkResult{ok: true, msg: "@" + username}
}

func checkGoogle(acc config.GoogleAccount) checkResult {
	info, err := gcal.TokenStatus(acc.Name)
	if err != nil {
		return checkResult{ok: false, msg: err.Error()}
	}
	if !info.Present {
		return checkResult{ok: false, msg: "no token — run: overseer daily to authorize"}
	}
	if !info.Valid {
		return checkResult{ok: false, msg: "token expired — run: overseer daily to refresh"}
	}
	until := time.Until(info.Expiry)
	if until > 0 {
		return checkResult{ok: true, msg: fmt.Sprintf("token valid (expires in %s)", formatDuration(until))}
	}
	return checkResult{ok: true, msg: "token valid"}
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
