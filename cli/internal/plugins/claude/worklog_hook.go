package claude

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

const (
	worklogHookScript = "session-end-worklog.sh"
	worklogHookEvent  = "SessionEnd"
	worklogHookCmd    = "\"$HOME\"/.claude/hooks/" + worklogHookScript
)

// worklogHookBody resolves overseer at run time rather than baking in a path,
// so the same brain works on every machine. It never fails the session.
const worklogHookBody = `#!/usr/bin/env bash
# Appends a Claude Code session record to the overseer worklog.
# Registered as a SessionEnd hook — must never fail the session.

set -uo pipefail

OVERSEER="$(command -v overseer || true)"
if [[ "${OVERSEER}" == "" ]] && [[ -x "${HOME}/bin/overseer" ]]; then
    OVERSEER="${HOME}/bin/overseer"
fi
if [[ "${OVERSEER}" == "" ]]; then
    exit 0
fi

"${OVERSEER}" claude worklog capture || true
exit 0
`

func worklogInstallCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install the SessionEnd hook that captures worklog records",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runWorklogInstall(cfg)
		},
	}
}

func worklogUninstallCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the SessionEnd worklog hook",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runWorklogUninstall(cfg)
		},
	}
}

func runWorklogInstall(cfg *config.Config) error {
	claudeDir := brainClaudeDir(cfg)
	hooksDir := filepath.Join(claudeDir, "hooks")
	scriptPath := filepath.Join(hooksDir, worklogHookScript)

	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("creating hooks dir: %w", err)
	}
	if err := os.WriteFile(scriptPath, []byte(worklogHookBody), 0o755); err != nil {
		return fmt.Errorf("writing hook script: %w", err)
	}
	fmt.Printf("  %s  %s\n", tui.StyleOK.Render("write "), tui.StyleNormal.Render(scriptPath))

	home, _ := os.UserHomeDir()
	localHook := filepath.Join(home, ".claude", "hooks", worklogHookScript)
	if err := makeLink(scriptPath, localHook, false, worklogHookScript); err != nil {
		return err
	}

	settingsPath := filepath.Join(claudeDir, "settings.json")
	settings, err := readClaudeSettings(settingsPath)
	if err != nil {
		return err
	}
	added, err := addWorklogHook(settings)
	if err != nil {
		return err
	}
	if added {
		if err := writeClaudeSettings(settingsPath, settings); err != nil {
			return err
		}
		fmt.Printf("  %s  %s\n", tui.StyleOK.Render("hook  "), tui.StyleNormal.Render(worklogHookEvent+" registered in settings.json"))
	} else {
		fmt.Printf("  %s  %s\n", tui.StyleMuted.Render("skip  "), tui.StyleDim.Render(worklogHookEvent+" already registered"))
	}
	return nil
}

func runWorklogUninstall(cfg *config.Config) error {
	claudeDir := brainClaudeDir(cfg)

	settingsPath := filepath.Join(claudeDir, "settings.json")
	settings, err := readClaudeSettings(settingsPath)
	if err != nil {
		return err
	}
	removed, err := removeWorklogHook(settings)
	if err != nil {
		return err
	}
	if removed {
		if err := writeClaudeSettings(settingsPath, settings); err != nil {
			return err
		}
		fmt.Printf("  %s  %s\n", tui.StyleOK.Render("hook  "), tui.StyleNormal.Render(worklogHookEvent+" removed from settings.json"))
	} else {
		fmt.Printf("  %s  %s\n", tui.StyleMuted.Render("skip  "), tui.StyleDim.Render("hook not registered"))
	}

	home, _ := os.UserHomeDir()
	localHook := filepath.Join(home, ".claude", "hooks", worklogHookScript)
	info, err := os.Lstat(localHook)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(localHook); err != nil {
			return fmt.Errorf("removing symlink: %w", err)
		}
		fmt.Printf("  %s  %s\n", tui.StyleMuted.Render("unlink"), tui.StyleNormal.Render(worklogHookScript))
	}

	fmt.Println()
	fmt.Println("  " + tui.StyleDim.Render("hook script kept in brain — delete it manually if unwanted"))
	return nil
}

// claudeSettings preserves the on-disk key order and keeps every untouched
// value as raw bytes, so rewriting the file leaves unrelated settings — and
// their formatting — byte-for-byte identical.
type claudeSettings struct {
	order  []string
	values map[string]json.RawMessage
}

func readClaudeSettings(path string) (*claudeSettings, error) {
	settings := &claudeSettings{values: map[string]json.RawMessage{}}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return settings, nil
		}
		return nil, fmt.Errorf("reading settings.json: %w", err)
	}
	if err := json.Unmarshal(data, &settings.values); err != nil {
		return nil, fmt.Errorf("parsing settings.json: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	if _, err := decoder.Token(); err != nil {
		return nil, fmt.Errorf("parsing settings.json: %w", err)
	}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("parsing settings.json: %w", err)
		}
		key, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("parsing settings.json: unexpected token %v", token)
		}
		settings.order = append(settings.order, key)
		skipped := json.RawMessage{}
		if err := decoder.Decode(&skipped); err != nil {
			return nil, fmt.Errorf("parsing settings.json: %w", err)
		}
	}
	return settings, nil
}

func (s *claudeSettings) get(key string) (json.RawMessage, bool) {
	value, ok := s.values[key]
	return value, ok
}

func (s *claudeSettings) set(key string, value json.RawMessage) {
	if _, ok := s.values[key]; !ok {
		s.order = append(s.order, key)
	}
	s.values[key] = value
}

func (s *claudeSettings) remove(key string) {
	if _, ok := s.values[key]; !ok {
		return
	}
	delete(s.values, key)
	kept := make([]string, 0, len(s.order))
	for _, candidate := range s.order {
		if candidate != key {
			kept = append(kept, candidate)
		}
	}
	s.order = kept
}

func writeClaudeSettings(path string, settings *claudeSettings) error {
	if len(settings.order) == 0 {
		return os.WriteFile(path, []byte("{}\n"), 0o644)
	}

	out := strings.Builder{}
	out.WriteString("{\n")
	for i, key := range settings.order {
		encodedKey, err := json.Marshal(key)
		if err != nil {
			return fmt.Errorf("encoding settings.json: %w", err)
		}
		out.WriteString("  ")
		out.Write(encodedKey)
		out.WriteString(": ")
		out.Write(settings.values[key])
		if i < len(settings.order)-1 {
			out.WriteByte(',')
		}
		out.WriteByte('\n')
	}
	out.WriteString("}\n")
	return os.WriteFile(path, []byte(out.String()), 0o644)
}

// indentValue re-indents an encoded value so it nests correctly at top level.
func indentValue(value any) (json.RawMessage, error) {
	encoded, err := json.MarshalIndent(value, "  ", "  ")
	if err != nil {
		return nil, fmt.Errorf("encoding hooks: %w", err)
	}
	return encoded, nil
}

func decodeHooks(settings *claudeSettings) map[string]any {
	hooks := map[string]any{}
	if raw, ok := settings.get("hooks"); ok {
		if err := json.Unmarshal(raw, &hooks); err != nil {
			return map[string]any{}
		}
	}
	return hooks
}

// addWorklogHook registers the hook, returning false when already present.
func addWorklogHook(settings *claudeSettings) (bool, error) {
	hooks := decodeHooks(settings)
	matchers, _ := hooks[worklogHookEvent].([]any)
	if worklogHookRegistered(matchers) {
		return false, nil
	}

	hooks[worklogHookEvent] = append(matchers, map[string]any{
		"hooks": []any{
			map[string]any{"type": "command", "command": worklogHookCmd},
		},
	})
	encoded, err := indentValue(hooks)
	if err != nil {
		return false, err
	}
	settings.set("hooks", encoded)
	return true, nil
}

// removeWorklogHook drops the hook, returning false when nothing was removed.
func removeWorklogHook(settings *claudeSettings) (bool, error) {
	hooks := decodeHooks(settings)
	matchers, _ := hooks[worklogHookEvent].([]any)
	kept := []any{}
	for _, matcher := range matchers {
		if !matcherHasWorklogHook(matcher) {
			kept = append(kept, matcher)
		}
	}
	if len(kept) == len(matchers) {
		return false, nil
	}

	if len(kept) == 0 {
		delete(hooks, worklogHookEvent)
	} else {
		hooks[worklogHookEvent] = kept
	}
	if len(hooks) == 0 {
		settings.remove("hooks")
		return true, nil
	}

	encoded, err := indentValue(hooks)
	if err != nil {
		return false, err
	}
	settings.set("hooks", encoded)
	return true, nil
}

func worklogHookRegistered(matchers []any) bool {
	for _, matcher := range matchers {
		if matcherHasWorklogHook(matcher) {
			return true
		}
	}
	return false
}

func matcherHasWorklogHook(matcher any) bool {
	entry, ok := matcher.(map[string]any)
	if !ok {
		return false
	}
	inner, ok := entry["hooks"].([]any)
	if !ok {
		return false
	}
	for _, hook := range inner {
		definition, ok := hook.(map[string]any)
		if !ok {
			continue
		}
		if command, ok := definition["command"].(string); ok && command == worklogHookCmd {
			return true
		}
	}
	return false
}
