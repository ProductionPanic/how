// Command how-eval runs a fixed set of questions against one or more Ollama
// models and reports how well each one does, so models can be compared.
//
//	go run ./cmd/how-eval                       # all installed models
//	go run ./cmd/how-eval -m qwen2.5-coder:3b,llama3.2:3b -v
//
// Every case runs in a temporary folder with a few known files, so the
// directory listing in the prompt is the same for every model. A case can
// have follow-up steps; each follow-up is sent with the model's own earlier
// answers as history, the same way the suggestion view does it.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ProductionPanic/how/internal/config"
	"github.com/ProductionPanic/how/internal/ollama"
	"github.com/ProductionPanic/how/internal/suggest"
)

type step struct {
	q      string
	want   []string // regexes that must all match the command
	not    []string // regexes that must not match
	danger *bool    // expected "dangerous" flag, nil = not checked
}

type testCase struct {
	name  string
	steps []step
}

var (
	yes = ptr(true)
	no  = ptr(false)
)

func ptr(b bool) *bool { return &b }

var cases = []testCase{
	{"tail log", []step{{q: "show the last 50 lines of the log and keep following it",
		want: []string{`\btail\b`, `-n ?50|-50\b`, `-[a-zA-Z]*f|-F`, `app\.log`}, danger: no}}},
	{"free disk", []step{{q: "how much disk space is free", want: []string{`\bdf\b`}, danger: no}}},
	{"big files", []step{{q: "find files bigger than 100MB in this folder",
		want: []string{`\bfind\b`, `-size \+100M`}, not: []string{`-delete`, `\brm\b`}, danger: no}}},
	{"delete tmp", []step{{q: "delete all .tmp files in this folder and below",
		want: []string{`\.tmp`, `-delete|\brm\b`}, danger: yes}}},
	{"port 8080", []step{{q: "what process is listening on port 8080",
		want: []string{`\blsof\b|\bnetstat\b`, `8080`}, not: []string{`\bkill\b`}, danger: no}}},
	{"untar", []step{{q: "extract the backup archive",
		want: []string{`\btar\b`, `x`, `backup\.tar\.gz`}}}},
	{"sed macOS", []step{{q: "replace foo with bar in notes.txt",
		want: []string{`sed -i ?(''|""|\.bak|'\.bak')|perl -pi`, `notes\.txt`}}}},
	{"new branch", []step{{q: "make a new git branch and switch to it",
		want: []string{`git (checkout -b|switch -c|branch)`, `<[a-zA-Z][\w-]*>`}, danger: no}}},
	{"force push", []step{{q: "force push my current branch",
		want: []string{`git push`, `--force|-f\b`}, danger: yes}}},
	{"count go lines", []step{{q: "count the lines in all go files under this folder",
		want: []string{`\.go`, `wc -l`}, danger: no}}},
	{"public ip", []step{{q: "what is my public ip address",
		want: []string{`\bcurl\b|\bdig\b|\bwget\b`}, danger: no}}},
	{"follow-up: wp export", []step{
		{q: "how do i export this wordpress db", want: []string{`wp db export|mysqldump`}},
		{q: "with the wp cli", want: []string{`wp db export`}, not: []string{`mysqldump`}},
		{q: "and gzip it", want: []string{`wp db export`, `gzip`}},
	}},
	{"follow-up: ls", []step{
		{q: "list the files in this folder", want: []string{`\bls\b`}, danger: no},
		{q: "sorted by size", want: []string{`\bls\b.*-[a-zA-Z]*S|sort`}, danger: no},
		{q: "only the 3 biggest", want: []string{`head -(n ?)?3\b|head -3\b`}, danger: no},
	}},
	{"follow-up: find then delete", []step{
		{q: "find all .log files here", want: []string{`\bfind\b`, `\.log`}, not: []string{`-delete`, `\brm\b`}, danger: no},
		{q: "and delete them", want: []string{`\.log`, `-delete|\brm\b`}, danger: yes},
	}},
}

// fixture creates the folder the cases run in.
func fixture() (string, error) {
	dir, err := os.MkdirTemp("", "how-eval-")
	if err != nil {
		return "", err
	}
	for _, f := range []string{"app.log", "notes.txt", "backup.tar.gz", "wp-config.php", "index.php", "main.go"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x\n"), 0o644); err != nil {
			return "", err
		}
	}
	for _, d := range []string{"wp-content", "src"} {
		if err := os.Mkdir(filepath.Join(dir, d), 0o755); err != nil {
			return "", err
		}
	}
	return dir, nil
}

// check returns the reasons a step failed, or nil when it passed.
func check(s step, got suggest.Suggestion) []string {
	var fails []string
	for _, re := range s.want {
		if !regexp.MustCompile(re).MatchString(got.Command) {
			fails = append(fails, "missing /"+re+"/")
		}
	}
	for _, re := range s.not {
		if regexp.MustCompile(re).MatchString(got.Command) {
			fails = append(fails, "should not match /"+re+"/")
		}
	}
	if s.danger != nil && got.Dangerous != *s.danger {
		fails = append(fails, fmt.Sprintf("dangerous=%v, want %v", got.Dangerous, *s.danger))
	}
	return fails
}

type score struct {
	model                   string
	steps, passed           int
	followSteps, followPass int
	dangerChecks, dangerOK  int
	errors                  int
	total                   time.Duration
}

func run(ctx context.Context, c *ollama.Client, model string, verbose bool) score {
	sc := score{model: model}
	// Warm up so loading the model does not count as answer time.
	_, _ = suggest.Ask(ctx, c, model, nil, "print the current directory")

	for _, tc := range cases {
		var history []suggest.Turn
		for i, s := range tc.steps {
			start := time.Now()
			got, err := suggest.Ask(ctx, c, model, history, s.q)
			sc.total += time.Since(start)
			sc.steps++
			if i > 0 {
				sc.followSteps++
			}
			var fails []string
			if err != nil {
				sc.errors++
				fails = []string{err.Error()}
			} else {
				fails = check(s, got)
				history = append(history, suggest.Turn{Question: s.q, Answer: got})
				if s.danger != nil {
					sc.dangerChecks++
					if got.Dangerous == *s.danger {
						sc.dangerOK++
					}
				}
			}
			mark := "✓"
			if len(fails) == 0 {
				sc.passed++
				if i > 0 {
					sc.followPass++
				}
			} else {
				mark = "✗"
			}
			if verbose || len(fails) > 0 {
				label := tc.name
				if len(tc.steps) > 1 {
					label = fmt.Sprintf("%s [%d] %q", tc.name, i+1, s.q)
				}
				fmt.Printf("  %s %-50s %s\n", mark, label, got.Command)
				for _, f := range fails {
					fmt.Printf("      %s\n", f)
				}
			}
			if err != nil {
				break // the rest of the chain has no history to build on
			}
		}
	}
	return sc
}

func pct(a, b int) string {
	if b == 0 {
		return "-"
	}
	return fmt.Sprintf("%d/%d (%d%%)", a, b, a*100/b)
}

func main() {
	models := flag.String("m", "", "comma-separated models (default: all installed)")
	verbose := flag.Bool("v", false, "show passing steps too")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	c := ollama.New(cfg.OllamaHost())
	ctx := context.Background()

	var names []string
	if *models != "" {
		names = strings.Split(*models, ",")
	} else {
		list, err := c.List(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		for _, m := range list {
			names = append(names, m.Name)
		}
		sort.Strings(names)
	}

	dir, err := fixture()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)
	if err := os.Chdir(dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var scores []score
	for _, m := range names {
		fmt.Printf("\n== %s\n", m)
		scores = append(scores, run(ctx, c, strings.TrimSpace(m), *verbose))
	}

	sort.SliceStable(scores, func(i, j int) bool { return scores[i].passed > scores[j].passed })
	fmt.Println()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "MODEL\tALL STEPS\tFOLLOW-UPS\tDANGER FLAG\tERRORS\tAVG TIME")
	for _, s := range scores {
		avg := time.Duration(0)
		if s.steps > 0 {
			avg = (s.total / time.Duration(s.steps)).Round(10 * time.Millisecond)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n", s.model, pct(s.passed, s.steps),
			pct(s.followPass, s.followSteps), pct(s.dangerOK, s.dangerChecks), s.errors, avg)
	}
	w.Flush()
}
