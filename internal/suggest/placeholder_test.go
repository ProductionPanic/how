package suggest

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPlaceholders(t *testing.T) {
	cases := []struct {
		cmd  string
		want []string
	}{
		{"tail -f <file>", []string{"file"}},
		{"cp <file> <file>.bak", []string{"file"}},
		{"git checkout -b <branch-name> && git push -u origin <branch-name>", []string{"branch-name"}},
		{"grep -r '<pattern>' <dir>", []string{"pattern", "dir"}},
		{"cat <<EOF", nil},
		{"cat <<EOF> out.txt", nil},
		{"diff <(ls a) <(ls b)", nil},
		{"make 2>&1 | less", nil},
		{"sort < input.txt > out.txt", nil},
		{"echo <1>", nil},
		{"ls -la", nil},
	}
	for _, c := range cases {
		if got := Placeholders(c.cmd); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Placeholders(%q) = %v, want %v", c.cmd, got, c.want)
		}
	}
}

func TestFill(t *testing.T) {
	cases := []struct {
		cmd  string
		vals map[string]string
		want string
	}{
		{"tail -f <file>", map[string]string{"file": "app.log"}, "tail -f app.log"},
		{"tail -f <file>", map[string]string{"file": "my log.txt"}, "tail -f 'my log.txt'"},
		{"cp <file> <file>.bak", map[string]string{"file": "a.txt"}, "cp a.txt a.txt.bak"},
		{"cat <file>", map[string]string{"file": "it's"}, `cat 'it'\''s'`},
		{"grep '<pattern>' x", map[string]string{"pattern": "a b's"}, `grep 'a b'\''s' x`},
		{`echo "<msg>"`, map[string]string{"msg": `say "$HOME"`}, `echo "say \"\$HOME\""`},
		{"mv <src> <dest>", map[string]string{"src": "a"}, "mv a <dest>"},
		{"cat <<EOF> <file>", map[string]string{"file": "x", "EOF": "y"}, "cat <<EOF> x"},
	}
	for _, c := range cases {
		if got := Fill(c.cmd, c.vals); got != c.want {
			t.Errorf("Fill(%q) = %q, want %q", c.cmd, got, c.want)
		}
	}
}

func TestQuote(t *testing.T) {
	cases := map[string]string{
		"app.log":        "app.log",
		"~/logs/app.log": "~/logs/app.log",
		"src/main.go":    "src/main.go",
		"":               "''",
		"a b":            "'a b'",
		"$(rm -rf /)":    "'$(rm -rf /)'",
		"*.log":          "'*.log'",
		"a~b":            "'a~b'",
		"feature/x-1":    "feature/x-1",
	}
	for in, want := range cases {
		if got := Quote(in); got != want {
			t.Errorf("Quote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDirListing(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"app.log", ".env", ".DS_Store", "b.txt"} {
		os.WriteFile(filepath.Join(dir, n), nil, 0o644)
	}
	os.Mkdir(filepath.Join(dir, "src"), 0o755)
	os.Mkdir(filepath.Join(dir, ".git"), 0o755)

	if got, want := dirListing(dir), ".env, app.log, b.txt, src/"; got != want {
		t.Errorf("dirListing = %q, want %q", got, want)
	}
	if got := dirListing(filepath.Join(dir, "missing")); got != "" {
		t.Errorf("missing dir: got %q, want empty", got)
	}
}

func TestDirListingTruncates(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < maxListing+7; i++ {
		os.WriteFile(filepath.Join(dir, "f"+string(rune('a'+i/26))+string(rune('a'+i%26))), nil, 0o644)
	}
	got := dirListing(dir)
	if n := strings.Count(got, ", "); n != maxListing { // 50 names + "… and 7 more" = 50 separators
		t.Errorf("got %d separators, want %d: %s", n, maxListing, got)
	}
	if !strings.HasSuffix(got, "… and 7 more") {
		t.Errorf("missing truncation note: %s", got)
	}
}
