package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestIsFilePlaceholder(t *testing.T) {
	yes := []string{"file", "logfile", "FILE_NAME", "path", "src", "dest", "target-dir", "log"}
	no := []string{"branch", "message", "pattern", "port", "name"}
	for _, n := range yes {
		if !isFilePlaceholder(n) {
			t.Errorf("%q should be a file placeholder", n)
		}
	}
	for _, n := range no {
		if isFilePlaceholder(n) {
			t.Errorf("%q should not be a file placeholder", n)
		}
	}
	if !wantsDir("dir") || !wantsDir("target-dir") || wantsDir("file") || wantsDir("dir-or-file") {
		t.Error("wantsDir is wrong")
	}
}

func TestFilterEntries(t *testing.T) {
	entries := []string{".env", "README.md", "app.log", "error.log", "logs/", "src/"}
	cases := []struct {
		query    string
		dirsOnly bool
		want     []string
	}{
		{"", false, []string{"README.md", "app.log", "error.log", "logs/", "src/"}},
		{"log", false, []string{"logs/", "app.log", "error.log"}},
		{"READ", false, []string{"README.md"}},
		{"readme", false, []string{"README.md"}},
		{".", false, []string{".env", "README.md", "app.log", "error.log"}},
		{"", true, []string{"logs/", "src/"}},
		{"nothing", false, nil},
	}
	for _, c := range cases {
		if got := filterEntries(entries, c.query, c.dirsOnly); !reflect.DeepEqual(got, c.want) {
			t.Errorf("filterEntries(%q, %v) = %v, want %v", c.query, c.dirsOnly, got, c.want)
		}
	}
}

func pickerDir(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"app.log", "notes.txt"} {
		os.WriteFile(filepath.Join(dir, n), nil, 0o644)
	}
	os.Mkdir(filepath.Join(dir, "logs"), 0o755)
	t.Chdir(dir)
}

func TestPickFileWithArrows(t *testing.T) {
	pickerDir(t)
	m := press(t, resultModel("tail -f <file>"), "enter", "down", "down", "down", "enter")
	if m.state != askDone || m.result.Command != "tail -f notes.txt" {
		t.Fatalf("got state %v command %q", m.state, m.result.Command)
	}
}

func TestPickAfterFiltering(t *testing.T) {
	pickerDir(t)
	m := press(t, resultModel("tail -f <logfile>"), "enter", "a", "down", "enter")
	if m.result.Command != "tail -f app.log" {
		t.Fatalf("got %q", m.result.Command)
	}
}

func TestTypedPathWinsWithoutPick(t *testing.T) {
	pickerDir(t)
	// "app" matches app.log, but without an arrow press the typed text is used.
	m := press(t, resultModel("cat <file>"), "enter", "/var/log/app", "enter")
	if m.result.Command != "cat /var/log/app" {
		t.Fatalf("got %q", m.result.Command)
	}
}

func TestPickerOnlyForFilePlaceholders(t *testing.T) {
	pickerDir(t)
	m := press(t, resultModel("git checkout <branch> -- <file>"), "enter")
	if m.matches() != nil {
		t.Fatal("branch field should not show the picker")
	}
	// down on a plain field moves to the next field, which has the picker.
	m = press(t, m, "down")
	if m.field != 1 || len(m.matches()) != 3 {
		t.Fatalf("field %d, matches %v", m.field, m.matches())
	}
}
