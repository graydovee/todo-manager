package config

import (
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

type Config struct {
	APIKey  string `yaml:"api_key" json:"api_key"`
	BaseURL string `yaml:"base_url" json:"base_url"`
}

type LoaderOptions struct {
	APIKeyOverride  string
	BaseURLOverride string
	HomeDir         string
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

	cfg := &Config{}
	if data, err := os.ReadFile(ConfigPath(homeDir)); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config file: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	if v := strings.TrimSpace(os.Getenv(EnvAPIKey)); v != "" {
		cfg.APIKey = v
	}
	if v := strings.TrimSpace(os.Getenv(EnvBaseURL)); v != "" {
		cfg.BaseURL = v
	}

	if v := strings.TrimSpace(opts.APIKeyOverride); v != "" {
		cfg.APIKey = v
	}
	if v := strings.TrimSpace(opts.BaseURLOverride); v != "" {
		cfg.BaseURL = v
	}

	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	normalizedBaseURL, err := NormalizeBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	cfg.BaseURL = normalizedBaseURL

	return cfg, nil
}

func Validate(cfg *Config) error {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return fmt.Errorf("api_key is required")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return fmt.Errorf("base_url is required")
	}
	if _, err := url.ParseRequestURI(cfg.BaseURL); err != nil {
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
	payload := map[string]string{
		"api_key":  strings.TrimSpace(cfg.APIKey),
		"base_url": strings.TrimSpace(cfg.BaseURL),
	}
	data, err := yaml.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal config yaml: %w", err)
	}
	return data, nil
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
