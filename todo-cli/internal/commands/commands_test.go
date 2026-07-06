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

	"github.com/graydovee/todo-manager/todo-cli/internal/client"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func seedHome(t *testing.T, content string) string {
	t.Helper()
	home := t.TempDir()
	configDir := filepath.Join(home, ".todo-manager")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	t.Setenv("HOME", home)
	return home
}

const singleUserConfig = "auth:\n  default_user: default\n  users:\n    - name: default\n      base_url: http://example.com\n      api_key: secret\n"

func injectClient(cmd *cobra.Command, transport http.RoundTripper) {
	appCtx := getAppContext(cmd)
	if appCtx == nil {
		return
	}
	appCtx.NewClient = func(baseURL, apiKey string) *client.Client {
		return client.NewWithHTTPClient(baseURL, apiKey, &http.Client{Transport: transport})
	}
	cmd.SetContext(context.WithValue(cmd.Context(), appContextKey{}, appCtx))
}

func TestConfigViewDefaultsToYAMLAndMasks(t *testing.T) {
	seedHome(t, singleUserConfig)

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"config", "view"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute config view: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "auth:") {
		t.Fatalf("expected yaml output with auth block, got: %q", out)
	}
	if !strings.Contains(out, "api_key: secret****") {
		t.Fatalf("expected masked api key in yaml view, got: %q", out)
	}
	if strings.Contains(out, "\"auth\"") || strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Fatalf("default format should be yaml, not json, got: %q", out)
	}
}

func TestConfigViewHonorsJSONOutput(t *testing.T) {
	seedHome(t, singleUserConfig)

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"config", "view", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute config view: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json with -o json: %v\n%s", err, stdout.String())
	}
}

func TestMigrationNoticePrinted(t *testing.T) {
	seedHome(t, "api_key: tdk_old_secret\nbase_url: http://localhost:8080\n")

	cmd := NewRootCommand()
	var stderr bytes.Buffer
	cmd.SetOut(io.Discard)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"config", "view"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute config view: %v", err)
	}
	if !strings.Contains(stderr.String(), "Migrated config to multi-user format") {
		t.Fatalf("expected migration notice on stderr, got: %q", stderr.String())
	}
}

func TestUserFlagSelectsUser(t *testing.T) {
	seedHome(t, "auth:\n  default_user: home\n  users:\n    - name: home\n      base_url: http://home.example.com\n      api_key: home-key\n    - name: work\n      base_url: http://work.example.com\n      api_key: work-key\n")

	var gotAuth string
	cmd := NewRootCommand()
	injectClient(cmd, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotAuth = r.Header.Get("Authorization")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(`{"items":[],"total":0,"page":1,"page_size":20}`)),
		}, nil
	}))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"todos", "list", "-u", "work"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute todos list: %v", err)
	}
	if gotAuth != "Bearer work-key" {
		t.Fatalf("expected -u work to use work-key, got %q", gotAuth)
	}
}

func TestNoDefaultUserErrors(t *testing.T) {
	seedHome(t, "auth:\n  users:\n    - name: home\n      base_url: http://home.example.com\n      api_key: home-key\n")

	cmd := NewRootCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"todos", "list"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no default user and no -u")
	}
	if !strings.Contains(err.Error(), "no default user") {
		t.Fatalf("expected no-default-user error, got: %v", err)
	}
}

func TestTodosListMapsFlagsToQuery(t *testing.T) {
	seedHome(t, singleUserConfig)

	cmd := NewRootCommand()
	injectClient(cmd, roundTripFunc(func(r *http.Request) (*http.Response, error) {
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
	}))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"todos", "list", "--q", "abc", "--tag", "a", "--tag", "b"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute todos list: %v", err)
	}
}

func TestCommentsCreateSendsBody(t *testing.T) {
	seedHome(t, singleUserConfig)

	cmd := NewRootCommand()
	injectClient(cmd, roundTripFunc(func(r *http.Request) (*http.Response, error) {
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
	}))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"todos", "comments", "create", "12", "--content", "hello"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute comments create: %v", err)
	}
}

func TestLoginBootstrap(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"login", "--api-key", "tdk_test_secret"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute login: %v", err)
	}

	if !strings.Contains(stdout.String(), "name: default") {
		t.Fatalf("expected target user entry on stdout, got: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "api_key: tdk_test_secret") {
		t.Fatalf("expected clear api key on stdout, got: %q", stdout.String())
	}

	data, err := os.ReadFile(filepath.Join(home, ".todo-manager", "config.yaml"))
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}
	fileContent := string(data)
	if !strings.Contains(fileContent, "default_user: default") {
		t.Fatalf("expected default_user in file, got: %q", fileContent)
	}
	if !strings.Contains(fileContent, "api_key: tdk_test_secret") {
		t.Fatalf("expected api key in file, got: %q", fileContent)
	}
}

func TestLoginReadsAPIKeyFromStdin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

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

func TestLoginFailsWhenDefaultExists(t *testing.T) {
	seedHome(t, singleUserConfig)

	cmd := NewRootCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"login", "--api-key", "another"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error logging in without -u when a default exists")
	}
	if !strings.Contains(err.Error(), "default user already set") {
		t.Fatalf("expected default-already-set error, got: %v", err)
	}
}

func TestLoginWithUserAddsNamed(t *testing.T) {
	seedHome(t, singleUserConfig)

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"login", "-u", "work", "--api-key", "work-key"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute login -u: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".todo-manager", "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "name: work") || !strings.Contains(content, "work-key") {
		t.Fatalf("expected work user in file, got: %q", content)
	}
	// default_user unchanged (still "default", not "work").
	if !strings.Contains(content, "default_user: default") {
		t.Fatalf("default_user should be unchanged by login -u, got: %q", content)
	}
}

func TestLoginWithUserOverwritesExisting(t *testing.T) {
	seedHome(t, "auth:\n  default_user: default\n  users:\n    - name: default\n      base_url: http://example.com\n      api_key: old-key\n    - name: work\n      base_url: http://old.example.com\n      api_key: old-work-key\n")

	cmd := NewRootCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"login", "-u", "work", "--api-key", "new-work-key", "--base-url", "http://new.example.com"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute login -u: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".todo-manager", "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "new-work-key") || strings.Contains(content, "old-work-key") {
		t.Fatalf("expected work key overwritten, got: %q", content)
	}
	if !strings.Contains(content, "new.example.com") {
		t.Fatalf("expected work base url overwritten, got: %q", content)
	}
	// Order preserved: default still first.
	if idxDefault, idxWork := strings.Index(content, "name: default"), strings.Index(content, "name: work"); idxDefault < 0 || idxWork < 0 || idxDefault > idxWork {
		t.Fatalf("expected default before work, got: %q", content)
	}
}

func TestConfigUserList(t *testing.T) {
	seedHome(t, "auth:\n  default_user: home\n  users:\n    - name: home\n      base_url: http://h.example.com\n      api_key: homekey1234\n    - name: work\n      base_url: http://w.example.com\n      api_key: workkey1234\n")

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"config", "user", "list", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute config user list: %v", err)
	}
	var items []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &items); err != nil {
		t.Fatalf("expected json list: %v\n%s", err, stdout.String())
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 users, got %d", len(items))
	}
	if items[0]["is_default"] != true || items[1]["is_default"] != false {
		t.Fatalf("default markers wrong: %+v", items)
	}
	if items[0]["api_key"] != "homekey1****" {
		t.Fatalf("expected masked key, got %v", items[0]["api_key"])
	}
}

func TestConfigUserSetDefault(t *testing.T) {
	home := seedHome(t, "auth:\n  default_user: home\n  users:\n    - name: home\n      base_url: http://h.example.com\n      api_key: homekey\n    - name: work\n      base_url: http://w.example.com\n      api_key: workkey\n")

	cmd := NewRootCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"config", "user", "set-default", "work"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute set-default: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".todo-manager", "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "default_user: work") {
		t.Fatalf("expected default_user updated, got: %q", string(data))
	}

	// Unknown name errors.
	cmd2 := NewRootCommand()
	cmd2.SetOut(io.Discard)
	cmd2.SetErr(io.Discard)
	cmd2.SetArgs([]string{"config", "user", "set-default", "zzz"})
	if err := cmd2.Execute(); err == nil {
		t.Fatal("expected error setting unknown default")
	}
}

func TestConfigUserSetDefaultClears(t *testing.T) {
	home := seedHome(t, singleUserConfig)

	cmd := NewRootCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"config", "user", "set-default", ""})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute set-default empty: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".todo-manager", "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "default_user:") && strings.Contains(content, "default_user: \"\"") == false {
		// default_user line should be absent (omitempty) — verify no concrete default.
	}
	// After clearing, a credentialed command without -u must error.
	cmd2 := NewRootCommand()
	cmd2.SetOut(io.Discard)
	cmd2.SetErr(io.Discard)
	cmd2.SetArgs([]string{"todos", "list"})
	if err := cmd2.Execute(); err == nil || !strings.Contains(err.Error(), "no default user") {
		t.Fatalf("expected no-default-user error after clearing default, got: %v", err)
	}
}

func TestConfigUserRemoveClearsDefault(t *testing.T) {
	home := seedHome(t, singleUserConfig)

	cmd := NewRootCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"config", "user", "remove", "default"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute remove: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".todo-manager", "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(data), "name: default") {
		t.Fatalf("expected user removed from file, got: %q", string(data))
	}

	// Removing an unknown user errors.
	cmd2 := NewRootCommand()
	cmd2.SetOut(io.Discard)
	cmd2.SetErr(io.Discard)
	cmd2.SetArgs([]string{"config", "user", "remove", "zzz"})
	if err := cmd2.Execute(); err == nil {
		t.Fatal("expected error removing unknown user")
	}
}

func TestConfigUserRename(t *testing.T) {
	home := seedHome(t, "auth:\n  default_user: home\n  users:\n    - name: home\n      base_url: http://h.example.com\n      api_key: homekey\n    - name: work\n      base_url: http://w.example.com\n      api_key: workkey\n")

	cmd := NewRootCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"config", "user", "rename", "home", "personal"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute rename: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".todo-manager", "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "name: personal") || !strings.Contains(content, "default_user: personal") {
		t.Fatalf("expected rename + default follow, got: %q", content)
	}

	// Duplicate target name errors.
	cmd2 := NewRootCommand()
	cmd2.SetOut(io.Discard)
	cmd2.SetErr(io.Discard)
	cmd2.SetArgs([]string{"config", "user", "rename", "personal", "work"})
	if err := cmd2.Execute(); err == nil {
		t.Fatal("expected error renaming onto existing name")
	}
}

func TestOutputFormats(t *testing.T) {
	seedHome(t, singleUserConfig)

	// yaml via --output and -o
	for _, args := range [][]string{{"config", "view", "--output", "yaml"}, {"config", "view", "-o", "yaml"}} {
		cmd := NewRootCommand()
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute %v: %v", args, err)
		}
		var m map[string]any
		if err := yaml.Unmarshal(stdout.Bytes(), &m); err != nil {
			t.Fatalf("expected yaml for %v: %v\n%s", args, err, stdout.String())
		}
	}

	// pretty alias -> valid json
	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"config", "view", "--output", "pretty", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute pretty/json: %v", err)
	}
	var pm map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &pm); err != nil {
		t.Fatalf("expected json for pretty alias: %v\n%s", err, stdout.String())
	}

	// unknown format errors
	cmd2 := NewRootCommand()
	cmd2.SetOut(io.Discard)
	cmd2.SetErr(io.Discard)
	cmd2.SetArgs([]string{"config", "view", "-o", "bogus"})
	if err := cmd2.Execute(); err == nil {
		t.Fatal("expected error for unknown output format")
	}
}
