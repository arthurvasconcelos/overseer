package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	styleTitle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	styleCursor   = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	styleSelected = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	styleNormal   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	styleSub      = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleCancel   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Faint(true)
)

// SelectItem is a single option shown in the selector.
type SelectItem struct {
	Title    string
	Subtitle string // optional, shown dimmed beside the title
}

// selectModel is the bubbletea model for the arrow-key selector.
type selectModel struct {
	title    string
	items    []SelectItem
	cursor   int
	chosen   int  // -1 = cancelled
	quitting bool
}

func (m selectModel) Init() tea.Cmd { return nil }

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "enter", " ":
			m.chosen = m.cursor
			m.quitting = true
			return m, tea.Quit
		case "q", "ctrl+c", "esc":
			m.chosen = -1
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m selectModel) View() string {
	if m.quitting {
		if m.chosen == -1 {
			return styleCancel.Render("cancelled") + "\n"
		}
		item := m.items[m.chosen]
		line := styleSelected.Render("✓ " + item.Title)
		if item.Subtitle != "" {
			line += " " + styleSub.Render(item.Subtitle)
		}
		return line + "\n"
	}

	s := styleTitle.Render(m.title) + "\n\n"
	for i, item := range m.items {
		cursor := "  "
		var line string
		if i == m.cursor {
			cursor = styleCursor.Render("▸ ")
			line = styleSelected.Render(item.Title)
		} else {
			line = styleNormal.Render(item.Title)
		}
		if item.Subtitle != "" {
			line += " " + styleSub.Render(item.Subtitle)
		}
		s += cursor + line + "\n"
	}
	s += "\n" + styleSub.Render("↑/↓ to move  •  enter to select  •  esc to cancel")
	return s
}

// Select shows an interactive arrow-key selector and returns the chosen index.
// Returns -1 if the user cancelled.
func Select(title string, items []SelectItem) (int, error) {
	m := selectModel{
		title:  title,
		items:  items,
		chosen: -1,
	}
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return -1, fmt.Errorf("tui: %w", err)
	}
	return result.(selectModel).chosen, nil
}
