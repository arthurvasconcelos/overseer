package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	stylePromptLabel = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	stylePromptHint  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	stylePromptDone  = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
)

type promptModel struct {
	label     string
	input     textinput.Model
	done      bool
	canceled  bool
}

func newPromptModel(label, defaultVal, placeholder string) promptModel {
	ti := textinput.New()
	ti.SetValue(defaultVal)
	ti.Placeholder = placeholder
	ti.Focus()
	ti.CharLimit = 512
	ti.Width = 60
	return promptModel{label: label, input: ti}
}

func (m promptModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m promptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.done = true
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.canceled = true
			m.done = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m promptModel) View() string {
	if m.done {
		if m.canceled {
			return styleCancel.Render("cancelled") + "\n"
		}
		val := m.input.Value()
		if val == "" {
			val = m.input.Placeholder
		}
		return stylePromptLabel.Render(m.label+":") + " " + stylePromptDone.Render(val) + "\n"
	}
	hint := ""
	if m.input.Placeholder != "" && m.input.Value() == "" {
		hint = " " + stylePromptHint.Render("(default: "+m.input.Placeholder+")")
	}
	return stylePromptLabel.Render(m.label) + hint + "\n" + m.input.View() + "\n"
}

// Prompt displays an interactive single-line text input and returns the entered value.
// If the user submits an empty value and a defaultVal is provided, the defaultVal is returned.
// Returns an error if the user cancels.
func Prompt(label, defaultVal, placeholder string) (string, error) {
	m := newPromptModel(label, defaultVal, placeholder)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("tui: %w", err)
	}
	final := result.(promptModel)
	if final.canceled {
		return "", fmt.Errorf("cancelled")
	}
	val := final.input.Value()
	if val == "" {
		val = placeholder
	}
	return val, nil
}
