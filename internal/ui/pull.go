package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ProductionPanic/how/internal/ollama"
)

type pullModel struct {
	name    string
	ch      chan tea.Msg
	cancel  context.CancelFunc
	spinner spinner.Model
	bar     progress.Model
	stat    ollama.PullProgress
	err     error
	done    bool
}

// RunPull downloads a model with a spinner and progress bar.
// Esc or ctrl+c cancels and returns context.Canceled.
func RunPull(client *ollama.Client, name string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = promptStyle

	m := pullModel{
		name:    name,
		ch:      startPull(ctx, client, name),
		cancel:  cancel,
		spinner: sp,
		bar:     progress.New(progress.WithDefaultGradient(), progress.WithWidth(40)),
		stat:    ollama.PullProgress{Status: "starting"},
	}
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return err
	}
	return final.(pullModel).err
}

// startPull runs the download in the background and streams
// pullMsg updates followed by a single pullDoneMsg.
func startPull(ctx context.Context, client *ollama.Client, name string) chan tea.Msg {
	ch := make(chan tea.Msg, 64)
	go func() {
		err := client.Pull(ctx, name, func(p ollama.PullProgress) {
			select {
			case ch <- pullMsg(p):
			default: // drop updates if the UI is behind
			}
		})
		ch <- pullDoneMsg{name, err}
		close(ch)
	}()
	return ch
}

func (m pullModel) Init() tea.Cmd {
	return tea.Batch(waitFor(m.ch), m.spinner.Tick)
}

func (m pullModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case pullMsg:
		m.stat = ollama.PullProgress(msg)
		return m, waitFor(m.ch)
	case pullDoneMsg:
		m.err = msg.err
		m.done = true
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c", "q":
			m.cancel() // the pull goroutine then sends pullDoneMsg
		}
	}
	return m, nil
}

func (m pullModel) View() string {
	if m.done {
		if m.err == nil {
			return successStyle.Render("✓ downloaded "+m.name) + "\n"
		}
		return ""
	}
	var b strings.Builder
	b.WriteString(m.spinner.View() + " Downloading " + commandStyle.Render(m.name) + mutedStyle.Render("  "+m.stat.Status) + "\n")
	if m.stat.Total > 0 {
		pct := float64(m.stat.Completed) / float64(m.stat.Total)
		b.WriteString(m.bar.ViewAs(pct) + mutedStyle.Render(fmt.Sprintf("  %s / %s", HumanSize(m.stat.Completed), HumanSize(m.stat.Total))) + "\n")
	}
	b.WriteString(helpLine("esc", "cancel") + "\n")
	return b.String()
}
