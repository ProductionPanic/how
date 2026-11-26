// Command help suggests a shell command for a question using a local Ollama model.
//
//	help what is my git remote url   ask for a command
//	help                             open the model manager
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/term"

	"github.com/ProductionPanic/help-cli/internal/config"
	"github.com/ProductionPanic/help-cli/internal/ollama"
	"github.com/ProductionPanic/help-cli/internal/suggest"
	"github.com/ProductionPanic/help-cli/internal/ui"
)

const usage = `help - ask a local Ollama model which command to run

Usage:
  help <question>        suggest a command, e.g. help "what is my git remote url"
  help                   open the model manager (download, choose, delete models)
  help -p <question>     print only the command, no TUI (also used when piped)
  help -m <model> ...    use a different model for this question

Config: %s
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "help:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	printOnly := !term.IsTerminal(int(os.Stdout.Fd()))
	model := ""
flags:
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "-h", "--help":
			p, _ := config.Path()
			fmt.Printf(usage, p)
			return nil
		case "-p", "--print":
			printOnly = true
			args = args[1:]
		case "-m", "--model":
			if len(args) < 2 {
				return errors.New("-m needs a model name")
			}
			model, args = args[1], args[2:]
		case "--models":
			args = nil
		case "--":
			args = args[1:]
			break flags
		default:
			return fmt.Errorf("unknown flag %s (see help --help)", args[0])
		}
	}
	client := ollama.New(cfg.OllamaHost())
	question := strings.TrimSpace(strings.Join(args, " "))

	if question == "" {
		return ui.RunManager(client, cfg)
	}

	if model == "" {
		model, err = pickModel(client, &cfg)
		if err != nil {
			return err
		}
	}

	if printOnly {
		s, err := suggest.Ask(context.Background(), client, model, question)
		if err != nil {
			return err
		}
		fmt.Println(s.Command)
		return nil
	}

	res, err := ui.RunAsk(client, model, question)
	if err != nil {
		return err
	}
	if res.Err != nil {
		return res.Err
	}
	if res.Action == ui.ActionRun {
		return execute(res.Command)
	}
	return nil
}

// pickModel returns the configured model, or falls back to the first
// installed one (and remembers it) when none is configured.
func pickModel(client *ollama.Client, cfg *config.Config) (string, error) {
	models, err := client.List(context.Background())
	if err != nil {
		return "", fmt.Errorf("%w\nStart it with `ollama serve` (or open the Ollama app)", err)
	}
	if len(models) == 0 {
		return "", errors.New("no Ollama models installed. Run `help` without arguments to download one")
	}
	if cfg.Model != "" {
		for _, m := range models {
			if ollama.SameModel(m.Name, cfg.Model) {
				return cfg.Model, nil
			}
		}
		return "", fmt.Errorf("model %q is not installed. Run `help` without arguments to pick or download one", cfg.Model)
	}
	cfg.Model = models[0].Name
	if err := cfg.Save(); err == nil {
		fmt.Fprintf(os.Stderr, "No model chosen yet, using %s (change it by running `help`).\n", cfg.Model)
	}
	return cfg.Model, nil
}

// execute runs the command in the user's shell with the terminal attached.
func execute(command string) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	c := exec.Command(shell, "-c", command)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			os.Exit(exit.ExitCode())
		}
		return err
	}
	return nil
}
