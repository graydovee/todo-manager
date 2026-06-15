package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/graydovee/todolist/internal/model"
	"github.com/graydovee/todolist/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupAccessKeyServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:accesskeys_%d?mode=memory", time.Now().UnixNano())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	sqlDB, _ := db.DB()
	ddls := []string{
		`CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY AUTOINCREMENT, auth_provider TEXT NOT NULL, auth_subject TEXT NOT NULL, display_name TEXT NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS access_keys (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, name TEXT NOT NULL, key_prefix TEXT NOT NULL, key_salt TEXT NOT NULL, key_hash TEXT NOT NULL, authorized_apis_json TEXT NOT NULL, expires_at DATETIME, last_used_at DATETIME, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_access_keys_key_prefix ON access_keys(key_prefix)`,
	}
	for _, ddl := range ddls {
		if _, err := sqlDB.Exec(ddl); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	return db
}

func createAccessKeyTestUser(t *testing.T, db *gorm.DB) uint {
	t.Helper()
	user := model.User{
		AuthProvider: "test",
		AuthSubject:  fmt.Sprintf("user_%d", time.Now().UnixNano()),
		DisplayName:  "Test User",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user.ID
}

func TestAccessKeyServiceCreateNormalizesPermissions(t *testing.T) {
	db := setupAccessKeyServiceTestDB(t)
	service := NewAccessKeyService(db, repository.NewAccessKeyRepo(db), repository.NewUserRepo(db))
	userID := createAccessKeyTestUser(t, db)

	key, plainKey, err := service.Create(userID, CreateAccessKeyInput{
		Name:           "CLI",
		AuthorizedAPIs: []string{"todos:get", "todos:list", "todos:get"},
	})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	if plainKey == "" {
		t.Fatal("expected plain key to be returned")
	}
	if key.KeyHash == "" || key.KeySalt == "" {
		t.Fatal("expected hash and salt to be stored")
	}
	if key.AuthorizedAPIsJSON != `["todos:get","todos:list"]` {
		t.Fatalf("unexpected authorized api list: %s", key.AuthorizedAPIsJSON)
	}
}

func TestAccessKeyServiceRotateInvalidatesOldKey(t *testing.T) {
	db := setupAccessKeyServiceTestDB(t)
	service := NewAccessKeyService(db, repository.NewAccessKeyRepo(db), repository.NewUserRepo(db))
	userID := createAccessKeyTestUser(t, db)

	key, plainKey, err := service.Create(userID, CreateAccessKeyInput{
		Name:           "CLI",
		AuthorizedAPIs: []string{"todos:list"},
	})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}

	result, err := service.Rotate(userID, key.ID)
	if err != nil {
		t.Fatalf("rotate key: %v", err)
	}
	if result.PlainKey == "" || result.PlainKey == plainKey {
		t.Fatal("expected a new plain key after rotation")
	}
	if result.Key.Name != key.Name {
		t.Fatal("expected name to stay unchanged")
	}
	if result.Key.AuthorizedAPIsJSON != key.AuthorizedAPIsJSON {
		t.Fatal("expected authorized APIs to stay unchanged")
	}
	if result.Key.LastUsedAt != nil {
		t.Fatal("expected last_used_at to be reset")
	}
	if _, err := service.Authenticate(plainKey); err == nil {
		t.Fatal("expected old key to be rejected after rotation")
	}
	if _, err := service.Authenticate(result.PlainKey); err != nil {
		t.Fatalf("expected new key to authenticate: %v", err)
	}
}
