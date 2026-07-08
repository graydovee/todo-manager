package config

import (
	"os"
	"path/filepath"
	"strings"
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

// TestLoadCorruptReturnsDefault reproduces the real-world crash: a power loss
// mid-write left the config as a block of NUL bytes, which previously made the
// app exit on startup. Load must fall back to defaults instead.
func TestLoadCorruptReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".todo-manager", "gui-config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// 264 bytes of NUL — exactly what the corrupted on-disk file looked like.
	nul := make([]byte, 264)
	if err := os.WriteFile(configPath, nul, 0o600); err != nil {
		t.Fatalf("seed corrupt config: %v", err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load of corrupt config returned err: %v (want default fallback)", err)
	}
	if cfg.BaseURL != DefaultBaseURL {
		t.Errorf("BaseURL = %q, want default %q", cfg.BaseURL, DefaultBaseURL)
	}
	if cfg.APIKey != "" {
		t.Errorf("APIKey = %q, want empty", cfg.APIKey)
	}

	// The corrupt content must have been quarantined to a .corrupt sidecar so the
	// user can recover it, and the live file must no longer be all NUL.
	backup, err := os.ReadFile(configPath + ".corrupt")
	if err != nil {
		t.Fatalf("expected quarantined backup: %v", err)
	}
	if len(backup) != 264 {
		t.Errorf("backup size = %d, want 264", len(backup))
	}
}

// TestWriteIsAtomic ensures that after Write succeeds the on-disk file is a
// complete, valid YAML document with no leftover temp files in the directory.
func TestWriteIsAtomic(t *testing.T) {
	dir := t.TempDir()
	cfg := Default()
	cfg.APIKey = "tdk_atomic"

	if err := Write(dir, cfg); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// No leftover temp files should remain next to the config.
	entries, err := os.ReadDir(filepath.Join(dir, ".todo-manager"))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}

	// The written file must parse cleanly (round-trips).
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load after Write: %v", err)
	}
	if loaded.APIKey != "tdk_atomic" {
		t.Errorf("APIKey = %q, want tdk_atomic", loaded.APIKey)
	}
}
