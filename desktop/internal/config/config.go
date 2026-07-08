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

	data, err := os.ReadFile(Path(homeDir))
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
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

// Write persists the config (0600, dir 0700).
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
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil
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
