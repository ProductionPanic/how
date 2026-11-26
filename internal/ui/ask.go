package ui

import (
	"context"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ProductionPanic/help-cli/internal/ollama"
	"github.com/ProductionPanic/help-cli/internal/suggest"
)

// AskAction is what the user chose to do with the suggestion.
type AskAction int

const (
	ActionNone AskAction = iota
	ActionRun
	ActionCopy
)

// AskResult is returned once the ask TUI exits.
type AskResult struct {
	Action  AskAction
	Command string
	Err     error
}

type askState int

const (
	askLoading askState = iota
	askResult
	askEditing
	askDone
)

type suggestionMsg struct {
	s   suggest.Suggestion
	err error
}

type askModel struct {
	ctx      context.Context
	cancel   context.CancelFunc
	client   *ollama.Client
	model    string
	question string

	state   askState
	spinner spinner.Model
	input   textinput.Model
	sugg    suggest.Suggestion
	result  AskResult
	note    string
}

// RunAsk shows a spinner while the model thinks, then lets the user run,
// edit, copy or retry the suggested command.
func RunAsk(client *ollama.Client, model, question string) (AskResult, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = promptStyle

	ti := textinput.New()
	ti.Prompt = promptStyle.Render("$ ")
	ti.CharLimit = 0

	m := askModel{
		ctx: ctx, cancel: cancel,
		client: client, model: model, question: question,
		spinner: sp, input: ti,
	}
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return AskResult{}, err
	}
	return final.(askModel).result, nil
}

func (m askModel) fetch() tea.Cmd {
	return func() tea.Msg {
		s, err := suggest.Ask(m.ctx, m.client, m.model, m.question)
		return suggestionMsg{s, err}
	}
}

func (m askModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.fetch())
}

func (m askModel) finish(r AskResult) (tea.Model, tea.Cmd) {
	m.result = r
	m.state = askDone
	m.cancel()
	return m, tea.Quit
}

func (m askModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case suggestionMsg:
		if msg.err != nil {
			return m.finish(AskResult{Err: msg.err})
		}
		m.sugg = msg.s
		m.state = askResult
		return m, nil

	case spinner.TickMsg:
		if m.state != askLoading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m.finish(AskResult{})
		}
		switch m.state {
		case askLoading:
			if msg.String() == "esc" || msg.String() == "q" {
				return m.finish(AskResult{})
			}
		case askResult:
			return m.updateResult(msg)
		case askEditing:
			return m.updateEditing(msg)
		}
	}
	return m, nil
}

func (m askModel) updateResult(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "y":
		return m.finish(AskResult{Action: ActionRun, Command: m.sugg.Command})
	case "c":
		if err := clipboard.WriteAll(m.sugg.Command); err != nil {
			m.note = errorStyle.Render("copy failed: " + err.Error())
			return m, nil
		}
		return m.finish(AskResult{Action: ActionCopy, Command: m.sugg.Command})
	case "e":
		m.state = askEditing
		m.input.SetValue(m.sugg.Command)
		m.input.CursorEnd()
		m.note = ""
		return m, m.input.Focus()
	case "r":
		m.state = askLoading
		m.note = ""
		return m, tea.Batch(m.spinner.Tick, m.fetch())
	case "q", "esc", "n":
		return m.finish(AskResult{})
	}
	return m, nil
}

func (m askModel) updateEditing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		cmd := strings.TrimSpace(m.input.Value())
		if cmd == "" {
			return m, nil
		}
		return m.finish(AskResult{Action: ActionRun, Command: cmd})
	case "esc":
		m.input.Blur()
		m.state = askResult
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m askModel) View() string {
	var b strings.Builder
	switch m.state {
	case askLoading:
		b.WriteString(m.spinner.View() + " " + mutedStyle.Render("Asking "+m.model+"…"))
		b.WriteString("\n")

	case askResult:
		box := commandBox
		if m.sugg.Dangerous {
			box = dangerBox
		}
		b.WriteString(box.Render(commandStyle.Render(m.sugg.Command)) + "\n")
		if m.sugg.Explanation != "" {
			b.WriteString(" " + mutedStyle.Render(m.sugg.Explanation) + "\n")
		}
		if m.sugg.Dangerous {
			b.WriteString(" " + warnStyle.Render("⚠ This command can change or delete data. Check it before running.") + "\n")
		}
		b.WriteString("\n " + helpLine("enter", "run", "e", "edit", "c", "copy", "r", "retry", "q", "quit") + "\n")
		if m.note != "" {
			b.WriteString(" " + m.note + "\n")
		}

	case askEditing:
		b.WriteString(m.input.View() + "\n\n")
		b.WriteString(" " + helpLine("enter", "run", "esc", "back") + "\n")

	case askDone:
		// Leave a compact record in the scrollback.
		switch m.result.Action {
		case ActionRun:
			b.WriteString(promptStyle.Render("$ ") + commandStyle.Render(m.result.Command) + "\n")
		case ActionCopy:
			b.WriteString(promptStyle.Render("$ ") + commandStyle.Render(m.result.Command) + "  " + successStyle.Render("✓ copied") + "\n")
		}
	}
	return b.String()
}
