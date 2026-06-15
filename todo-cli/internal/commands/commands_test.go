package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/graydovee/todolist/todo-cli/internal/client"
)

func TestConfigViewMasksAPIKey(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".todo-manager")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("api_key: tdk_abcdef_secret\nbase_url: http://localhost:8080\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	oldHome := os.Getenv("HOME")
	t.Setenv("HOME", home)
	defer os.Setenv("HOME", oldHome)

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"config", "view"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute config view: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload["api_key"] != "tdk_abcd****" {
		t.Fatalf("unexpected masked api key: %#v", payload["api_key"])
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestTodosListMapsFlagsToQuery(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.Query().Get("q"); got != "abc" {
			t.Fatalf("unexpected q: %q", got)
		}
		if got := r.URL.Query().Get("tag"); got != "a,b" {
			t.Fatalf("unexpected tag: %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(`{"items":[],"total":0,"page":1,"page_size":20}`)),
		}, nil
	})}

	home := t.TempDir()
	configDir := filepath.Join(home, ".todo-manager")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("api_key: secret\nbase_url: http://example.com\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	oldHome := os.Getenv("HOME")
	t.Setenv("HOME", home)
	defer os.Setenv("HOME", oldHome)

	cmd := NewRootCommand()
	appCtx := getAppContext(cmd)
	if appCtx != nil {
		appCtx.NewClient = func(baseURL, apiKey string) *client.Client {
			return client.NewWithHTTPClient(baseURL, apiKey, httpClient)
		}
		cmd.SetContext(context.WithValue(cmd.Context(), appContextKey{}, appCtx))
	}
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"todos", "list", "--q", "abc", "--tag", "a", "--tag", "b"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute todos list: %v", err)
	}
}

func TestCommentsCreateSendsBody(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/v1/todos/12/comments" {
			t.Fatalf("unexpected path: %q", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if payload["content"] != "hello" {
			t.Fatalf("unexpected content: %#v", payload["content"])
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(`{"id":1,"todo_id":12,"user_id":1,"content":"hello","created_at":"2026-01-01T00:00:00Z"}`)),
		}, nil
	})}

	home := t.TempDir()
	configDir := filepath.Join(home, ".todo-manager")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("api_key: secret\nbase_url: http://example.com\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	oldHome := os.Getenv("HOME")
	t.Setenv("HOME", home)
	defer os.Setenv("HOME", oldHome)

	cmd := NewRootCommand()
	appCtx := getAppContext(cmd)
	if appCtx != nil {
		appCtx.NewClient = func(baseURL, apiKey string) *client.Client {
			return client.NewWithHTTPClient(baseURL, apiKey, httpClient)
		}
		cmd.SetContext(context.WithValue(cmd.Context(), appContextKey{}, appCtx))
	}
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"todos", "comments", "create", "12", "--content", "hello"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute comments create: %v", err)
	}
}

func TestLoginWritesConfigAndPrintsYAML(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Setenv("HOME", home)
	defer os.Setenv("HOME", oldHome)

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"login", "--api-key", "tdk_test_secret"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute login: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "api_key: tdk_test_secret") {
		t.Fatalf("unexpected login stdout: %q", output)
	}
	if !strings.Contains(output, "base_url: https://todo.qaer.io") {
		t.Fatalf("unexpected login stdout base url: %q", output)
	}

	data, err := os.ReadFile(filepath.Join(home, ".todo-manager", "config.yaml"))
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}
	if string(data) != output {
		t.Fatalf("expected written config to match stdout.\nstdout=%q\nfile=%q", output, string(data))
	}
}

func TestLoginReadsAPIKeyFromStdin(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Setenv("HOME", home)
	defer os.Setenv("HOME", oldHome)

	cmd := NewRootCommand()
	cmd.SetIn(bytes.NewBufferString("tdk_from_stdin\n"))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"login"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute login from stdin: %v", err)
	}
	if !strings.Contains(stdout.String(), "api_key: tdk_from_stdin") {
		t.Fatalf("unexpected login output: %q", stdout.String())
	}
}
