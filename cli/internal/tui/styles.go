package tui

import (
	"fmt"

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

// Logo renders the overseer wordmark as a styled rounded box with an optional
// version badge. Pass an empty string or "dev" to omit the badge.
func Logo(version string) string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("99")).
		Padding(0, 1).
		Render(StyleHeader.Render("▸ O V E R S E E R"))
	if version != "" && version != "dev" {
		return box + "  " + StyleMuted.Render(version)
	}
	return box
}
