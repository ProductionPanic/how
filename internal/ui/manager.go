package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ProductionPanic/help-cli/internal/config"
	"github.com/ProductionPanic/help-cli/internal/ollama"
)

// Recommended small models that work well for shell suggestions.
var recommended = []struct{ Name, Size, Desc string }{
	{"qwen2.5-coder:1.5b", "1.0 GB", "fastest, good at shell commands"},
	{"qwen2.5-coder:3b", "1.9 GB", "best balance of speed and quality"},
	{"llama3.2:3b", "2.0 GB", "solid general-purpose model"},
	{"gemma3:1b", "0.8 GB", "tiny, for low-memory machines"},
	{"phi4-mini", "2.5 GB", "strong reasoning for its size"},
	{"qwen2.5-coder:7b", "4.7 GB", "most accurate, slower"},
}

type item struct {
	name      string
	installed bool
	size      string
	params    string
	desc      string
}

type mgrState int

const (
	mgrBrowse mgrState = iota
	mgrConfirmDelete
	mgrInput
	mgrPulling
)

type (
	modelsMsg struct {
		models []ollama.Model
		err    error
	}
	pullMsg     ollama.PullProgress
	pullDoneMsg struct {
		name string
		err  error
	}
	deletedMsg struct {
		name string
		err  error
	}
)

type managerModel struct {
	client *ollama.Client
	cfg    config.Config

	state   mgrState
	loading bool
	items   []item
	cursor  int
	status  string
	reachOK bool

	input    textinput.Model
	spinner  spinner.Model
	bar      progress.Model
	pullName string
	pullStat ollama.PullProgress
	pullCh   chan tea.Msg
	cancel   context.CancelFunc
	width    int
}

// RunManager opens the model manager TUI.
func RunManager(client *ollama.Client, cfg config.Config) error {
	ti := textinput.New()
	ti.Placeholder = "e.g. qwen2.5-coder:3b"
	ti.Prompt = promptStyle.Render("pull › ")

	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = promptStyle

	m := managerModel{
		client:  client,
		cfg:     cfg,
		loading: true,
		input:   ti,
		spinner: sp,
		bar:     progress.New(progress.WithDefaultGradient(), progress.WithWidth(40)),
	}
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func (m managerModel) loadModels() tea.Cmd {
	return func() tea.Msg {
		models, err := m.client.List(context.Background())
		return modelsMsg{models, err}
	}
}

func (m managerModel) Init() tea.Cmd {
	return tea.Batch(m.loadModels(), m.spinner.Tick)
}

func (m *managerModel) setItems(models []ollama.Model) {
	sort.Slice(models, func(i, j int) bool { return models[i].Name < models[j].Name })
	var items []item
	for _, mo := range models {
		items = append(items, item{
			name: mo.Name, installed: true,
			size: HumanSize(mo.Size), params: mo.Details.ParameterSize,
		})
	}
	for _, r := range recommended {
		have := false
		for _, mo := range models {
			if ollama.SameModel(mo.Name, r.Name) {
				have = true
				break
			}
		}
		if !have {
			items = append(items, item{name: r.Name, size: "~" + r.Size, desc: r.Desc})
		}
	}
	m.items = items
	if m.cursor >= len(items) {
		m.cursor = max(0, len(items)-1)
	}
}

func (m managerModel) selected() *item {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return nil
	}
	return &m.items[m.cursor]
}

func (m managerModel) activeInstalled() bool {
	for _, it := range m.items {
		if it.installed && ollama.SameModel(it.name, m.cfg.Model) {
			return true
		}
	}
	return false
}

func (m *managerModel) use(name string) {
	m.cfg.Model = name
	if err := m.cfg.Save(); err != nil {
		m.status = errorStyle.Render("could not save config: " + err.Error())
		return
	}
	m.status = successStyle.Render("✓ now using " + name)
}

func (m managerModel) startPull(name string) (tea.Model, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan tea.Msg, 64)
	m.state = mgrPulling
	m.pullName = name
	m.pullStat = ollama.PullProgress{Status: "starting"}
	m.pullCh = ch
	m.cancel = cancel
	m.status = ""
	go func() {
		err := m.client.Pull(ctx, name, func(p ollama.PullProgress) {
			select {
			case ch <- pullMsg(p):
			default: // drop updates if the UI is behind
			}
		})
		ch <- pullDoneMsg{name, err}
		close(ch)
	}()
	return m, tea.Batch(waitFor(ch), m.spinner.Tick)
}

func waitFor(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func (m managerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.bar.Width = min(60, max(20, msg.Width-30))
		return m, nil

	case spinner.TickMsg:
		if !m.loading && m.state != mgrPulling {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case modelsMsg:
		m.loading = false
		if msg.err != nil {
			m.reachOK = false
			m.status = errorStyle.Render(msg.err.Error() + " — is `ollama serve` running?")
			m.setItems(nil)
			return m, nil
		}
		m.reachOK = true
		m.setItems(msg.models)
		return m, nil

	case pullMsg:
		m.pullStat = ollama.PullProgress(msg)
		return m, waitFor(m.pullCh)

	case pullDoneMsg:
		m.state = mgrBrowse
		m.cancel = nil
		switch {
		case msg.err == context.Canceled:
			m.status = mutedStyle.Render("download of " + msg.name + " cancelled")
		case msg.err != nil:
			m.status = errorStyle.Render("pull failed: " + msg.err.Error())
		default:
			m.status = successStyle.Render("✓ downloaded " + msg.name)
			if m.cfg.Model == "" || !m.activeInstalled() {
				m.use(msg.name)
			} else {
				m.status += mutedStyle.Render(" — press enter to use it")
			}
		}
		m.loading = true
		return m, tea.Batch(m.loadModels(), m.spinner.Tick)

	case deletedMsg:
		if msg.err != nil {
			m.status = errorStyle.Render("delete failed: " + msg.err.Error())
		} else {
			m.status = successStyle.Render("✓ deleted " + msg.name)
			if ollama.SameModel(msg.name, m.cfg.Model) {
				m.cfg.Model = ""
				_ = m.cfg.Save()
				m.status += mutedStyle.Render(" — pick another model to use")
			}
		}
		m.loading = true
		return m, tea.Batch(m.loadModels(), m.spinner.Tick)

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
		switch m.state {
		case mgrBrowse:
			return m.updateBrowse(msg)
		case mgrConfirmDelete:
			if msg.String() == "y" {
				name := m.selected().name
				m.state = mgrBrowse
				m.status = mutedStyle.Render("deleting " + name + "…")
				return m, func() tea.Msg {
					return deletedMsg{name, m.client.Delete(context.Background(), name)}
				}
			}
			m.state = mgrBrowse
			m.status = ""
			return m, nil
		case mgrInput:
			switch msg.String() {
			case "enter":
				name := strings.TrimSpace(m.input.Value())
				m.input.Blur()
				if name == "" {
					m.state = mgrBrowse
					return m, nil
				}
				return m.startPull(name)
			case "esc":
				m.input.Blur()
				m.state = mgrBrowse
				return m, nil
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		case mgrPulling:
			if msg.String() == "esc" || msg.String() == "q" {
				if m.cancel != nil {
					m.cancel()
				}
			}
			return m, nil
		}
	}
	return m, nil
}

func (m managerModel) updateBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case "enter", " ":
		it := m.selected()
		if it == nil || !m.reachOK {
			return m, nil
		}
		if it.installed {
			m.use(it.name)
			return m, nil
		}
		return m.startPull(it.name)
	case "p":
		if !m.reachOK {
			return m, nil
		}
		m.state = mgrInput
		m.input.SetValue("")
		m.status = ""
		return m, m.input.Focus()
	case "d", "x":
		if it := m.selected(); it != nil && it.installed {
			m.state = mgrConfirmDelete
			m.status = warnStyle.Render(fmt.Sprintf("Delete %s? ", it.name)) + helpLine("y", "yes", "any key", "no")
		}
	case "r":
		m.loading = true
		m.status = ""
		return m, tea.Batch(m.loadModels(), m.spinner.Tick)
	}
	return m, nil
}

func (m managerModel) View() string {
	var b strings.Builder
	b.WriteString("\n " + titleStyle.Render("help") + mutedStyle.Render(" · model manager") + "\n")
	active := m.cfg.Model
	if active == "" {
		active = "none"
	}
	b.WriteString(" " + mutedStyle.Render("ollama "+m.client.Host+"  ·  in use: ") + commandStyle.Render(active) + "\n")

	if m.loading && len(m.items) == 0 {
		b.WriteString("\n " + m.spinner.View() + mutedStyle.Render(" loading models…") + "\n")
	}

	nameW := 22
	for _, it := range m.items {
		nameW = max(nameW, len(it.name)+2)
	}

	lastInstalled := -1
	for i, it := range m.items {
		if i == 0 && it.installed {
			b.WriteString(sectionStyle.Render(" Installed") + "\n")
		}
		if !it.installed && lastInstalled == i-1 {
			b.WriteString(sectionStyle.Render(" Download") + mutedStyle.Render("  (recommended small models)") + "\n")
		}
		if it.installed {
			lastInstalled = i
		}

		pointer := "  "
		if i == m.cursor {
			pointer = promptStyle.Render("▸ ")
		}
		mark := "  "
		if it.installed && ollama.SameModel(it.name, m.cfg.Model) {
			mark = successStyle.Render("● ")
		}
		name := fmt.Sprintf("%-*s", nameW, it.name)
		if i == m.cursor {
			name = selectedRow.Render(name)
		}
		var meta string
		if it.installed {
			meta = fmt.Sprintf("%-8s %8s", it.params, it.size)
		} else {
			meta = fmt.Sprintf("%-8s %8s  %s", "", it.size, it.desc)
		}
		b.WriteString(" " + pointer + mark + name + mutedStyle.Render(meta) + "\n")
	}

	b.WriteString("\n")
	switch m.state {
	case mgrPulling:
		b.WriteString(" " + m.spinner.View() + " Downloading " + commandStyle.Render(m.pullName) + mutedStyle.Render("  "+m.pullStat.Status) + "\n")
		if m.pullStat.Total > 0 {
			pct := float64(m.pullStat.Completed) / float64(m.pullStat.Total)
			b.WriteString(" " + m.bar.ViewAs(pct) + mutedStyle.Render(fmt.Sprintf("  %s / %s", HumanSize(m.pullStat.Completed), HumanSize(m.pullStat.Total))) + "\n")
		}
		b.WriteString("\n " + helpLine("esc", "cancel download") + "\n")
	case mgrInput:
		b.WriteString(" " + m.input.View() + "\n")
		b.WriteString(" " + mutedStyle.Render("any model from ollama.com/library") + "\n\n")
		b.WriteString(" " + helpLine("enter", "download", "esc", "back") + "\n")
	default:
		if m.status != "" {
			b.WriteString(" " + m.status + "\n\n")
		}
		action := "use"
		if it := m.selected(); it != nil && !it.installed {
			action = "download"
		}
		b.WriteString(" " + helpLine("↑/↓", "move", "enter", action, "p", "pull by name", "d", "delete", "r", "refresh", "q", "quit") + "\n")
	}
	return b.String()
}
