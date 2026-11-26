// Package ollama is a minimal client for the Ollama HTTP API.
package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	Host string
	http *http.Client
}

func New(host string) *Client {
	return &Client{Host: host, http: &http.Client{}}
}

// ErrUnreachable means the Ollama server could not be contacted.
var ErrUnreachable = errors.New("ollama is not reachable")

type Model struct {
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
	Details    struct {
		ParameterSize     string `json:"parameter_size"`
		QuantizationLevel string `json:"quantization_level"`
	} `json:"details"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
	Format   any       `json:"format,omitempty"`
	Think    *bool     `json:"think,omitempty"`
	Options  any       `json:"options,omitempty"`
}

type PullProgress struct {
	Status    string `json:"status"`
	Digest    string `json:"digest"`
	Total     int64  `json:"total"`
	Completed int64  `json:"completed"`
	Error     string `json:"error"`
}

func (c *Client) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Host+path, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w at %s", ErrUnreachable, c.Host)
	}
	if resp.StatusCode >= 300 {
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(b, &e) == nil && e.Error != "" {
			return nil, errors.New(e.Error)
		}
		return nil, fmt.Errorf("ollama: %s", resp.Status)
	}
	return resp, nil
}

// List returns the locally installed models.
func (c *Client) List(ctx context.Context) ([]Model, error) {
	resp, err := c.do(ctx, http.MethodGet, "/api/tags", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct {
		Models []Model `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Models, nil
}

// Chat sends a non-streaming chat request and returns the reply text.
// Thinking is disabled when the model supports it, to keep replies fast.
func (c *Client) Chat(ctx context.Context, req ChatRequest) (string, error) {
	req.Stream = false
	off := false
	req.Think = &off
	content, err := c.chat(ctx, req)
	if err != nil && strings.Contains(err.Error(), "think") {
		// Model does not support the think flag; retry without it.
		req.Think = nil
		content, err = c.chat(ctx, req)
	}
	return content, err
}

func (c *Client) chat(ctx context.Context, req ChatRequest) (string, error) {
	resp, err := c.do(ctx, http.MethodPost, "/api/chat", req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Message Message `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Message.Content, nil
}

// Pull downloads a model, calling fn for every progress update.
func (c *Client) Pull(ctx context.Context, model string, fn func(PullProgress)) error {
	resp, err := c.do(ctx, http.MethodPost, "/api/pull", map[string]any{"model": model, "stream": true})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var p PullProgress
		if err := json.Unmarshal(sc.Bytes(), &p); err != nil {
			continue
		}
		if p.Error != "" {
			return errors.New(p.Error)
		}
		fn(p)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return sc.Err()
}

// Delete removes a local model.
func (c *Client) Delete(ctx context.Context, model string) error {
	resp, err := c.do(ctx, http.MethodDelete, "/api/delete", map[string]string{"model": model})
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// SameModel reports whether two model names refer to the same model,
// treating a missing tag as ":latest".
func SameModel(a, b string) bool {
	return normalize(a) == normalize(b)
}

func normalize(n string) string {
	n = strings.TrimSpace(strings.ToLower(n))
	if n != "" && !strings.Contains(n, ":") {
		n += ":latest"
	}
	return n
}
