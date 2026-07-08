// Package config persists the desktop client settings to a YAML file separate
// from todo-cli (~/.todo-manager/gui-config.yaml). It stores a single backend
// profile plus window and filter preferences.
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultBaseURL matches the production deployment used by todo-cli.
const DefaultBaseURL = "https://todo.qaer.io"

// Config is everything the desktop client persists.
type Config struct {
	BaseURL  string      `yaml:"base_url,omitempty"`
	APIKey   string      `yaml:"api_key,omitempty"`
	Language string      `yaml:"language,omitempty"` // "en" | "zh"; empty = follow system
	Window   Window      `yaml:"window"`
	Filters  ListFilters `yaml:"filters"`
	Dock     DockSettings `yaml:"dock"`
}

// DockSettings controls the auto-hide animation and timing.
type DockSettings struct {
	AnimMs     int `yaml:"anim_ms,omitempty"`     // slide animation duration (ms); 0 = default 500
	HideDelayMs int `yaml:"hide_delay_ms,omitempty"` // cursor-leave delay before auto-hide (ms); 0 = default 600
}

// Window holds the last window geometry and pin/lock state.
type Window struct {
	Width   int  `yaml:"width"`
	Height  int  `yaml:"height"`
	Locked  bool `yaml:"locked"`
	TopMost bool `yaml:"topmost"`
}

// ListFilters mirrors the backend query parameters the user can tweak.
type ListFilters struct {
	Status    []string `yaml:"status,omitempty"`
	Category  []string `yaml:"category,omitempty"`
	Priority  []string `yaml:"priority,omitempty"`
	Query     string   `yaml:"q,omitempty"`
	Code      string   `yaml:"code,omitempty"`
	SortBy    string   `yaml:"sort_by,omitempty"`
	SortOrder string   `yaml:"sort_order,omitempty"`
}

// Default returns a fresh config with sane initial values.
func Default() *Config {
	return &Config{
		BaseURL: DefaultBaseURL,
		Window: Window{
			Width:   360,
			Height:  560,
			Locked:  false,
			TopMost: false,
		},
		Filters: ListFilters{
			Status:    []string{"open", "in_progress"},
			SortBy:    "created_at",
			SortOrder: "desc",
		},
	}
}

// Path returns the absolute config file location for homeDir.
func Path(homeDir string) string {
	return filepath.Join(homeDir, ".todo-manager", "gui-config.yaml")
}

// Load reads the config from disk, falling back to Default() when missing.
func Load(homeDir string) (*Config, error) {
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve user home directory: %w", err)
		}
	}

	configPath := Path(homeDir)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		// A corrupt config (e.g. all NUL bytes after a crash/power loss during a
		// non-atomic write) must not make the app unlaunchable. Quarantine the
		// bad file (best effort) and fall back to defaults so the user lands on
		// the login page instead of a hard exit.
		quarantine(configPath, data)
		return Default(), nil
	}
	// Ensure slices are non-nil so JSON/query handling is uniform.
	if cfg.Filters.Status == nil {
		cfg.Filters.Status = []string{}
	}
	if cfg.Filters.Category == nil {
		cfg.Filters.Category = []string{}
	}
	if cfg.Filters.Priority == nil {
		cfg.Filters.Priority = []string{}
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if cfg.Window.Width == 0 {
		cfg.Window.Width = 360
	}
	if cfg.Window.Height == 0 {
		cfg.Window.Height = 560
	}
	return cfg, nil
}

// Write persists the config (0600, dir 0700). It writes to a sibling temp file
// and atomically renames it over the real path, so a crash or power loss mid-write
// can never leave a truncated/corrupt config (the previous full content survives).
func Write(homeDir string, cfg *Config) error {
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve user home directory: %w", err)
		}
	}

	data, err := marshal(cfg)
	if err != nil {
		return err
	}

	configPath := Path(homeDir)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	// tmp + rename keeps the on-disk config always-complete. os.Rename is atomic
	// on POSIX and on Windows (MoveFileEx with MOVEFILE_REPLACE_EXISTENCE).
	tmp, err := os.CreateTemp(filepath.Dir(configPath), ".gui-config-*.yaml.tmp")
	if err != nil {
		return fmt.Errorf("create temp config file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write config file: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod config file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close config file: %w", err)
	}
	if err := os.Rename(tmpPath, configPath); err != nil {
		return fmt.Errorf("rename config file: %w", err)
	}
	cleanup = false
	return nil
}

// quarantine saves a copy of the unreadable config alongside the real one so the
// user can inspect/recover it, then truncates the live file so the next write
// starts clean. Failures are ignored: this is best-effort cleanup, never fatal.
func quarantine(configPath string, data []byte) {
	if len(data) == 0 {
		return
	}
	backup := configPath + ".corrupt"
	if werr := os.WriteFile(backup, data, 0o600); werr == nil {
		_ = os.Remove(configPath)
	}
}

func marshal(cfg *Config) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(cfg); err != nil {
		return nil, fmt.Errorf("marshal config yaml: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("marshal config yaml: %w", err)
	}
	return buf.Bytes(), nil
}
