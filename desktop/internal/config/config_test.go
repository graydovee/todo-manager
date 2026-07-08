package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BaseURL != DefaultBaseURL {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, DefaultBaseURL)
	}
	if cfg.Window.Width != 360 || cfg.Window.Height != 560 {
		t.Errorf("Window size = %dx%d, want 360x560", cfg.Window.Width, cfg.Window.Height)
	}
	if len(cfg.Filters.Status) != 2 || cfg.Filters.Status[0] != "open" {
		t.Errorf("Status = %v, want [open in_progress]", cfg.Filters.Status)
	}
}

func TestWriteThenLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := Default()
	cfg.BaseURL = "https://example.com"
	cfg.APIKey = "tdk_test"
	cfg.Window.Locked = true
	cfg.Window.TopMost = true
	cfg.Filters.Priority = []string{"p0", "p1"}

	if err := Write(dir, cfg); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// File must exist with restrictive perms.
	info, err := os.Stat(filepath.Join(dir, ".todo-manager", "gui-config.yaml"))
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("file mode = %o, want 0600", mode)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.BaseURL != "https://example.com" {
		t.Errorf("BaseURL = %q", loaded.BaseURL)
	}
	if loaded.APIKey != "tdk_test" {
		t.Errorf("APIKey = %q", loaded.APIKey)
	}
	if !loaded.Window.Locked || !loaded.Window.TopMost {
		t.Errorf("Window flags lost: %+v", loaded.Window)
	}
	if len(loaded.Filters.Priority) != 2 {
		t.Errorf("Priority = %v", loaded.Filters.Priority)
	}
}
