package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ProductionPanic/how/internal/ollama"
	"github.com/ProductionPanic/how/internal/suggest"
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
	askFilling
	askFollowUp
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
	history  []suggest.Turn // earlier questions and answers, for follow-ups

	state   askState
	spinner spinner.Model
	input   textinput.Model
	follow  textinput.Model // follow-up question
	sugg    suggest.Suggestion
	result  AskResult
	note    string

	// Fill-in form for placeholders like <file>.
	command string            // command being filled in
	names   []string          // unique placeholder names, in order
	fields  []textinput.Model // one input per name
	field   int               // focused field
	pending AskAction         // what to do once the form is complete
	entries []string          // current directory, for the file picker
	pick    int               // highlighted picker entry, -1 for none
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

	fu := textinput.New()
	fu.Prompt = promptStyle.Render("› ")
	fu.Placeholder = "e.g. use wp-cli instead"
	fu.CharLimit = 0

	m := askModel{
		ctx: ctx, cancel: cancel,
		client: client, model: model, question: question,
		spinner: sp, input: ti, follow: fu,
	}
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return AskResult{}, err
	}
	return final.(askModel).result, nil
}

func (m askModel) fetch() tea.Cmd {
	return func() tea.Msg {
		s, err := suggest.Ask(m.ctx, m.client, m.model, m.history, m.question)
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
		case askFilling:
			return m.updateFilling(msg)
		case askFollowUp:
			return m.updateFollowUp(msg)
		}
	}
	return m, nil
}

func (m askModel) updateResult(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "y":
		return m.complete(ActionRun, m.sugg.Command)
	case "c":
		return m.complete(ActionCopy, m.sugg.Command)
	case "e":
		m.state = askEditing
		m.input.SetValue(m.sugg.Command)
		m.input.CursorEnd()
		m.note = ""
		return m, m.input.Focus()
	case "f":
		m.state = askFollowUp
		m.follow.SetValue("")
		m.note = ""
		return m, m.follow.Focus()
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
		m.input.Blur()
		return m.complete(ActionRun, cmd)
	case "esc":
		m.input.Blur()
		m.state = askResult
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// updateFollowUp reads a follow-up question and asks it with the current
// question and suggestion added to the history.
func (m askModel) updateFollowUp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		q := strings.TrimSpace(m.follow.Value())
		if q == "" {
			return m, nil
		}
		m.follow.Blur()
		m.history = append(m.history, suggest.Turn{Question: m.question, Answer: m.sugg})
		m.question = q
		m.state = askLoading
		return m, tea.Batch(m.spinner.Tick, m.fetch())
	case "esc":
		m.follow.Blur()
		m.state = askResult
		return m, nil
	}
	var cmd tea.Cmd
	m.follow, cmd = m.follow.Update(msg)
	return m, cmd
}

// complete runs or copies command, first asking for any placeholders in it.
func (m askModel) complete(action AskAction, command string) (tea.Model, tea.Cmd) {
	names := suggest.Placeholders(command)
	if len(names) == 0 {
		return m.act(action, command)
	}
	m.state = askFilling
	m.command = command
	m.names = names
	m.pending = action
	m.note = ""
	width := 0
	for _, n := range names {
		width = max(width, len(n))
	}
	m.fields = make([]textinput.Model, len(names))
	for i, n := range names {
		ti := textinput.New()
		ti.Prompt = promptStyle.Render(fmt.Sprintf("%-*s ", width+1, n+":"))
		ti.CharLimit = 0
		m.fields[i] = ti
	}
	m.field = 0
	m.pick = -1
	m.entries = listEntries(".")
	return m, m.fields[0].Focus()
}

// matches returns the picker entries for the focused field, or nil when it
// is not a file placeholder.
func (m askModel) matches() []string {
	name := m.names[m.field]
	if !isFilePlaceholder(name) {
		return nil
	}
	return filterEntries(m.entries, m.fields[m.field].Value(), wantsDir(name))
}

// act finishes with the command, copying it to the clipboard when asked.
func (m askModel) act(action AskAction, command string) (tea.Model, tea.Cmd) {
	if action == ActionCopy {
		if err := clipboard.WriteAll(command); err != nil {
			m.state = askResult
			m.note = errorStyle.Render("copy failed: " + err.Error())
			return m, nil
		}
	}
	return m.finish(AskResult{Action: action, Command: command})
}

func (m askModel) values() map[string]string {
	v := make(map[string]string, len(m.names))
	for i, n := range m.names {
		if val := m.fields[i].Value(); val != "" {
			v[n] = val
		}
	}
	return v
}

func (m askModel) focus(i int) (tea.Model, tea.Cmd) {
	m.fields[m.field].Blur()
	m.field = i
	m.pick = -1
	return m, m.fields[i].Focus()
}

func (m askModel) updateFilling(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.fields[m.field].Blur()
		m.state = askResult
		return m, nil
	case "enter":
		if matches := m.matches(); m.pick >= 0 && m.pick < len(matches) {
			m.fields[m.field].SetValue(matches[m.pick])
			m.fields[m.field].CursorEnd()
			m.pick = -1
		}
		if m.fields[m.field].Value() == "" {
			return m, nil
		}
		for i := range m.fields {
			if m.fields[i].Value() == "" {
				return m.focus(i)
			}
		}
		return m.act(m.pending, suggest.Fill(m.command, m.values()))
	case "down":
		if n := min(len(m.matches()), maxMatches); n > 0 {
			m.pick = min(m.pick+1, n-1)
			return m, nil
		}
		return m.focus((m.field + 1) % len(m.fields))
	case "up":
		if m.pick >= 0 {
			m.pick--
			return m, nil
		}
		return m.focus((m.field + len(m.fields) - 1) % len(m.fields))
	case "tab":
		return m.focus((m.field + 1) % len(m.fields))
	case "shift+tab":
		return m.focus((m.field + len(m.fields) - 1) % len(m.fields))
	}
	m.pick = -1
	var cmd tea.Cmd
	m.fields[m.field], cmd = m.fields[m.field].Update(msg)
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
		b.WriteString("\n " + helpLine("enter", "run", "e", "edit", "c", "copy", "f", "follow-up", "r", "retry", "q", "quit") + "\n")
		if m.note != "" {
			b.WriteString(" " + m.note + "\n")
		}

	case askEditing:
		b.WriteString(m.input.View() + "\n\n")
		b.WriteString(" " + helpLine("enter", "run", "esc", "back") + "\n")

	case askFollowUp:
		b.WriteString(commandBox.Render(commandStyle.Render(m.sugg.Command)) + "\n")
		b.WriteString(" " + m.follow.View() + "\n\n")
		b.WriteString(" " + helpLine("enter", "ask", "esc", "back") + "\n")

	case askFilling:
		b.WriteString(commandBox.Render(commandStyle.Render(suggest.Fill(m.command, m.values()))) + "\n")
		matches := m.matches()
		for i, f := range m.fields {
			b.WriteString(" " + f.View() + "\n")
			if i == m.field {
				b.WriteString(pickerView(matches, m.pick))
			}
		}
		verb := "run"
		if m.pending == ActionCopy {
			verb = "copy"
		}
		help := []string{"enter", "next / " + verb, "tab", "switch field", "esc", "back"}
		if matches != nil {
			help = append([]string{"↑↓", "pick"}, help...)
		}
		b.WriteString("\n " + helpLine(help...) + "\n")
		if m.note != "" {
			b.WriteString(" " + m.note + "\n")
		}

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

// pickerView renders the picker entries under a file field, highlighting
// the picked one.
func pickerView(matches []string, pick int) string {
	if len(matches) == 0 {
		return ""
	}
	var b strings.Builder
	for i, e := range matches[:min(len(matches), maxMatches)] {
		if i == pick {
			b.WriteString("   " + selectedRow.Render("› "+e) + "\n")
		} else {
			b.WriteString("     " + mutedStyle.Render(e) + "\n")
		}
	}
	if more := len(matches) - maxMatches; more > 0 {
		b.WriteString("     " + mutedStyle.Render(fmt.Sprintf("… %d more, type to filter", more)) + "\n")
	}
	return b.String()
}
