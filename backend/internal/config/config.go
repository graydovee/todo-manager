package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server  ServerConfig  `yaml:"server"`
	DB      DBConfig      `yaml:"db"`
	Auth    AuthConfig    `yaml:"auth"`
	Session SessionConfig `yaml:"session"`
	Log     LogConfig     `yaml:"log"`
}

type ServerConfig struct {
	Port        int      `yaml:"port"`
	CORSOrigins []string `yaml:"cors_origins"`
}

type DBConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

type AuthConfig struct {
	Mode  string      `yaml:"mode"`
	Basic BasicConfig `yaml:"basic"`
	OIDC  OIDCConfig  `yaml:"oidc"`
}

type BasicConfig struct {
	Users []BasicUser `yaml:"users"`
}

type BasicUser struct {
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	DisplayName string `yaml:"display_name"`
}

type OIDCConfig struct {
	Issuer       string   `yaml:"issuer"`
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	RedirectURL  string   `yaml:"redirect_url"`
	Scopes       []string `yaml:"scopes"`
}

type SessionConfig struct {
	Secret          string `yaml:"secret"`
	MaxAge          int    `yaml:"max_age"`
	CleanupInterval int    `yaml:"cleanup_interval"`
}

type LogConfig struct {
	Format string `yaml:"format"`
	Level  string `yaml:"level"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	applyDefaults(cfg)
	if err := applyEnvOverrides(cfg); err != nil {
		return nil, fmt.Errorf("apply env overrides: %w", err)
	}
	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.DB.Driver == "" {
		cfg.DB.Driver = "sqlite"
	}
	if cfg.DB.DSN == "" && cfg.DB.Driver == "sqlite" {
		cfg.DB.DSN = "todolist.db"
	}
	if cfg.Auth.Mode == "" {
		cfg.Auth.Mode = "basic"
	}
	if cfg.Session.MaxAge == 0 {
		cfg.Session.MaxAge = 86400
	}
	if cfg.Session.CleanupInterval == 0 {
		cfg.Session.CleanupInterval = 3600
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = "text"
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
}

func applyEnvOverrides(cfg *Config) error {
	if v := os.Getenv("TODOLIST_SERVER_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Server.Port)
	}
	if v := os.Getenv("TODOLIST_DB_DRIVER"); v != "" {
		cfg.DB.Driver = v
	}
	if v := os.Getenv("TODOLIST_DB_DSN"); v != "" {
		cfg.DB.DSN = v
	}
	if v := os.Getenv("TODOLIST_AUTH_MODE"); v != "" {
		cfg.Auth.Mode = v
	}
	if v := os.Getenv("TODOLIST_SESSION_SECRET"); v != "" {
		cfg.Session.Secret = v
	}
	if v := os.Getenv("TODOLIST_LOG_FORMAT"); v != "" {
		cfg.Log.Format = v
	}
	if v := os.Getenv("TODOLIST_LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
	return nil
}

func validate(cfg *Config) error {
	if cfg.DB.Driver != "sqlite" && cfg.DB.Driver != "mysql" && cfg.DB.Driver != "postgres" {
		return fmt.Errorf("invalid db.driver: %q (must be sqlite, mysql, or postgres)", cfg.DB.Driver)
	}
	if cfg.Auth.Mode != "basic" && cfg.Auth.Mode != "oidc" {
		return fmt.Errorf("invalid auth.mode: %q (must be basic or oidc)", cfg.Auth.Mode)
	}
	if cfg.Auth.Mode == "oidc" {
		if cfg.Auth.OIDC.Issuer == "" || cfg.Auth.OIDC.ClientID == "" || cfg.Auth.OIDC.ClientSecret == "" {
			return fmt.Errorf("auth.oidc issuer, client_id, and client_secret are required when auth.mode=oidc")
		}
	}
	if len(strings.TrimSpace(cfg.Session.Secret)) == 0 {
		return fmt.Errorf("session.secret is required")
	}
	return nil
}
