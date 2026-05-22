package handler

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/graydovee/todolist/internal/model"
	"github.com/graydovee/todolist/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"pgregory.net/rapid"
)

var handlerPropertyTestDBCounter atomic.Int64

// setupHandlerTestDB creates an in-memory SQLite database for handler property tests.
func setupHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	id := handlerPropertyTestDBCounter.Add(1)
	dsn := fmt.Sprintf("file:handlerdb_%d?mode=memory", id)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	sqlDB, _ := db.DB()
	tables := []string{
		`CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY AUTOINCREMENT, auth_provider TEXT NOT NULL DEFAULT '', auth_subject TEXT NOT NULL DEFAULT '', display_name TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS todos (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, code TEXT NOT NULL, title TEXT NOT NULL, description TEXT DEFAULT '', category TEXT NOT NULL CHECK(category IN ('bug','feature','task')), priority TEXT NOT NULL DEFAULT 'p2' CHECK(priority IN ('p0','p1','p2','p3')), status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open','in_progress','completed')), due_at DATETIME, pinned INTEGER NOT NULL DEFAULT 0, highlighted INTEGER NOT NULL DEFAULT 0, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, UNIQUE(user_id, code))`,
		`CREATE TABLE IF NOT EXISTS todo_tags (id INTEGER PRIMARY KEY AUTOINCREMENT, todo_id INTEGER NOT NULL, tag TEXT NOT NULL, UNIQUE(todo_id, tag))`,
	}
	for _, ddl := range tables {
		if _, err := sqlDB.Exec(ddl); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	return db
}

// Feature: ai-summary-streaming, Property 9: Date range query correctness
// **Validates: Requirements 8.2, 8.3**
//
// Property: For any valid date range query, the repository SHALL return only todos
// belonging to the authenticated user whose updated_at falls within [start_date, end_date],
// and each returned todo SHALL include the fields: id, code, title, status, category, and priority.
func TestProperty_DateRangeQueryCorrectness(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		db := setupHandlerTestDB(t)
		todoRepo := repository.NewTodoRepo(db)

		userID := uint(1)
		otherUserID := uint(2)

		categories := []string{"bug", "feature", "task"}
		priorities := []string{"p0", "p1", "p2", "p3"}
		statuses := []string{"open", "in_progress", "completed"}

		// Generate a base date around which we create todos
		baseYear := rapid.IntRange(2020, 2025).Draw(rt, "baseYear")
		baseMonth := rapid.IntRange(1, 12).Draw(rt, "baseMonth")
		baseDay := rapid.IntRange(1, 28).Draw(rt, "baseDay") // use 28 to avoid month overflow
		baseDate := time.Date(baseYear, time.Month(baseMonth), baseDay, 0, 0, 0, 0, time.UTC)

		// Generate random todos (3 to 15) with various updated_at values
		numTodos := rapid.IntRange(3, 15).Draw(rt, "numTodos")

		type todoRecord struct {
			id        uint
			userID    uint
			code      string
			title     string
			status    string
			category  string
			priority  string
			updatedAt time.Time
		}
		allTodos := make([]todoRecord, 0, numTodos)

		for i := range numTodos {
			// Randomly assign to user or other user
			uid := userID
			if rapid.Bool().Draw(rt, fmt.Sprintf("otherUser_%d", i)) {
				uid = otherUserID
			}

			cat := categories[rapid.IntRange(0, len(categories)-1).Draw(rt, fmt.Sprintf("cat_%d", i))]
			pri := priorities[rapid.IntRange(0, len(priorities)-1).Draw(rt, fmt.Sprintf("pri_%d", i))]
			status := statuses[rapid.IntRange(0, len(statuses)-1).Draw(rt, fmt.Sprintf("status_%d", i))]

			// Generate updated_at within a 60-day window around baseDate
			dayOffset := rapid.IntRange(-30, 30).Draw(rt, fmt.Sprintf("dayOffset_%d", i))
			hourOffset := rapid.IntRange(0, 23).Draw(rt, fmt.Sprintf("hour_%d", i))
			minOffset := rapid.IntRange(0, 59).Draw(rt, fmt.Sprintf("min_%d", i))
			updatedAt := baseDate.AddDate(0, 0, dayOffset).Add(
				time.Duration(hourOffset)*time.Hour + time.Duration(minOffset)*time.Minute,
			)

			code := fmt.Sprintf("T-%03d", i+1)
			title := fmt.Sprintf("Todo item %d", i+1)

			todo := model.Todo{
				UserID:   uid,
				Code:     code,
				Title:    title,
				Category: cat,
				Priority: pri,
				Status:   status,
			}
			if err := db.Create(&todo).Error; err != nil {
				rt.Fatalf("create todo %d: %v", i, err)
			}

			// Manually set updated_at since GORM auto-updates it
			if err := db.Exec("UPDATE todos SET updated_at = ? WHERE id = ?", updatedAt, todo.ID).Error; err != nil {
				rt.Fatalf("update updated_at for todo %d: %v", i, err)
			}

			allTodos = append(allTodos, todoRecord{
				id:        todo.ID,
				userID:    uid,
				code:      code,
				title:     title,
				status:    status,
				category:  cat,
				priority:  pri,
				updatedAt: updatedAt,
			})
		}

		// Generate a random date range for the query
		startOffset := rapid.IntRange(-30, 20).Draw(rt, "startOffset")
		rangeWidth := rapid.IntRange(1, 30).Draw(rt, "rangeWidth")
		startDate := baseDate.AddDate(0, 0, startOffset)
		endDate := startDate.AddDate(0, 0, rangeWidth)

		// Use end of day for endDate (matching handler behavior)
		endDateTime := endDate.Add(24*time.Hour - time.Nanosecond)

		// Call the repository method
		results, err := todoRepo.FindByUserAndUpdatedAtRange(nil, userID, startDate, endDateTime)
		if err != nil {
			rt.Fatalf("FindByUserAndUpdatedAtRange: %v", err)
		}

		// Compute expected set: todos belonging to userID with updated_at in [startDate, endDateTime]
		expectedIDs := make(map[uint]bool)
		expectedTodos := make(map[uint]todoRecord)
		for _, todo := range allTodos {
			if todo.userID == userID &&
				!todo.updatedAt.Before(startDate) &&
				!todo.updatedAt.After(endDateTime) {
				expectedIDs[todo.id] = true
				expectedTodos[todo.id] = todo
			}
		}

		// Verify: result contains exactly the expected todos
		resultIDs := make(map[uint]bool)
		for _, todo := range results {
			resultIDs[todo.ID] = true
		}

		// No missing todos
		for id := range expectedIDs {
			if !resultIDs[id] {
				rt.Fatalf("expected todo ID %d in results but it was missing\nstart: %v, end: %v\nexpected IDs: %v\ngot IDs: %v",
					id, startDate, endDateTime, expectedIDs, resultIDs)
			}
		}

		// No extra todos (no todos outside range or from other users)
		for id := range resultIDs {
			if !expectedIDs[id] {
				rt.Fatalf("unexpected todo ID %d in results\nstart: %v, end: %v\nexpected IDs: %v\ngot IDs: %v",
					id, startDate, endDateTime, expectedIDs, resultIDs)
			}
		}

		// Verify all required fields are present for each returned todo
		for _, todo := range results {
			if todo.ID == 0 {
				rt.Fatalf("returned todo has zero ID")
			}
			if todo.Code == "" {
				rt.Fatalf("returned todo ID %d has empty code", todo.ID)
			}
			if todo.Title == "" {
				rt.Fatalf("returned todo ID %d has empty title", todo.ID)
			}
			if todo.Status == "" {
				rt.Fatalf("returned todo ID %d has empty status", todo.ID)
			}
			if todo.Category == "" {
				rt.Fatalf("returned todo ID %d has empty category", todo.ID)
			}
			if todo.Priority == "" {
				rt.Fatalf("returned todo ID %d has empty priority", todo.ID)
			}

			// Verify field values match what was inserted
			expected, ok := expectedTodos[todo.ID]
			if !ok {
				rt.Fatalf("todo ID %d not found in expected set", todo.ID)
			}
			if todo.Code != expected.code {
				rt.Fatalf("todo ID %d code mismatch: expected %q, got %q", todo.ID, expected.code, todo.Code)
			}
			if todo.Title != expected.title {
				rt.Fatalf("todo ID %d title mismatch: expected %q, got %q", todo.ID, expected.title, todo.Title)
			}
			if todo.Status != expected.status {
				rt.Fatalf("todo ID %d status mismatch: expected %q, got %q", todo.ID, expected.status, todo.Status)
			}
			if todo.Category != expected.category {
				rt.Fatalf("todo ID %d category mismatch: expected %q, got %q", todo.ID, expected.category, todo.Category)
			}
			if todo.Priority != expected.priority {
				rt.Fatalf("todo ID %d priority mismatch: expected %q, got %q", todo.ID, expected.priority, todo.Priority)
			}
		}
	})
}
