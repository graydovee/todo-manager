package config

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	EnvAPIKey      = "TODO_MANAGER_API_KEY"
	EnvBaseURL     = "TODO_MANAGER_BASE_URL"
	DefaultBaseURL = "https://todo.qaer.io"
)

// ErrNoDefaultUser is returned when no -u flag is given and the config has no
// default_user to fall back on.
var ErrNoDefaultUser = errors.New("no default user configured; specify -u <name>")

// Config is the full multi-user CLI configuration. Only the Auth block is
// serialized; the unexported fields track migration state for the current run.
type Config struct {
	Auth              AuthConfig `yaml:"auth" json:"auth"`
	migrated          bool       `yaml:"-" json:"-"`
	migrationWriteErr error      `yaml:"-" json:"-"`
}

type AuthConfig struct {
	DefaultUser string      `yaml:"default_user,omitempty" json:"default_user,omitempty"`
	Users       []UserEntry `yaml:"users" json:"users"`
}

type UserEntry struct {
	Name    string `yaml:"name" json:"name"`
	BaseURL string `yaml:"base_url,omitempty" json:"base_url,omitempty"`
	APIKey  string `yaml:"api_key,omitempty" json:"api_key,omitempty"`
}

// rawConfig captures both the legacy flat format (api_key/base_url at root) and
// the new auth block so Load can detect and migrate old files.
type rawConfig struct {
	APIKey  string      `yaml:"api_key"`
	BaseURL string      `yaml:"base_url"`
	Auth    *AuthConfig `yaml:"auth"`
}

// NewConfig returns a Config with a non-nil (empty) users slice so empty lists
// marshal as `[]` rather than `null`.
func NewConfig() *Config {
	return &Config{Auth: AuthConfig{Users: []UserEntry{}}}
}

type LoaderOptions struct {
	HomeDir string
}

func Load(opts LoaderOptions) (*Config, error) {
	homeDir := opts.HomeDir
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve user home directory: %w", err)
		}
	}

	cfg := NewConfig()
	data, readErr := os.ReadFile(ConfigPath(homeDir))
	switch {
	case readErr == nil:
		var raw rawConfig
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("parse config file: %w", err)
		}
		switch {
		case raw.Auth != nil:
			// New format wins; ignore any legacy flat fields if also present.
			cfg.Auth = *raw.Auth
		case strings.TrimSpace(raw.APIKey) != "" || strings.TrimSpace(raw.BaseURL) != "":
			// Legacy flat format -> migrate to a single "default" user.
			cfg.Auth.DefaultUser = "default"
			baseURL := raw.BaseURL
			if normalized, err := NormalizeBaseURL(raw.BaseURL); err == nil {
				baseURL = normalized
			}
			cfg.Auth.Users = []UserEntry{{
				Name:    "default",
				BaseURL: baseURL,
				APIKey:  raw.APIKey,
			}}
			cfg.migrated = true
			if werr := Write(homeDir, cfg); werr != nil {
				cfg.migrationWriteErr = werr
			}
		}
	case !errors.Is(readErr, os.ErrNotExist):
		return nil, fmt.Errorf("read config file: %w", readErr)
	}

	if cfg.Auth.Users == nil {
		cfg.Auth.Users = []UserEntry{}
	}
	return cfg, nil
}

// Migrated reports whether Load migrated a legacy flat config this run.
func (c *Config) Migrated() bool { return c != nil && c.migrated }

// MigrationWriteErr returns any error encountered while persisting a migrated
// config (best-effort write-back). Nil on success or when no migration ran.
func (c *Config) MigrationWriteErr() error {
	if c == nil {
		return nil
	}
	return c.migrationWriteErr
}

func ValidateUser(u UserEntry) error {
	if strings.TrimSpace(u.APIKey) == "" {
		return fmt.Errorf("api_key is required")
	}
	if strings.TrimSpace(u.BaseURL) == "" {
		return fmt.Errorf("base_url is required")
	}
	if _, err := url.ParseRequestURI(u.BaseURL); err != nil {
		return fmt.Errorf("invalid base_url: %w", err)
	}
	return nil
}

func NormalizeBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid base_url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid base_url: must include scheme and host")
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	parsed.Path = strings.TrimSuffix(parsed.Path, "/api/v1")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimSuffix(parsed.String(), "/"), nil
}

func ConfigPath(homeDir string) string {
	return filepath.Join(homeDir, ".todo-manager", "config.yaml")
}

func MarshalYAML(cfg *Config) ([]byte, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
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

func Write(homeDir string, cfg *Config) error {
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve user home directory: %w", err)
		}
	}

	data, err := MarshalYAML(cfg)
	if err != nil {
		return err
	}

	configPath := ConfigPath(homeDir)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil
}

func MaskAPIKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return value + "****"
	}
	return value[:8] + "****"
}

func (c *Config) findUserIndex(name string) int {
	if c == nil {
		return -1
	}
	for i, u := range c.Auth.Users {
		if u.Name == name {
			return i
		}
	}
	return -1
}

// ResolveUser returns a copy of the effective user's credentials. An empty name
// resolves to DefaultUser; if that is also empty it returns ErrNoDefaultUser.
func (c *Config) ResolveUser(name string) (UserEntry, error) {
	if c == nil {
		return UserEntry{}, ErrNoDefaultUser
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = c.Auth.DefaultUser
	}
	if name == "" {
		return UserEntry{}, ErrNoDefaultUser
	}
	idx := c.findUserIndex(name)
	if idx < 0 {
		return UserEntry{}, fmt.Errorf("user %q not found", name)
	}
	return c.Auth.Users[idx], nil
}

// UpsertUser inserts u, or — when a user with the same Name already exists —
// overwrites its BaseURL and APIKey in place. Slice order is preserved on
// update; new users are appended.
func (c *Config) UpsertUser(u UserEntry) {
	if c == nil {
		return
	}
	u.Name = strings.TrimSpace(u.Name)
	if idx := c.findUserIndex(u.Name); idx >= 0 {
		c.Auth.Users[idx].BaseURL = u.BaseURL
		c.Auth.Users[idx].APIKey = u.APIKey
		return
	}
	c.Auth.Users = append(c.Auth.Users, u)
}

// RemoveUser removes the named user. If it was the default, DefaultUser is
// cleared. It reports whether a user was removed.
func (c *Config) RemoveUser(name string) bool {
	if c == nil {
		return false
	}
	name = strings.TrimSpace(name)
	idx := c.findUserIndex(name)
	if idx < 0 {
		return false
	}
	c.Auth.Users = append(c.Auth.Users[:idx], c.Auth.Users[idx+1:]...)
	if c.Auth.DefaultUser == name {
		c.Auth.DefaultUser = ""
	}
	return true
}

// SetDefault sets DefaultUser to name. An empty name clears the default. A
// non-empty name must already exist in Users.
func (c *Config) SetDefault(name string) error {
	if c == nil {
		return ErrNoDefaultUser
	}
	name = strings.TrimSpace(name)
	if name == "" {
		c.Auth.DefaultUser = ""
		return nil
	}
	if c.findUserIndex(name) < 0 {
		return fmt.Errorf("user %q not found", name)
	}
	c.Auth.DefaultUser = name
	return nil
}

// RenameUser renames a user. The new name must be non-empty and not collide
// with an existing user (unless unchanged). If the renamed user was the
// default, DefaultUser is updated to follow it.
func (c *Config) RenameUser(oldName, newName string) error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	idx := c.findUserIndex(oldName)
	if idx < 0 {
		return fmt.Errorf("user %q not found", oldName)
	}
	if newName == "" {
		return fmt.Errorf("new user name must not be empty")
	}
	if newName != oldName && c.findUserIndex(newName) >= 0 {
		return fmt.Errorf("user %q already exists", newName)
	}
	c.Auth.Users[idx].Name = newName
	if c.Auth.DefaultUser == oldName {
		c.Auth.DefaultUser = newName
	}
	return nil
}

// HasDefault reports whether a non-empty default_user is configured.
func (c *Config) HasDefault() bool {
	return c != nil && strings.TrimSpace(c.Auth.DefaultUser) != ""
}

// configView / authView / userView are masked, display-only projections so that
// `config view` output is struct-ordered and stable.

type configView struct {
	Auth authView `yaml:"auth" json:"auth"`
}

type authView struct {
	DefaultUser string     `yaml:"default_user,omitempty" json:"default_user,omitempty"`
	Users       []userView `yaml:"users" json:"users"`
}

type userView struct {
	Name    string `yaml:"name" json:"name"`
	BaseURL string `yaml:"base_url,omitempty" json:"base_url,omitempty"`
	APIKey  string `yaml:"api_key" json:"api_key"` // masked
}

// View returns a masked projection of the full stored config.
func (c *Config) View() configView {
	v := configView{Auth: authView{Users: []userView{}}}
	if c == nil {
		return v
	}
	v.Auth.DefaultUser = c.Auth.DefaultUser
	for _, u := range c.Auth.Users {
		v.Auth.Users = append(v.Auth.Users, userView{
			Name:    u.Name,
			BaseURL: u.BaseURL,
			APIKey:  MaskAPIKey(u.APIKey),
		})
	}
	return v
}

// UserListItem is one row of `config user list`.
type UserListItem struct {
	Name      string `yaml:"name" json:"name"`
	BaseURL   string `yaml:"base_url,omitempty" json:"base_url,omitempty"`
	APIKey    string `yaml:"api_key" json:"api_key"` // masked
	IsDefault bool   `yaml:"is_default" json:"is_default"`
}

// UserList returns a masked list of users with the default flagged.
func (c *Config) UserList() []UserListItem {
	items := []UserListItem{}
	if c == nil {
		return items
	}
	for _, u := range c.Auth.Users {
		items = append(items, UserListItem{
			Name:      u.Name,
			BaseURL:   u.BaseURL,
			APIKey:    MaskAPIKey(u.APIKey),
			IsDefault: u.Name == c.Auth.DefaultUser,
		})
	}
	return items
}
