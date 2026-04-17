package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/arthurvasconcelos/overseer/internal/config"
	jiraclient "github.com/arthurvasconcelos/overseer/internal/jira"
	"github.com/arthurvasconcelos/overseer/internal/notify"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

var focusIssue string
var focusName string
var focusInstanceFlag string

var focusCmd = &cobra.Command{
	Use:   "focus [duration]",
	Short: "Start a timed focus session (default 25m; use --issue to log time to Jira)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runFocus,
}

func init() {
	focusCmd.Flags().StringVar(&focusIssue, "issue", "", "Jira issue key to log time against when done")
	focusCmd.Flags().StringVar(&focusName, "name", "", "Session label")
	focusCmd.Flags().StringVar(&focusInstanceFlag, "instance", "", "Jira instance name (used with --issue)")
	rootCmd.AddCommand(focusCmd)
}

func runFocus(_ *cobra.Command, args []string) error {
	durStr := "25m"
	if len(args) > 0 {
		durStr = args[0]
	}

	seconds, err := focusParseWorkDuration(durStr)
	if err != nil {
		return err
	}
	duration := time.Duration(seconds) * time.Second

	label := focusName
	if label == "" && focusIssue != "" {
		label = focusIssue
	} else if label == "" {
		label = "focus session"
	}

	fmt.Printf("\n  %s\n", tui.SectionHeader("focus", label))
	fmt.Printf("  %s\n\n", tui.StyleMuted.Render(formatCountdown(duration)+" session · ctrl+c to stop"))

	startTime := time.Now()
	end := startTime.Add(duration)
	elapsed := duration

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

outer:
	for {
		select {
		case t := <-ticker.C:
			remaining := end.Sub(t)
			if remaining <= 0 {
				fmt.Printf("\r\033[K  %s\n\n", tui.StyleOK.Render("✓ done!"))
				break outer
			}
			fmt.Printf("\r\033[K  %s remaining", tui.StyleHeader.Render(formatCountdown(remaining)))
		case <-sigCh:
			elapsed = time.Since(startTime)
			fmt.Printf("\r\033[K  %s\n\n", tui.StyleWarn.Render("⚡ stopped"))
			break outer
		}
	}

	_ = notify.Send("Focus session complete", label, "overseer")

	if focusIssue != "" && elapsed > 0 {
		mins := int(elapsed.Minutes())
		if mins < 1 {
			mins = 1
		}
		logDur := fmt.Sprintf("%dm", mins)

		ok, err := tui.Confirm(fmt.Sprintf("Log %s to %s?", logDur, focusIssue))
		if err != nil || !ok {
			return nil
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		if len(cfg.Integrations.Jira) == 0 {
			return fmt.Errorf("no Jira instances configured")
		}
		inst := cfg.Integrations.Jira[0]
		if focusInstanceFlag != "" {
			found := false
			for _, i := range cfg.Integrations.Jira {
				if i.Name == focusInstanceFlag {
					inst = i
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("Jira instance %q not found", focusInstanceFlag)
			}
		}

		email, err := secrets.ReadAs(inst.Email, inst.OPAccount)
		if err != nil {
			return fmt.Errorf("resolving email: %w", err)
		}
		token, err := secrets.ReadAs(inst.Token, inst.OPAccount)
		if err != nil {
			return fmt.Errorf("resolving token: %w", err)
		}
		client := jiraclient.New(inst.BaseURL, email, token)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := client.AddWorklog(ctx, focusIssue, mins*60, ""); err != nil {
			fmt.Println("  " + tui.WarnLine("worklog", err.Error()))
		} else {
			fmt.Printf("  %s\n", tui.StyleOK.Render("✓ logged "+logDur+" to "+focusIssue))
		}
	}

	return nil
}

func formatCountdown(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func focusParseWorkDuration(s string) (int, error) {
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
