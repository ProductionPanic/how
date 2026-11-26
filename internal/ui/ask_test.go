package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ProductionPanic/how/internal/suggest"
)

func resultModel(command string) askModel {
	m := askModel{state: askResult, cancel: func() {}, input: textinput.New()}
	m.sugg = suggest.Suggestion{Command: command}
	return m
}

func press(t *testing.T, m askModel, keys ...string) askModel {
	t.Helper()
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		next, _ := m.Update(msg)
		m = next.(askModel)
	}
	return m
}

func TestRunWithoutPlaceholders(t *testing.T) {
	m := press(t, resultModel("ls -la"), "enter")
	if m.state != askDone || m.result.Command != "ls -la" || m.result.Action != ActionRun {
		t.Fatalf("got state %v result %+v", m.state, m.result)
	}
}

func TestFillPlaceholdersThenRun(t *testing.T) {
	m := press(t, resultModel("cp <file> <dest> && ls <file>"), "enter")
	if m.state != askFilling || len(m.fields) != 2 {
		t.Fatalf("expected form with 2 fields, got state %v, %d fields", m.state, len(m.fields))
	}
	m = press(t, m, "enter") // empty field: stays
	if m.state != askFilling || m.field != 0 {
		t.Fatalf("empty enter should not advance")
	}
	m = press(t, m, "my file", "enter", "out", "enter")
	if m.state != askDone {
		t.Fatalf("expected done, got %v", m.state)
	}
	if want := "cp 'my file' out && ls 'my file'"; m.result.Command != want || m.result.Action != ActionRun {
		t.Fatalf("got %+v, want %q", m.result, want)
	}
}

func TestFormEscGoesBack(t *testing.T) {
	m := press(t, resultModel("tail -f <file>"), "enter", "esc")
	if m.state != askResult {
		t.Fatalf("esc should return to the result, got %v", m.state)
	}
}

func TestEditedCommandWithPlaceholderGoesToForm(t *testing.T) {
	m := press(t, resultModel("ls"), "e", " <dir>", "enter")
	if m.state != askFilling || len(m.names) != 1 || m.names[0] != "dir" {
		t.Fatalf("expected form for <dir>, got state %v names %v", m.state, m.names)
	}
	m = press(t, m, "src", "enter")
	if m.result.Command != "ls src" {
		t.Fatalf("got %q", m.result.Command)
	}
}
