// Command how suggests a shell command for a question using a local Ollama model.
//
//	how do I see my git remote url   ask for a command
//	how                              open the model manager
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/ProductionPanic/how/internal/config"
	"github.com/ProductionPanic/how/internal/ollama"
	"github.com/ProductionPanic/how/internal/server"
	"github.com/ProductionPanic/how/internal/suggest"
	"github.com/ProductionPanic/how/internal/ui"
)

const usage = `how - ask a local Ollama model which command to run

Usage:
  how <question>         suggest a command, e.g. how do I see my git remote url
  how                    open the model manager (download, choose, delete models)
  how -p <question>      print only the command, no TUI (also used when piped)
  how -m <model> ...     use a different model for this question

Config: %s
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "how:", err)
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
			return fmt.Errorf("unknown flag %s (see how --help)", args[0])
		}
	}
	client := ollama.New(cfg.OllamaHost())
	question := strings.TrimSpace(strings.Join(args, " "))

	if question == "" {
		return ui.RunManager(client, cfg)
	}

	if model == "" {
		model, err = pickModel(client, &cfg, !printOnly)
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
// installed one (and remembers it) when none is configured. With no models
// installed it offers to download ui.DefaultModel when interactive is true.
func pickModel(client *ollama.Client, cfg *config.Config, interactive bool) (string, error) {
	models, err := client.List(context.Background())
	if errors.Is(err, ollama.ErrUnreachable) {
		if err := ensureRunning(client, interactive); err != nil {
			return "", err
		}
		models, err = client.List(context.Background())
	}
	if err != nil {
		return "", err
	}
	if len(models) == 0 {
		return offerDefaultModel(client, cfg, interactive)
	}
	if cfg.Model != "" {
		for _, m := range models {
			if ollama.SameModel(m.Name, cfg.Model) {
				return cfg.Model, nil
			}
		}
		return "", fmt.Errorf("model %q is not installed. Run `how` without arguments to pick or download one", cfg.Model)
	}
	cfg.Model = models[0].Name
	if err := cfg.Save(); err == nil {
		fmt.Fprintf(os.Stderr, "No model chosen yet, using %s (change it by running `how`).\n", cfg.Model)
	}
	return cfg.Model, nil
}

// offerDefaultModel asks to download the recommended model and, on yes,
// downloads it and makes it the configured model.
func offerDefaultModel(client *ollama.Client, cfg *config.Config, interactive bool) (string, error) {
	hint := fmt.Errorf("no Ollama models installed. Download the recommended one with `ollama pull %s`, or run `how` to pick one", ui.DefaultModel)
	if !interactive || !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", hint
	}
	if !confirm(fmt.Sprintf("No Ollama models installed yet. Download %s (~%s, recommended)?", ui.DefaultModel, ui.DefaultModelSize)) {
		return "", hint
	}
	if err := ui.RunPull(client, ui.DefaultModel); err != nil {
		if errors.Is(err, context.Canceled) {
			return "", errors.New("download cancelled")
		}
		return "", fmt.Errorf("downloading %s: %w", ui.DefaultModel, err)
	}
	cfg.Model = ui.DefaultModel
	if err := cfg.Save(); err != nil {
		return "", fmt.Errorf("saving config: %w", err)
	}
	return cfg.Model, nil
}

// ensureRunning explains why Ollama can't be reached and, when it is
// installed locally but not running, offers to start it.
func ensureRunning(client *ollama.Client, interactive bool) error {
	if !server.IsLocal(client.Host) {
		return fmt.Errorf("ollama is not reachable at %s. Check that the server is up", client.Host)
	}
	in := server.Find()
	if !in.Found() {
		return fmt.Errorf("ollama isn't installed. %s, then try again", server.InstallHint)
	}
	notRunning := errors.New("ollama isn't running. Start it with `ollama serve` or open the Ollama app")
	if !interactive || !term.IsTerminal(int(os.Stdin.Fd())) {
		return notRunning
	}
	if !confirm("Ollama isn't running. Start it now?") {
		return notRunning
	}
	err := ui.RunSpinner("Starting Ollama…", "Ollama is running", func() error {
		if err := server.Start(in, client.Host); err != nil {
			return err
		}
		return server.WaitReady(client, 30*time.Second)
	})
	switch {
	case errors.Is(err, ui.ErrInterrupted):
		return errors.New("stopped waiting for Ollama")
	case err != nil:
		return fmt.Errorf("could not start Ollama: %w (log: %s)", err, server.LogPath())
	}
	return nil
}

// confirm asks a yes/no question on stderr; enter means yes.
func confirm(question string) bool {
	fmt.Fprint(os.Stderr, question+" [Y/n] ")
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "", "y", "yes":
		return true
	}
	return false
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
