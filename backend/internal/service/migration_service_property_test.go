package service

import (
	"fmt"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/graydovee/todolist/internal/model"
	"github.com/graydovee/todolist/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"pgregory.net/rapid"
)

// setupMigrationTestDB creates an in-memory SQLite database for migration tests.
func setupMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	id := propertyTestDBCounter.Add(1)
	dsn := fmt.Sprintf("file:migdb_%d?mode=memory", id)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	sqlDB, _ := db.DB()
	tables := []string{
		`CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY AUTOINCREMENT, auth_provider TEXT NOT NULL DEFAULT '', auth_subject TEXT NOT NULL DEFAULT '', display_name TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS todos (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, code TEXT NOT NULL, title TEXT NOT NULL, description TEXT DEFAULT '', category TEXT NOT NULL CHECK(category IN ('bug','feature','task')), priority TEXT NOT NULL DEFAULT 'p2' CHECK(priority IN ('p0','p1','p2','p3')), status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open','in_progress','completed')), due_at DATETIME, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, UNIQUE(user_id, code))`,
		`CREATE TABLE IF NOT EXISTS todo_tags (id INTEGER PRIMARY KEY AUTOINCREMENT, todo_id INTEGER NOT NULL, tag TEXT NOT NULL, UNIQUE(todo_id, tag))`,
		`CREATE TABLE IF NOT EXISTS todo_relations (id INTEGER PRIMARY KEY AUTOINCREMENT, source_id INTEGER NOT NULL, target_id INTEGER NOT NULL, relation_type TEXT NOT NULL CHECK(relation_type IN ('depends_on','duplicate_of')), UNIQUE(source_id, target_id, relation_type))`,
		`CREATE TABLE IF NOT EXISTS code_counters (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, last_code INTEGER NOT NULL DEFAULT 0, UNIQUE(user_id))`,
		`CREATE TABLE IF NOT EXISTS sessions (id INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL, user_id INTEGER NOT NULL, data BLOB, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, expires_at DATETIME NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS comments (id INTEGER PRIMARY KEY AUTOINCREMENT, todo_id INTEGER NOT NULL, user_id INTEGER NOT NULL, content TEXT NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
	}
	for _, ddl := range tables {
		if _, err := sqlDB.Exec(ddl); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	return db
}

// setupMigrationService creates a MigrationService backed by an in-memory SQLite DB.
func setupMigrationService(t *testing.T) (*MigrationService, *gorm.DB) {
	t.Helper()
	db := setupMigrationTestDB(t)
	todoRepo := repository.NewTodoRepo(db)
	counterRepo := repository.NewCodeCounterRepo(db)
	svc := NewMigrationService(db, todoRepo, counterRepo)
	return svc, db
}

// Feature: numbering-system-refactor, Property 9: Migration assigns sequential codes in creation order
// **Validates: Requirements 4.1, 4.2**
//
// Property: For any set of todos belonging to a single user where all codes match
// the old format (^[A-Z]+-\d+$), after migration, the codes SHALL be reassigned as
// "1", "2", ..., "N" in order of (created_at ASC, id ASC).
func TestProperty_MigrationAssignsSequentialCodesInCreationOrder(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, db := setupMigrationService(t)

		userID := uint(1)
		// Generate between 1 and 30 todos with old-format codes
		n := rapid.IntRange(1, 30).Draw(rt, "numTodos")

		// Generate random creation timestamps (within a range to allow duplicates)
		baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		type todoInput struct {
			code      string
			createdAt time.Time
		}
		inputs := make([]todoInput, n)
		for i := 0; i < n; i++ {
			// Generate unique old-format code by incorporating index into numeric suffix
			prefixLen := rapid.IntRange(1, 8).Draw(rt, fmt.Sprintf("code_%d_prefixLen", i))
			prefix := make([]byte, prefixLen)
			for j := range prefix {
				prefix[j] = byte(rapid.IntRange(65, 90).Draw(rt, fmt.Sprintf("code_%d_char_%d", i, j)))
			}
			code := fmt.Sprintf("%s-%d", string(prefix), (i+1)*10000+rapid.IntRange(1, 9999).Draw(rt, fmt.Sprintf("code_%d_num", i)))
			// Random offset in seconds (0 to 1000) — allows duplicate timestamps
			offsetSec := rapid.IntRange(0, 1000).Draw(rt, fmt.Sprintf("offset_%d", i))
			createdAt := baseTime.Add(time.Duration(offsetSec) * time.Second)
			inputs[i] = todoInput{code: code, createdAt: createdAt}
		}

		// Insert todos directly into the database
		categories := []string{"bug", "feature", "task"}
		for i, inp := range inputs {
			cat := categories[i%3]
			todo := model.Todo{
				UserID:    userID,
				Code:      inp.code,
				Title:     fmt.Sprintf("Todo %d", i+1),
				Category:  cat,
				Priority:  "p2",
				Status:    "open",
				CreatedAt: inp.createdAt,
				UpdatedAt: inp.createdAt,
			}
			if err := db.Create(&todo).Error; err != nil {
				rt.Fatalf("insert todo %d: %v", i, err)
			}
		}

		// Run migration
		if err := svc.Run(); err != nil {
			rt.Fatalf("migration run: %v", err)
		}

		// Load todos after migration
		var todos []*model.Todo
		if err := db.Where("user_id = ?", userID).Find(&todos).Error; err != nil {
			rt.Fatalf("load todos: %v", err)
		}

		// Sort by (created_at ASC, id ASC) — same order migration uses
		sort.Slice(todos, func(i, j int) bool {
			if todos[i].CreatedAt.Equal(todos[j].CreatedAt) {
				return todos[i].ID < todos[j].ID
			}
			return todos[i].CreatedAt.Before(todos[j].CreatedAt)
		})

		// Verify codes are "1", "2", ..., "N" in that order
		if len(todos) != n {
			rt.Fatalf("expected %d todos, got %d", n, len(todos))
		}
		for i, todo := range todos {
			expected := strconv.Itoa(i + 1)
			if todo.Code != expected {
				rt.Fatalf("todo id=%d: expected code %q, got %q (position %d)", todo.ID, expected, todo.Code, i)
			}
		}
	})
}

// Feature: numbering-system-refactor, Property 10: Migration processes users independently
// **Validates: Requirements 4.3, 4.4**
//
// Property: For any set of users each having old-format todos, after migration,
// each user's todos SHALL have codes starting from "1" independently, and each
// user SHALL have exactly one CodeCounter record with last_code equal to their todo count.
func TestProperty_MigrationProcessesUsersIndependently(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, db := setupMigrationService(t)

		// Generate 2 to 5 users
		numUsers := rapid.IntRange(2, 5).Draw(rt, "numUsers")
		baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		categories := []string{"bug", "feature", "task"}

		// Track expected todo counts per user
		todoCounts := make(map[uint]int)

		for u := 1; u <= numUsers; u++ {
			userID := uint(u)
			// Each user gets 1 to 15 todos
			n := rapid.IntRange(1, 15).Draw(rt, fmt.Sprintf("user%d_numTodos", u))
			todoCounts[userID] = n

			for i := 0; i < n; i++ {
				// Generate unique old-format code by incorporating index into numeric suffix
				prefixLen := rapid.IntRange(1, 8).Draw(rt, fmt.Sprintf("u%d_code_%d_prefixLen", u, i))
				prefix := make([]byte, prefixLen)
				for j := range prefix {
					prefix[j] = byte(rapid.IntRange(65, 90).Draw(rt, fmt.Sprintf("u%d_code_%d_char_%d", u, i, j)))
				}
				code := fmt.Sprintf("%s-%d", string(prefix), (i+1)*10000+rapid.IntRange(1, 9999).Draw(rt, fmt.Sprintf("u%d_code_%d_num", u, i)))
				offsetSec := rapid.IntRange(0, 500).Draw(rt, fmt.Sprintf("u%d_offset_%d", u, i))
				createdAt := baseTime.Add(time.Duration(offsetSec) * time.Second)
				cat := categories[i%3]
				todo := model.Todo{
					UserID:    userID,
					Code:      code,
					Title:     fmt.Sprintf("User%d Todo%d", u, i+1),
					Category:  cat,
					Priority:  "p2",
					Status:    "open",
					CreatedAt: createdAt,
					UpdatedAt: createdAt,
				}
				if err := db.Create(&todo).Error; err != nil {
					rt.Fatalf("insert todo for user %d: %v", u, err)
				}
			}
		}

		// Run migration
		if err := svc.Run(); err != nil {
			rt.Fatalf("migration run: %v", err)
		}

		// Verify each user independently
		for u := 1; u <= numUsers; u++ {
			userID := uint(u)
			expectedCount := todoCounts[userID]

			// Load todos for this user
			var todos []*model.Todo
			if err := db.Where("user_id = ?", userID).Find(&todos).Error; err != nil {
				rt.Fatalf("load todos for user %d: %v", u, err)
			}

			if len(todos) != expectedCount {
				rt.Fatalf("user %d: expected %d todos, got %d", u, expectedCount, len(todos))
			}

			// Verify codes start from "1" for each user
			sort.Slice(todos, func(i, j int) bool {
				if todos[i].CreatedAt.Equal(todos[j].CreatedAt) {
					return todos[i].ID < todos[j].ID
				}
				return todos[i].CreatedAt.Before(todos[j].CreatedAt)
			})
			for i, todo := range todos {
				expected := strconv.Itoa(i + 1)
				if todo.Code != expected {
					rt.Fatalf("user %d, todo id=%d: expected code %q, got %q", u, todo.ID, expected, todo.Code)
				}
			}

			// Verify exactly one CodeCounter record with last_code = count
			var counters []model.CodeCounter
			if err := db.Where("user_id = ?", userID).Find(&counters).Error; err != nil {
				rt.Fatalf("load counters for user %d: %v", u, err)
			}
			if len(counters) != 1 {
				rt.Fatalf("user %d: expected 1 CodeCounter, got %d", u, len(counters))
			}
			if counters[0].LastCode != expectedCount {
				rt.Fatalf("user %d: expected last_code=%d, got %d", u, expectedCount, counters[0].LastCode)
			}
		}
	})
}

// Feature: numbering-system-refactor, Property 11: Migration skips already-migrated users
// **Validates: Requirements 4.7**
//
// Property: For any user whose todos all have numeric-only codes (^\d+$), running
// the migration SHALL leave all their codes and CodeCounter records unchanged.
func TestProperty_MigrationSkipsAlreadyMigratedUsers(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, db := setupMigrationService(t)

		userID := uint(1)
		// Generate 1 to 20 todos with numeric codes
		n := rapid.IntRange(1, 20).Draw(rt, "numTodos")
		baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		categories := []string{"bug", "feature", "task"}

		// Generate unique numeric codes (use sequential to avoid duplicates)
		for i := 0; i < n; i++ {
			code := strconv.Itoa(i + 1)
			offsetSec := i * 60
			createdAt := baseTime.Add(time.Duration(offsetSec) * time.Second)
			cat := categories[i%3]
			todo := model.Todo{
				UserID:    userID,
				Code:      code,
				Title:     fmt.Sprintf("Todo %d", i+1),
				Category:  cat,
				Priority:  "p2",
				Status:    "open",
				CreatedAt: createdAt,
				UpdatedAt: createdAt,
			}
			if err := db.Create(&todo).Error; err != nil {
				rt.Fatalf("insert todo %d: %v", i, err)
			}
		}

		// Also create a CodeCounter record (simulating already-migrated state)
		counter := model.CodeCounter{
			UserID:   userID,
			LastCode: n,
		}
		if err := db.Create(&counter).Error; err != nil {
			rt.Fatalf("create counter: %v", err)
		}

		// Snapshot codes and counter before migration
		var todosBefore []*model.Todo
		if err := db.Where("user_id = ?", userID).Order("id ASC").Find(&todosBefore).Error; err != nil {
			rt.Fatalf("load todos before: %v", err)
		}
		codesBefore := make([]string, len(todosBefore))
		for i, todo := range todosBefore {
			codesBefore[i] = todo.Code
		}

		var countersBefore []model.CodeCounter
		if err := db.Where("user_id = ?", userID).Find(&countersBefore).Error; err != nil {
			rt.Fatalf("load counters before: %v", err)
		}

		// Run migration
		if err := svc.Run(); err != nil {
			rt.Fatalf("migration run: %v", err)
		}

		// Verify codes are unchanged
		var todosAfter []*model.Todo
		if err := db.Where("user_id = ?", userID).Order("id ASC").Find(&todosAfter).Error; err != nil {
			rt.Fatalf("load todos after: %v", err)
		}
		if len(todosAfter) != len(todosBefore) {
			rt.Fatalf("todo count changed: before=%d, after=%d", len(todosBefore), len(todosAfter))
		}
		for i, todo := range todosAfter {
			if todo.Code != codesBefore[i] {
				rt.Fatalf("todo id=%d: code changed from %q to %q", todo.ID, codesBefore[i], todo.Code)
			}
		}

		// Verify CodeCounter is unchanged
		var countersAfter []model.CodeCounter
		if err := db.Where("user_id = ?", userID).Find(&countersAfter).Error; err != nil {
			rt.Fatalf("load counters after: %v", err)
		}
		if len(countersAfter) != len(countersBefore) {
			rt.Fatalf("counter count changed: before=%d, after=%d", len(countersBefore), len(countersAfter))
		}
		if len(countersAfter) > 0 && countersAfter[0].LastCode != countersBefore[0].LastCode {
			rt.Fatalf("counter last_code changed: before=%d, after=%d", countersBefore[0].LastCode, countersAfter[0].LastCode)
		}
	})
}

// Feature: numbering-system-refactor, Property 12: Migration skips corrupted-state users
// **Validates: Requirements 4.8**
//
// Property: For any user who has a mix of old-format codes and numeric codes,
// running the migration SHALL leave all their codes unchanged and log a warning.
func TestProperty_MigrationSkipsCorruptedStateUsers(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, db := setupMigrationService(t)

		userID := uint(1)
		baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		categories := []string{"bug", "feature", "task"}

		// Generate at least 1 old-format code and at least 1 numeric code
		numOld := rapid.IntRange(1, 10).Draw(rt, "numOldFormat")
		numNumeric := rapid.IntRange(1, 10).Draw(rt, "numNumeric")
		total := numOld + numNumeric

		// Insert old-format todos with unique codes (use index in numeric suffix to avoid duplicates)
		for i := 0; i < numOld; i++ {
			// Generate a prefix of 1-8 uppercase letters
			prefixLen := rapid.IntRange(1, 8).Draw(rt, fmt.Sprintf("old_%d_prefixLen", i))
			prefix := make([]byte, prefixLen)
			for j := range prefix {
				prefix[j] = byte(rapid.IntRange(65, 90).Draw(rt, fmt.Sprintf("old_%d_char_%d", i, j)))
			}
			// Use a unique numeric suffix by combining random base with index
			code := fmt.Sprintf("%s-%d", string(prefix), (i+1)*10000+rapid.IntRange(1, 9999).Draw(rt, fmt.Sprintf("old_%d_num", i)))
			offsetSec := i * 60
			createdAt := baseTime.Add(time.Duration(offsetSec) * time.Second)
			cat := categories[i%3]
			todo := model.Todo{
				UserID:    userID,
				Code:      code,
				Title:     fmt.Sprintf("OldTodo %d", i+1),
				Category:  cat,
				Priority:  "p2",
				Status:    "open",
				CreatedAt: createdAt,
				UpdatedAt: createdAt,
			}
			if err := db.Create(&todo).Error; err != nil {
				rt.Fatalf("insert old-format todo %d: %v", i, err)
			}
		}

		// Insert numeric-code todos (use high numbers to avoid code conflicts)
		for i := 0; i < numNumeric; i++ {
			code := strconv.Itoa(1000 + i)
			offsetSec := (numOld + i) * 60
			createdAt := baseTime.Add(time.Duration(offsetSec) * time.Second)
			cat := categories[i%3]
			todo := model.Todo{
				UserID:    userID,
				Code:      code,
				Title:     fmt.Sprintf("NumericTodo %d", i+1),
				Category:  cat,
				Priority:  "p2",
				Status:    "open",
				CreatedAt: createdAt,
				UpdatedAt: createdAt,
			}
			if err := db.Create(&todo).Error; err != nil {
				rt.Fatalf("insert numeric todo %d: %v", i, err)
			}
		}

		// Snapshot codes before migration
		var todosBefore []*model.Todo
		if err := db.Where("user_id = ?", userID).Order("id ASC").Find(&todosBefore).Error; err != nil {
			rt.Fatalf("load todos before: %v", err)
		}
		if len(todosBefore) != total {
			rt.Fatalf("expected %d todos, got %d", total, len(todosBefore))
		}
		codesBefore := make([]string, len(todosBefore))
		for i, todo := range todosBefore {
			codesBefore[i] = todo.Code
		}

		// Run migration
		if err := svc.Run(); err != nil {
			rt.Fatalf("migration run: %v", err)
		}

		// Verify all codes are unchanged
		var todosAfter []*model.Todo
		if err := db.Where("user_id = ?", userID).Order("id ASC").Find(&todosAfter).Error; err != nil {
			rt.Fatalf("load todos after: %v", err)
		}
		if len(todosAfter) != total {
			rt.Fatalf("todo count changed: before=%d, after=%d", total, len(todosAfter))
		}
		for i, todo := range todosAfter {
			if todo.Code != codesBefore[i] {
				rt.Fatalf("todo id=%d: code changed from %q to %q (corrupted user should be skipped)", todo.ID, codesBefore[i], todo.Code)
			}
		}
	})
}
