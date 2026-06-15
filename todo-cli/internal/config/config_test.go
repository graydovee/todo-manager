package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPrefersEnvironmentOverFile(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".todo-manager")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("api_key: file-key\nbase_url: http://file.example.com\n"), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	t.Setenv(EnvAPIKey, "env-key")
	t.Setenv(EnvBaseURL, "http://env.example.com/api/v1")

	cfg, err := Load(LoaderOptions{HomeDir: home})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.APIKey != "env-key" {
		t.Fatalf("expected env api key, got %q", cfg.APIKey)
	}
	if cfg.BaseURL != "http://env.example.com" {
		t.Fatalf("expected normalized env base url, got %q", cfg.BaseURL)
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "strip trailing slash", input: "http://localhost:8080/", want: "http://localhost:8080"},
		{name: "strip api prefix", input: "http://localhost:8080/api/v1", want: "http://localhost:8080"},
		{name: "invalid", input: "localhost", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeBaseURL(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	if err := Validate(&Config{APIKey: "", BaseURL: "http://localhost:8080"}); err == nil {
		t.Fatal("expected error for missing api key")
	}
	if err := Validate(&Config{APIKey: "key", BaseURL: ""}); err == nil {
		t.Fatal("expected error for missing base url")
	}
	if err := Validate(&Config{APIKey: "key", BaseURL: "http://localhost:8080"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadUsesDefaultBaseURLWhenUnset(t *testing.T) {
	cfg, err := Load(LoaderOptions{HomeDir: t.TempDir()})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.BaseURL != DefaultBaseURL {
		t.Fatalf("expected default base url %q, got %q", DefaultBaseURL, cfg.BaseURL)
	}
}

func TestWriteConfig(t *testing.T) {
	home := t.TempDir()
	cfg := &Config{
		APIKey:  "secret",
		BaseURL: DefaultBaseURL,
	}
	if err := Write(home, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}
	data, err := os.ReadFile(ConfigPath(home))
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	content := string(data)
	if content == "" || !containsAll(content, "api_key: secret", "base_url: https://todo.qaer.io") {
		t.Fatalf("unexpected config content: %q", content)
	}
	info, err := os.Stat(ConfigPath(home))
	if err != nil {
		t.Fatalf("stat config file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected mode 0600, got %#o", info.Mode().Perm())
	}
}

func containsAll(content string, values ...string) bool {
	for _, value := range values {
		if !strings.Contains(content, value) {
			return false
		}
	}
	return true
}
