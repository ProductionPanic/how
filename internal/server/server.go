// Package server finds a local Ollama install and starts it when it is
// installed but not running.
package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/ProductionPanic/how/internal/ollama"
)

// InstallHint tells the user how to install Ollama.
const InstallHint = "Install it with `brew install ollama` or from https://ollama.com/download"

// Install describes where Ollama is installed, if anywhere.
type Install struct {
	Binary string // path to the ollama binary, "" if not found
	App    string // path to Ollama.app (macOS), "" if not found
}

func (i Install) Found() bool { return i.Binary != "" || i.App != "" }

// Search locations besides $PATH; variables so tests can override them.
var (
	binaryPaths = []string{"/usr/local/bin/ollama", "/opt/homebrew/bin/ollama"}
	appPaths    = func() []string {
		home, _ := os.UserHomeDir()
		return []string{"/Applications/Ollama.app", filepath.Join(home, "Applications", "Ollama.app")}
	}
)

// Find looks for the ollama binary on PATH and in common locations,
// and for the macOS app.
func Find() Install {
	var in Install
	if p, err := exec.LookPath("ollama"); err == nil {
		in.Binary = p
	} else {
		for _, p := range binaryPaths {
			if isFile(p) {
				in.Binary = p
				break
			}
		}
	}
	if runtime.GOOS == "darwin" {
		for _, p := range appPaths() {
			if isDir(p) {
				in.App = p
				break
			}
		}
	}
	return in
}

// IsLocal reports whether host (a URL) points at this machine, so it makes
// sense to start Ollama for it.
func IsLocal(host string) bool {
	u, err := url.Parse(host)
	if err != nil {
		return false
	}
	switch u.Hostname() {
	case "", "localhost", "127.0.0.1", "::1", "0.0.0.0":
		return true
	}
	return false
}

func port(host string) string {
	u, err := url.Parse(host)
	if err != nil || u.Port() == "" {
		return "11434"
	}
	return u.Port()
}

// Start launches Ollama in the background. On macOS the app is preferred
// when Ollama should listen on its default port, started hidden as it is at
// login; otherwise `ollama serve` is started detached, listening on host.
func Start(in Install, host string) error {
	if in.App != "" && (port(host) == "11434" || in.Binary == "") {
		return exec.Command("open", "-g", "-a", in.App, "--args", "hidden").Run()
	}
	if in.Binary == "" {
		return errors.New("ollama is not installed")
	}

	u, _ := url.Parse(host)
	listen := net.JoinHostPort(u.Hostname(), port(host))
	if u.Hostname() == "" {
		listen = "127.0.0.1:" + port(host)
	}

	cmd := exec.Command(in.Binary, "serve")
	cmd.Env = append(os.Environ(), "OLLAMA_HOST="+listen)
	if log, err := logFile(); err == nil {
		cmd.Stdout, cmd.Stderr = log, log
		defer log.Close()
	}
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// WaitReady polls the server until it answers or timeout passes.
func WaitReady(c *ollama.Client, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		reqCtx, reqCancel := context.WithTimeout(ctx, 2*time.Second)
		_, err := c.List(reqCtx)
		reqCancel()
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("ollama did not respond at %s within %s", c.Host, timeout)
		case <-time.After(300 * time.Millisecond):
		}
	}
}

// LogPath is where output of a started `ollama serve` goes.
func LogPath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "how", "ollama-serve.log")
}

func logFile() (*os.File, error) {
	p := LogPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
