package ui

import (
	"errors"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type spinDoneMsg struct{ err error }

type spinModel struct {
	label   string
	done    string
	work    func() error
	spinner spinner.Model
	err     error
	over    bool
}

// RunSpinner shows label with a spinner while work runs. On success it
// leaves done on screen. Ctrl+c stops waiting and returns ErrInterrupted.
func RunSpinner(label, done string, work func() error) error {
	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = promptStyle
	final, err := tea.NewProgram(spinModel{label: label, done: done, work: work, spinner: sp}).Run()
	if err != nil {
		return err
	}
	return final.(spinModel).err
}

// ErrInterrupted is returned when the user stops a RunSpinner wait.
var ErrInterrupted = errors.New("interrupted")

func (m spinModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, func() tea.Msg { return spinDoneMsg{m.work()} })
}

func (m spinModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinDoneMsg:
		m.err, m.over = msg.err, true
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "esc" {
			m.err, m.over = ErrInterrupted, true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m spinModel) View() string {
	if m.over {
		if m.err == nil && m.done != "" {
			return successStyle.Render("✓ "+m.done) + "\n"
		}
		return ""
	}
	return m.spinner.View() + " " + mutedStyle.Render(m.label) + "\n"
}
