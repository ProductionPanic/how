package server

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFindNotInstalled(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	binaryPaths = nil
	appPaths = func() []string { return nil }
	if in := Find(); in.Found() {
		t.Fatalf("expected nothing found, got %+v", in)
	}
}

func TestFindBinaryAndApp(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "ollama")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	app := filepath.Join(dir, "Ollama.app")
	if err := os.Mkdir(app, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	appPaths = func() []string { return []string{app} }
	in := Find()
	if in.Binary != bin {
		t.Errorf("Binary = %q, want %q", in.Binary, bin)
	}
	if runtime.GOOS == "darwin" && in.App != app {
		t.Errorf("App = %q, want %q", in.App, app)
	}
}

func TestIsLocal(t *testing.T) {
	for host, want := range map[string]bool{
		"http://localhost:11434":     true,
		"http://127.0.0.1:11999":     true,
		"http://[::1]:11434":         true,
		"http://0.0.0.0:11434":       true,
		"http://gpu-box.lan:11434":   false,
		"https://ollama.example.com": false,
	} {
		if got := IsLocal(host); got != want {
			t.Errorf("IsLocal(%q) = %v, want %v", host, got, want)
		}
	}
}
