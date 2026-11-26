// Package config loads and saves the help-cli settings file.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const defaultHost = "http://localhost:11434"

// Config is stored as JSON in ~/.config/help-cli/config.json
// (or $XDG_CONFIG_HOME/help-cli/config.json).
type Config struct {
	// Model is the Ollama model used for suggestions, e.g. "qwen2.5-coder:3b".
	Model string `json:"model"`
	// Host is the Ollama server URL. Empty means $OLLAMA_HOST or the default.
	Host string `json:"host,omitempty"`
}

// Path returns the location of the config file.
func Path() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "help-cli", "config.json"), nil
}

// Load reads the config file. A missing file yields an empty config.
func Load() (Config, error) {
	var c Config
	p, err := Path()
	if err != nil {
		return c, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, err
	}
	return c, nil
}

// Save writes the config file, creating its directory when needed.
func (c Config) Save() error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(data, '\n'), 0o644)
}

// OllamaHost resolves the server URL: config, then $OLLAMA_HOST, then default.
func (c Config) OllamaHost() string {
	h := c.Host
	if h == "" {
		h = os.Getenv("OLLAMA_HOST")
	}
	if h == "" {
		return defaultHost
	}
	if !strings.Contains(h, "://") {
		h = "http://" + h
	}
	return strings.TrimRight(h, "/")
}
