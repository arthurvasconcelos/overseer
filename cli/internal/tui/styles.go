package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Shared styles — consistent across all overseer commands.
// Palette is built on 256-colour terminal codes to work in any modern terminal.
var (
	StyleHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))  // purple — section titles
	StyleAccent = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))             // pink   — keys, channels, usernames
	StyleOK     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))  // green  — success
	StyleWarn   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")) // amber  — warnings
	StyleError  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196")) // red    — errors
	StyleMuted  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))             // dark   — hints, empty state
	StyleDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))             // grey   — secondary info
	StyleNormal = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))             // light  — body text
)

// SectionHeader renders a styled section header with an optional badge.
//
//	▸ Jira / p24  ·  3 open
func SectionHeader(label, badge string) string {
	s := StyleHeader.Render("▸ " + label)
	if badge != "" {
		s += "  " + StyleMuted.Render("·  "+badge)
	}
	return s
}

// WarnLine renders a warning line in the style used by daily/repos output.
//
//	⚠  label: message
func WarnLine(label, msg string) string {
	return StyleWarn.Render("⚠  "+label+":") + " " + StyleMuted.Render(msg)
}

// UpdateNotice renders the "new version available" banner shown after commands.
func UpdateNotice(current, latest string) string {
	line1 := StyleWarn.Render(fmt.Sprintf("A new version is available: %s → %s", current, latest))
	line2 := StyleMuted.Render("Run ") + StyleWarn.Bold(true).Render("overseer update") + StyleMuted.Render(" to upgrade.")
	return "\n" + line1 + "\n" + line2
}

// Hyperlink renders text as an OSC 8 terminal hyperlink.
// Most modern terminals (iTerm2, Kitty, WezTerm, macOS Terminal ≥ 2.12) support this.
func Hyperlink(url, text string) string {
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

// Logo renders the overseer wordmark as a styled rounded box with an optional
// version badge. The badge colour reflects the release channel:
// stable = green, beta/rc = amber, dev = red.
func Logo(version string) string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("99")).
		Padding(0, 1).
		Render(StyleHeader.Render("▸ O V E R S E E R"))
	if version == "" {
		return box
	}
	return box + "  " + versionStyle(version).Render(version)
}

func versionStyle(version string) lipgloss.Style {
	if version == "dev" {
		return StyleError
	}
	if strings.Contains(version, "-") {
		return StyleWarn
	}
	return StyleOK
}
