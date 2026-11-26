// Package suggest turns a natural-language question into a shell command.
package suggest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ProductionPanic/how/internal/ollama"
)

type Suggestion struct {
	Command     string `json:"command"`
	Explanation string `json:"explanation"`
	Dangerous   bool   `json:"dangerous"`
}

var schema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"command":     map[string]any{"type": "string"},
		"explanation": map[string]any{"type": "string"},
		"dangerous":   map[string]any{"type": "boolean"},
	},
	"required": []string{"command", "explanation", "dangerous"},
}

func osName() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS (BSD userland: use BSD flags for sed, date, stat, etc.)"
	case "linux":
		return "Linux (GNU userland)"
	default:
		return runtime.GOOS
	}
}

func shellName() string {
	if s := os.Getenv("SHELL"); s != "" {
		return filepath.Base(s)
	}
	return "sh"
}

func systemPrompt() string {
	cwd, _ := os.Getwd()
	files := ""
	if l := dirListing(cwd); l != "" {
		files = "\n- Files in the current directory: " + l
	}
	return fmt.Sprintf(`You are a command-line expert. The user describes something they want to do in their terminal. Reply with exactly one shell command that does it.

Environment:
- OS: %s (%s)
- Shell: %s
- Current directory: %s%s

Rules:
- "command" is a single line that can be pasted and run as-is. Use pipes or && to combine steps. No markdown, no backticks, no leading "$", no comments.
- Prefer tools that are installed by default on this OS. Use the simplest, most common command.
- When the user clearly means one of the files in the current directory, use its name.
- Only use placeholders when the user did not give a value that is required. Write them as <name> with a short descriptive name, like <file>, <logfile> or <branch>.
- "explanation" is one short sentence describing what the command does.
- "dangerous" is true when the command deletes or overwrites data, changes system settings, kills processes, rewrites git history or force-pushes; otherwise false.`,
		osName(), runtime.GOARCH, shellName(), cwd, files)
}

// Few-shot examples help small models stick to the format.
var examples = []ollama.Message{
	{Role: "user", Content: "what is my git remote url"},
	{Role: "assistant", Content: `{"command":"git remote get-url origin","explanation":"Prints the URL of the 'origin' remote.","dangerous":false}`},
	{Role: "user", Content: "delete all .log files in this folder and below"},
	{Role: "assistant", Content: `{"command":"find . -type f -name '*.log' -delete","explanation":"Recursively finds and deletes every .log file under the current directory.","dangerous":true}`},
}

// Ask asks the model for a command.
func Ask(ctx context.Context, c *ollama.Client, model, question string) (Suggestion, error) {
	msgs := []ollama.Message{{Role: "system", Content: systemPrompt()}}
	msgs = append(msgs, examples...)
	msgs = append(msgs, ollama.Message{Role: "user", Content: question})

	out, err := c.Chat(ctx, ollama.ChatRequest{
		Model:    model,
		Messages: msgs,
		Format:   schema,
		Options:  map[string]any{"temperature": 0.1},
	})
	if err != nil {
		return Suggestion{}, err
	}
	var s Suggestion
	if err := json.Unmarshal([]byte(out), &s); err != nil {
		return Suggestion{}, fmt.Errorf("model returned an unexpected answer: %s", strings.TrimSpace(out))
	}
	s.Command = cleanCommand(s.Command)
	if s.Command == "" {
		return Suggestion{}, fmt.Errorf("model did not suggest a command")
	}
	return s, nil
}

func cleanCommand(c string) string {
	c = strings.TrimSpace(c)
	c = strings.Trim(c, "`")
	c = strings.TrimPrefix(c, "$ ")
	return strings.TrimSpace(c)
}
