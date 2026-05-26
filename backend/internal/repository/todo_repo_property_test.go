package repository

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/graydovee/todolist/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"pgregory.net/rapid"
)

var repoPropertyTestDBCounter atomic.Int64

// setupRepoTestDB creates an in-memory SQLite database for repository property tests.
func setupRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	id := repoPropertyTestDBCounter.Add(1)
	dsn := fmt.Sprintf("file:repodb_%d?mode=memory", id)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	sqlDB, _ := db.DB()
	tables := []string{
		`CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY AUTOINCREMENT, auth_provider TEXT NOT NULL DEFAULT '', auth_subject TEXT NOT NULL DEFAULT '', display_name TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS todos (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, code TEXT NOT NULL, title TEXT NOT NULL, description TEXT DEFAULT '', category TEXT NOT NULL CHECK(category IN ('bug','feature','task')), priority TEXT NOT NULL DEFAULT 'p2' CHECK(priority IN ('p0','p1','p2','p3')), status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open','in_progress','completed','duplicate')), due_at DATETIME, pinned INTEGER NOT NULL DEFAULT 0, highlighted INTEGER NOT NULL DEFAULT 0, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, UNIQUE(user_id, code))`,
		`CREATE TABLE IF NOT EXISTS todo_tags (id INTEGER PRIMARY KEY AUTOINCREMENT, todo_id INTEGER NOT NULL, tag TEXT NOT NULL, UNIQUE(todo_id, tag))`,
	}
	for _, ddl := range tables {
		if _, err := sqlDB.Exec(ddl); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	return db
}

// Feature: todo-filter-duplicate, Property 1: updated_after filter returns exactly matching todos
// **Validates: Requirements 1.2, 1.4**
//
// Property: For any set of todos with various updated_at timestamps and for any valid
// updated_after threshold, the List method with UpdatedAfter filter SHALL return exactly
// those todos whose updated_at is greater than or equal to the threshold, and no others.
func TestProperty_UpdatedAfterFilterCorrectness(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		db := setupRepoTestDB(t)
		todoRepo := NewTodoRepo(db)

		userID := uint(1)

		categories := []string{"bug", "feature", "task"}
		priorities := []string{"p0", "p1", "p2", "p3"}
		statuses := []string{"open", "in_progress", "completed"}

		// Generate a base date around which we create todos
		baseYear := rapid.IntRange(2020, 2025).Draw(rt, "baseYear")
		baseMonth := rapid.IntRange(1, 12).Draw(rt, "baseMonth")
		baseDay := rapid.IntRange(1, 28).Draw(rt, "baseDay")
		baseDate := time.Date(baseYear, time.Month(baseMonth), baseDay, 12, 0, 0, 0, time.UTC)

		// Generate random todos (3 to 20) with various updated_at values
		numTodos := rapid.IntRange(3, 20).Draw(rt, "numTodos")

		type todoRecord struct {
			id        uint
			updatedAt time.Time
		}
		allTodos := make([]todoRecord, 0, numTodos)

		for i := range numTodos {
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

			code := fmt.Sprintf("T-%04d", i+1)
			title := fmt.Sprintf("Todo %d", i+1)

			todo := model.Todo{
				UserID:   userID,
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
				updatedAt: updatedAt,
			})
		}

		// Generate a random threshold within the range of generated timestamps
		thresholdDayOffset := rapid.IntRange(-30, 30).Draw(rt, "thresholdDayOffset")
		thresholdHour := rapid.IntRange(0, 23).Draw(rt, "thresholdHour")
		thresholdMin := rapid.IntRange(0, 59).Draw(rt, "thresholdMin")
		threshold := baseDate.AddDate(0, 0, thresholdDayOffset).Add(
			time.Duration(thresholdHour)*time.Hour + time.Duration(thresholdMin)*time.Minute,
		)

		// Call List with UpdatedAfter filter, use large page size to get all results
		filters := TodoFilters{
			UpdatedAfter: &threshold,
			Page:         1,
			PageSize:     1000,
		}
		results, total, err := todoRepo.List(nil, userID, filters)
		if err != nil {
			rt.Fatalf("List with UpdatedAfter: %v", err)
		}

		// Compute expected set: todos with updated_at >= threshold
		expectedIDs := make(map[uint]bool)
		for _, todo := range allTodos {
			if !todo.updatedAt.Before(threshold) {
				expectedIDs[todo.id] = true
			}
		}

		// Verify total count matches expected
		if int64(len(expectedIDs)) != total {
			rt.Fatalf("total mismatch: expected %d, got %d (threshold: %v)",
				len(expectedIDs), total, threshold)
		}

		// Verify result set matches expected
		resultIDs := make(map[uint]bool)
		for _, todo := range results {
			resultIDs[todo.ID] = true
		}

		// No missing todos
		for id := range expectedIDs {
			if !resultIDs[id] {
				rt.Fatalf("expected todo ID %d in results but it was missing (threshold: %v)",
					id, threshold)
			}
		}

		// No extra todos
		for id := range resultIDs {
			if !expectedIDs[id] {
				rt.Fatalf("unexpected todo ID %d in results (threshold: %v)",
					id, threshold)
			}
		}
	})
}
