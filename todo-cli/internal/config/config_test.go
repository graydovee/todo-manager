package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, home, content string) {
	t.Helper()
	configDir := filepath.Join(home, ".todo-manager")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}
}

func TestLoadReturnsStoredValuesAndIgnoresEnv(t *testing.T) {
	home := t.TempDir()
	writeConfig(t, home, "auth:\n  default_user: aaa\n  users:\n    - name: aaa\n      base_url: http://file.example.com\n      api_key: file-key\n")

	t.Setenv(EnvAPIKey, "env-key")
	t.Setenv(EnvBaseURL, "http://env.example.com")

	cfg, err := Load(LoaderOptions{HomeDir: home})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Migrated() {
		t.Fatalf("new-format config should not be marked migrated")
	}
	if cfg.Auth.DefaultUser != "aaa" {
		t.Fatalf("expected default_user aaa, got %q", cfg.Auth.DefaultUser)
	}
	u, err := cfg.ResolveUser("")
	if err != nil {
		t.Fatalf("resolve user: %v", err)
	}
	if u.APIKey != "file-key" {
		t.Fatalf("expected stored api key (env must not apply in Load), got %q", u.APIKey)
	}
	if u.BaseURL != "http://file.example.com" {
		t.Fatalf("expected stored base url, got %q", u.BaseURL)
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

func TestValidateUser(t *testing.T) {
	if err := ValidateUser(UserEntry{APIKey: "", BaseURL: "http://localhost:8080"}); err == nil {
		t.Fatal("expected error for missing api key")
	}
	if err := ValidateUser(UserEntry{APIKey: "key", BaseURL: ""}); err == nil {
		t.Fatal("expected error for missing base url")
	}
	if err := ValidateUser(UserEntry{APIKey: "key", BaseURL: "http://localhost:8080"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadEmptyHomeReturnsEmptyConfig(t *testing.T) {
	cfg, err := Load(LoaderOptions{HomeDir: t.TempDir()})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Migrated() {
		t.Fatalf("empty home should not migrate")
	}
	if cfg.Auth.DefaultUser != "" {
		t.Fatalf("expected empty default user, got %q", cfg.Auth.DefaultUser)
	}
	if cfg.Auth.Users == nil || len(cfg.Auth.Users) != 0 {
		t.Fatalf("expected non-nil empty users slice, got %v", cfg.Auth.Users)
	}
}

func TestWriteConfig(t *testing.T) {
	home := t.TempDir()
	cfg := NewConfig()
	cfg.Auth.DefaultUser = "default"
	cfg.Auth.Users = []UserEntry{{Name: "default", BaseURL: DefaultBaseURL, APIKey: "secret"}}
	if err := Write(home, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}
	data, err := os.ReadFile(ConfigPath(home))
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	content := string(data)
	if content == "" || !containsAll(content, "auth:", "default_user: default", "api_key: secret", "base_url: https://todo.qaer.io") {
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

func TestLoadMigratesOldFormat(t *testing.T) {
	home := t.TempDir()
	writeConfig(t, home, "api_key: file-key\nbase_url: http://file.example.com/api/v1\n")

	cfg, err := Load(LoaderOptions{HomeDir: home})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.Migrated() {
		t.Fatalf("expected legacy config to be migrated")
	}
	if cfg.Auth.DefaultUser != "default" {
		t.Fatalf("expected default user %q, got %q", "default", cfg.Auth.DefaultUser)
	}
	if cfg.MigrationWriteErr() != nil {
		t.Fatalf("unexpected migration write error: %v", cfg.MigrationWriteErr())
	}
	if len(cfg.Auth.Users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(cfg.Auth.Users))
	}
	u := cfg.Auth.Users[0]
	if u.Name != "default" || u.APIKey != "file-key" || u.BaseURL != "http://file.example.com" {
		t.Fatalf("unexpected migrated user: %+v", u)
	}

	// File on disk is now new format.
	data, err := os.ReadFile(ConfigPath(home))
	if err != nil {
		t.Fatalf("read rewritten config: %v", err)
	}
	if !containsAll(string(data), "auth:", "default_user: default", "api_key: file-key") {
		t.Fatalf("expected rewritten new-format config: %q", string(data))
	}
}

func TestLoadMigrationPreservesValuesNotOverrides(t *testing.T) {
	home := t.TempDir()
	writeConfig(t, home, "api_key: file-key\nbase_url: http://file.example.com\n")

	t.Setenv(EnvAPIKey, "env-key")

	cfg, err := Load(LoaderOptions{HomeDir: home})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.Migrated() {
		t.Fatalf("expected migration")
	}
	// Rewritten file must keep the file key, not the env override.
	data, err := os.ReadFile(ConfigPath(home))
	if err != nil {
		t.Fatalf("read rewritten config: %v", err)
	}
	if !strings.Contains(string(data), "file-key") || strings.Contains(string(data), "env-key") {
		t.Fatalf("env override leaked into migrated file: %q", string(data))
	}
}

func TestLoadMigrationWriteFailureIsSoft(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses file permissions")
	}
	home := t.TempDir()
	configFile := ConfigPath(home)
	writeConfig(t, home, "api_key: file-key\nbase_url: http://file.example.com\n")
	if err := os.Chmod(configFile, 0o400); err != nil {
		t.Fatalf("chmod config file: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(configFile, 0o600) })

	cfg, err := Load(LoaderOptions{HomeDir: home})
	if err != nil {
		t.Fatalf("load should not fail on migration write error: %v", err)
	}
	if !cfg.Migrated() {
		t.Fatalf("expected migration to be detected")
	}
	if cfg.MigrationWriteErr() == nil {
		t.Fatalf("expected a migration write error")
	}
	if cfg.Auth.Users[0].APIKey != "file-key" {
		t.Fatalf("in-memory migrated values should still be present")
	}
}

func TestLoadNewFormat(t *testing.T) {
	home := t.TempDir()
	writeConfig(t, home, "auth:\n  default_user: aaa\n  users:\n    - name: aaa\n      base_url: http://x.example.com\n      api_key: k\n    - name: bbb\n      base_url: http://y.example.com\n      api_key: k2\n")

	cfg, err := Load(LoaderOptions{HomeDir: home})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Migrated() {
		t.Fatalf("new-format config should not migrate")
	}
	if len(cfg.Auth.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(cfg.Auth.Users))
	}
}

func TestLoadIgnoresLegacyFieldsWhenAuthPresent(t *testing.T) {
	home := t.TempDir()
	writeConfig(t, home, "api_key: legacy-key\nbase_url: http://legacy.example.com\nauth:\n  default_user: aaa\n  users:\n    - name: aaa\n      base_url: http://new.example.com\n      api_key: new-key\n")

	cfg, err := Load(LoaderOptions{HomeDir: home})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Migrated() {
		t.Fatalf("auth block present -> no migration")
	}
	u, err := cfg.ResolveUser("")
	if err != nil {
		t.Fatalf("resolve user: %v", err)
	}
	if u.APIKey != "new-key" {
		t.Fatalf("auth block should win over legacy fields; got key %q", u.APIKey)
	}
}

func TestResolveUser(t *testing.T) {
	cfg := NewConfig()
	cfg.Auth.DefaultUser = "aaa"
	cfg.Auth.Users = []UserEntry{{Name: "aaa", APIKey: "k1"}, {Name: "bbb", APIKey: "k2"}}

	if u, err := cfg.ResolveUser(""); err != nil || u.Name != "aaa" {
		t.Fatalf("empty name should resolve to default: %v, %+v", err, u)
	}
	if u, err := cfg.ResolveUser("bbb"); err != nil || u.APIKey != "k2" {
		t.Fatalf("explicit name should resolve: %v, %+v", err, u)
	}
	if _, err := cfg.ResolveUser("zzz"); err == nil {
		t.Fatal("expected error for unknown user")
	}

	cfg.Auth.DefaultUser = ""
	if _, err := cfg.ResolveUser(""); !errors.Is(err, ErrNoDefaultUser) {
		t.Fatalf("expected ErrNoDefaultUser, got %v", err)
	}
}

func TestUpsertUser(t *testing.T) {
	cfg := NewConfig()
	cfg.UpsertUser(UserEntry{Name: "a", APIKey: "1", BaseURL: "http://a"})
	cfg.UpsertUser(UserEntry{Name: "b", APIKey: "2", BaseURL: "http://b"})
	if len(cfg.Auth.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(cfg.Auth.Users))
	}
	// Update existing in place, preserving order.
	cfg.UpsertUser(UserEntry{Name: "a", APIKey: "1-updated", BaseURL: "http://a2"})
	if len(cfg.Auth.Users) != 2 {
		t.Fatalf("upsert should not append on update")
	}
	if cfg.Auth.Users[0].Name != "a" || cfg.Auth.Users[0].APIKey != "1-updated" {
		t.Fatalf("update not applied: %+v", cfg.Auth.Users[0])
	}
}

func TestRemoveUser(t *testing.T) {
	cfg := NewConfig()
	cfg.Auth.DefaultUser = "a"
	cfg.Auth.Users = []UserEntry{{Name: "a"}, {Name: "b"}}

	if !cfg.RemoveUser("a") {
		t.Fatal("expected removal to succeed")
	}
	if cfg.Auth.DefaultUser != "" {
		t.Fatalf("removing the default should clear default_user, got %q", cfg.Auth.DefaultUser)
	}
	if len(cfg.Auth.Users) != 1 || cfg.Auth.Users[0].Name != "b" {
		t.Fatalf("unexpected users after removal: %+v", cfg.Auth.Users)
	}
	if cfg.RemoveUser("zzz") {
		t.Fatal("removing unknown user should return false")
	}
	// Remove the last user.
	if !cfg.RemoveUser("b") {
		t.Fatal("expected removal of b")
	}
	if cfg.Auth.Users == nil || len(cfg.Auth.Users) != 0 {
		t.Fatalf("expected non-nil empty users slice, got %v", cfg.Auth.Users)
	}
}

func TestSetDefault(t *testing.T) {
	cfg := NewConfig()
	cfg.Auth.Users = []UserEntry{{Name: "a"}, {Name: "b"}}
	if err := cfg.SetDefault("a"); err != nil {
		t.Fatalf("set default: %v", err)
	}
	if cfg.Auth.DefaultUser != "a" {
		t.Fatalf("default not set: %q", cfg.Auth.DefaultUser)
	}
	if err := cfg.SetDefault("zzz"); err == nil {
		t.Fatal("expected error setting unknown default")
	}
	if err := cfg.SetDefault(""); err != nil {
		t.Fatalf("clearing default should not error: %v", err)
	}
	if cfg.Auth.DefaultUser != "" {
		t.Fatalf("default should be cleared, got %q", cfg.Auth.DefaultUser)
	}
}

func TestRenameUser(t *testing.T) {
	cfg := NewConfig()
	cfg.Auth.DefaultUser = "a"
	cfg.Auth.Users = []UserEntry{{Name: "a"}, {Name: "b"}}

	if err := cfg.RenameUser("a", "c"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if cfg.Auth.Users[0].Name != "c" {
		t.Fatalf("rename not applied: %+v", cfg.Auth.Users[0])
	}
	if cfg.Auth.DefaultUser != "c" {
		t.Fatalf("default should follow rename, got %q", cfg.Auth.DefaultUser)
	}
	if err := cfg.RenameUser("zzz", "y"); err == nil {
		t.Fatal("expected error renaming unknown user")
	}
	if err := cfg.RenameUser("b", ""); err == nil {
		t.Fatal("expected error for empty new name")
	}
	if err := cfg.RenameUser("b", "c"); err == nil {
		t.Fatal("expected error for duplicate new name")
	}
	// Renaming to the same name is a no-op.
	if err := cfg.RenameUser("b", "b"); err != nil {
		t.Fatalf("renaming to same name should succeed: %v", err)
	}
}

func TestMarshalYAMLNewFormat(t *testing.T) {
	cfg := NewConfig()
	cfg.Auth.DefaultUser = "default"
	cfg.Auth.Users = []UserEntry{{Name: "default", BaseURL: DefaultBaseURL, APIKey: "secret"}}

	data, err := MarshalYAML(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	content := string(data)
	if strings.HasPrefix(strings.TrimSpace(content), "---") {
		t.Fatalf("yaml encoder should not emit a document marker: %q", content)
	}
	if !containsAll(content, "auth:", "default_user: default", "users:", "name: default", "api_key: secret") {
		t.Fatalf("unexpected marshaled config: %q", content)
	}
	// 2-space indent (no 4-space runs like "    name").
	if strings.Contains(content, "\n    name:") {
		t.Fatalf("expected 2-space indent, got 4-space:\n%s", content)
	}
}

func TestViewMasksAPIKeys(t *testing.T) {
	cfg := NewConfig()
	cfg.Auth.DefaultUser = "aaa"
	cfg.Auth.Users = []UserEntry{{Name: "aaa", APIKey: "tdk_abcdef_secret", BaseURL: "http://x"}}

	v := cfg.View()
	if v.Auth.DefaultUser != "aaa" {
		t.Fatalf("expected default_user aaa, got %q", v.Auth.DefaultUser)
	}
	if len(v.Auth.Users) != 1 {
		t.Fatalf("expected 1 user in view")
	}
	if v.Auth.Users[0].APIKey != "tdk_abcd****" {
		t.Fatalf("expected masked key, got %q", v.Auth.Users[0].APIKey)
	}
}

func TestUserListMarksDefault(t *testing.T) {
	cfg := NewConfig()
	cfg.Auth.DefaultUser = "aaa"
	cfg.Auth.Users = []UserEntry{{Name: "aaa", APIKey: "tdk_abcdef_secret"}, {Name: "bbb", APIKey: "k2"}}

	items := cfg.UserList()
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if !items[0].IsDefault || items[1].IsDefault {
		t.Fatalf("default marker wrong: %+v", items)
	}
	if items[0].APIKey != "tdk_abcd****" {
		t.Fatalf("expected masked key, got %q", items[0].APIKey)
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
