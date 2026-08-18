package claude

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/output"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

const worklogDateFormat = "2006-01-02"

// Workdir is one git root a session touched, with the branch it was on.
type Workdir struct {
	Root   string `json:"root"`
	Branch string `json:"branch"`
}

// WorklogRecord is one session's activity on one date. A session that spans
// midnight or resumes on a later day produces one record per date.
type WorklogRecord struct {
	Date       string    `json:"date"`
	Session    string    `json:"session"`
	Transcript string    `json:"transcript"`
	StartedAt  string    `json:"started_at"`
	EndedAt    string    `json:"ended_at"`
	Workdirs   []Workdir `json:"workdirs"`
	Prompts    []string  `json:"prompts"`
	Files      []string  `json:"files"`
}

// hookPayload is the JSON Claude Code writes to a hook's stdin.
type hookPayload struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	HookEventName  string `json:"hook_event_name"`
}

// transcriptEntry is a permissive view of one transcript JSONL line. The
// format is not a public contract, so every field is optional.
type transcriptEntry struct {
	Type         string             `json:"type"`
	Timestamp    string             `json:"timestamp"`
	SessionID    string             `json:"sessionId"`
	Cwd          string             `json:"cwd"`
	GitBranch    string             `json:"gitBranch"`
	PromptSource string             `json:"promptSource"`
	TrackingPath string             `json:"trackingPath"`
	Message      *transcriptMessage `json:"message"`
	Snapshot     *transcriptSnap    `json:"snapshot"`
}

type transcriptMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type transcriptSnap struct {
	TrackedFileBackups map[string]json.RawMessage `json:"trackedFileBackups"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// syntheticPrompt matches user records that carry promptSource but are not
// typed by a human — IDE notifications, slash command echoes, hook output.
func syntheticPrompt() *regexp.Regexp {
	return regexp.MustCompile(`^<(ide_opened_file|command-name|command-message|` +
		`local-command-stdout|local-command-caveat|system-reminder|bash-input|` +
		`task-notification|user-prompt-submit-hook)`)
}

// scratchpadPath reports whether p is a session scratchpad file rather than
// real project work.
func scratchpadPath(p string) bool {
	return strings.HasPrefix(p, "/private/tmp/claude-") || strings.HasPrefix(p, "/tmp/claude-")
}

func worklogDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "overseer", "worklog")
}

func worklogFile(date string) string {
	return filepath.Join(worklogDir(), date+".jsonl")
}

func worklogCmd(cfg *config.Config) *cobra.Command {
	root := &cobra.Command{
		Use:   "worklog",
		Short: "Capture and read Claude Code session activity",
	}
	root.AddCommand(worklogCaptureCmd())
	root.AddCommand(worklogShowCmd())
	root.AddCommand(worklogInstallCmd(cfg))
	root.AddCommand(worklogUninstallCmd(cfg))
	return root
}

func worklogCaptureCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "capture",
		Short: "Append a worklog record from a SessionEnd hook payload on stdin",
		RunE: func(_ *cobra.Command, _ []string) error {
			// A hook that fails is noise on every session end. Report to
			// stderr and always exit 0.
			if err := runWorklogCapture(); err != nil {
				fmt.Fprintf(os.Stderr, "worklog capture: %v\n", err)
			}
			return nil
		},
	}
}

func worklogShowCmd() *cobra.Command {
	date := ""
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show captured sessions for a date",
		RunE: func(_ *cobra.Command, _ []string) error {
			if date == "" {
				date = time.Now().Format(worklogDateFormat)
			}
			return runWorklogShow(date)
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "date to show (YYYY-MM-DD, default today)")
	return cmd
}

func runWorklogCapture() error {
	payload := hookPayload{}
	if err := json.NewDecoder(os.Stdin).Decode(&payload); err != nil {
		return fmt.Errorf("reading hook payload: %w", err)
	}
	if payload.TranscriptPath == "" {
		return fmt.Errorf("hook payload has no transcript_path")
	}

	records, err := parseTranscript(payload.TranscriptPath)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}

	for _, record := range records {
		if record.Session == "" {
			record.Session = payload.SessionID
		}
		if err := writeWorklogRecord(record); err != nil {
			return err
		}
	}
	return nil
}

// parseTranscript reads a transcript and returns one record per date it covers.
func parseTranscript(path string) ([]WorklogRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening transcript: %w", err)
	}
	defer file.Close()

	// Transcript lines carry whole tool results and can be very large.
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 64*1024*1024)

	synthetic := syntheticPrompt()
	byDate := map[string]*WorklogRecord{}
	branches := map[string]string{}
	seenWorkdir := map[string]map[string]bool{}
	seenFile := map[string]map[string]bool{}

	for scanner.Scan() {
		entry := transcriptEntry{}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.Timestamp == "" || len(entry.Timestamp) < len(worklogDateFormat) {
			continue
		}
		date := entry.Timestamp[:len(worklogDateFormat)]

		record, ok := byDate[date]
		if !ok {
			record = &WorklogRecord{Date: date, Transcript: path, StartedAt: entry.Timestamp}
			byDate[date] = record
			seenWorkdir[date] = map[string]bool{}
			seenFile[date] = map[string]bool{}
		}
		if entry.Timestamp < record.StartedAt {
			record.StartedAt = entry.Timestamp
		}
		if entry.Timestamp > record.EndedAt {
			record.EndedAt = entry.Timestamp
		}
		if record.Session == "" {
			record.Session = entry.SessionID
		}

		if entry.Cwd != "" {
			if root := gitRoot(entry.Cwd); root != "" && !seenWorkdir[date][root] {
				seenWorkdir[date][root] = true
				if entry.GitBranch != "" && branches[root] == "" {
					branches[root] = entry.GitBranch
				}
				record.Workdirs = append(record.Workdirs, Workdir{Root: root, Branch: branches[root]})
			}
		}

		if entry.Type == "user" && entry.PromptSource != "" {
			if text := promptText(entry.Message); text != "" && !synthetic.MatchString(text) {
				record.Prompts = append(record.Prompts, text)
			}
		}

		for _, path := range touchedFiles(entry) {
			if scratchpadPath(path) || seenFile[date][path] {
				continue
			}
			seenFile[date][path] = true
			record.Files = append(record.Files, path)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading transcript: %w", err)
	}

	records := make([]WorklogRecord, 0, len(byDate))
	for _, record := range byDate {
		sort.Strings(record.Files)
		if record.Prompts == nil {
			record.Prompts = []string{}
		}
		if record.Files == nil {
			record.Files = []string{}
		}
		if record.Workdirs == nil {
			record.Workdirs = []Workdir{}
		}
		records = append(records, *record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Date < records[j].Date })
	return records, nil
}

// promptText joins the text blocks of a message, ignoring tool results.
func promptText(message *transcriptMessage) string {
	if message == nil || len(message.Content) == 0 {
		return ""
	}

	text := ""
	if err := json.Unmarshal(message.Content, &text); err == nil {
		return strings.TrimSpace(text)
	}

	blocks := []contentBlock{}
	if err := json.Unmarshal(message.Content, &blocks); err != nil {
		return ""
	}
	parts := []string{}
	for _, block := range blocks {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			parts = append(parts, strings.TrimSpace(block.Text))
		}
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

// touchedFiles returns file paths recorded by Claude Code's file history.
// Edits made through Bash never appear here — git is the authoritative source.
func touchedFiles(entry transcriptEntry) []string {
	paths := []string{}
	if entry.Type == "file-history-delta" && entry.TrackingPath != "" {
		paths = append(paths, entry.TrackingPath)
	}
	if entry.Type == "file-history-snapshot" && entry.Snapshot != nil {
		for path := range entry.Snapshot.TrackedFileBackups {
			paths = append(paths, path)
		}
	}
	return paths
}

func gitRoot(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// writeWorklogRecord appends a record, replacing any earlier record for the
// same session and date so a resumed session does not duplicate.
func writeWorklogRecord(record WorklogRecord) error {
	if err := os.MkdirAll(worklogDir(), 0o755); err != nil {
		return fmt.Errorf("creating worklog dir: %w", err)
	}

	existing, err := readWorklog(record.Date)
	if err != nil {
		return err
	}
	kept := make([]WorklogRecord, 0, len(existing)+1)
	for _, candidate := range existing {
		if candidate.Session != record.Session {
			kept = append(kept, candidate)
		}
	}
	kept = append(kept, record)
	sort.Slice(kept, func(i, j int) bool { return kept[i].StartedAt < kept[j].StartedAt })

	lines := strings.Builder{}
	for _, candidate := range kept {
		encoded, err := json.Marshal(candidate)
		if err != nil {
			return fmt.Errorf("encoding record: %w", err)
		}
		lines.Write(encoded)
		lines.WriteByte('\n')
	}
	return os.WriteFile(worklogFile(record.Date), []byte(lines.String()), 0o644)
}

func readWorklog(date string) ([]WorklogRecord, error) {
	file, err := os.Open(worklogFile(date))
	if err != nil {
		if os.IsNotExist(err) {
			return []WorklogRecord{}, nil
		}
		return nil, fmt.Errorf("opening worklog: %w", err)
	}
	defer file.Close()

	records := []WorklogRecord{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	for scanner.Scan() {
		record := WorklogRecord{}
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue
		}
		records = append(records, record)
	}
	return records, scanner.Err()
}

func runWorklogShow(date string) error {
	records, err := readWorklog(date)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		return output.PrintJSON(records)
	}

	fmt.Println(tui.SectionHeader("claude worklog", date))
	fmt.Println()
	if len(records) == 0 {
		fmt.Println("  " + tui.StyleMuted.Render("no sessions captured"))
		return nil
	}

	for _, record := range records {
		roots := []string{}
		for _, workdir := range record.Workdirs {
			roots = append(roots, filepath.Base(workdir.Root))
		}
		fmt.Printf("  %s  %s  %s\n",
			tui.StyleAccent.Render(clockRange(record.StartedAt, record.EndedAt)),
			tui.StyleNormal.Render(strings.Join(roots, ", ")),
			tui.StyleDim.Render(fmt.Sprintf("%s, %s",
				plural(len(record.Prompts), "prompt"), plural(len(record.Files), "file"))),
		)
		for _, prompt := range record.Prompts {
			fmt.Printf("      %s %s\n", tui.StyleMuted.Render("·"), tui.StyleDim.Render(truncateStr(oneLine(prompt), 90)))
		}
	}
	return nil
}

func clockRange(startedAt, endedAt string) string {
	return localClock(startedAt) + "-" + localClock(endedAt)
}

func localClock(timestamp string) string {
	parsed, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return "--:--"
	}
	return parsed.Local().Format("15:04")
}

func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func plural(count int, noun string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, noun)
	}
	return fmt.Sprintf("%d %ss", count, noun)
}
