package repository

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/graydovee/todo-manager/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"pgregory.net/rapid"
)

// setupStatusHistoryTestDB creates an in-memory SQLite database for status history property tests.
func setupStatusHistoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	id := repoPropertyTestDBCounter.Add(1)
	dsn := fmt.Sprintf("file:statushistorydb_%d?mode=memory", id)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	sqlDB, _ := db.DB()
	tables := []string{
		`CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY AUTOINCREMENT, auth_provider TEXT NOT NULL DEFAULT '', auth_subject TEXT NOT NULL DEFAULT '', display_name TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS todos (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, code TEXT NOT NULL, title TEXT NOT NULL, description TEXT DEFAULT '', category TEXT NOT NULL, priority TEXT NOT NULL DEFAULT 'p2', status TEXT NOT NULL DEFAULT 'open', due_at DATETIME, pinned INTEGER NOT NULL DEFAULT 0, highlighted INTEGER NOT NULL DEFAULT 0, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, UNIQUE(user_id, code))`,
		`CREATE TABLE IF NOT EXISTS todo_tags (id INTEGER PRIMARY KEY AUTOINCREMENT, todo_id INTEGER NOT NULL, tag TEXT NOT NULL, UNIQUE(todo_id, tag))`,
		`CREATE TABLE IF NOT EXISTS todo_status_history (id INTEGER PRIMARY KEY AUTOINCREMENT, todo_id INTEGER NOT NULL, old_status TEXT NOT NULL, new_status TEXT NOT NULL, changed_at DATETIME NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS idx_status_history_todo_id ON todo_status_history(todo_id)`,
		`CREATE INDEX IF NOT EXISTS idx_status_history_changed_at ON todo_status_history(changed_at)`,
	}
	for _, ddl := range tables {
		if _, err := sqlDB.Exec(ddl); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	return db
}

// genTimestamp generates a random timestamp between 2020-01-01 and 2024-12-31.
func genTimestamp(rt *rapid.T, label string) time.Time {
	year := rapid.IntRange(2020, 2024).Draw(rt, label+"_year")
	month := rapid.IntRange(1, 12).Draw(rt, label+"_month")
	day := rapid.IntRange(1, 28).Draw(rt, label+"_day")
	hour := rapid.IntRange(0, 23).Draw(rt, label+"_hour")
	minute := rapid.IntRange(0, 59).Draw(rt, label+"_min")
	second := rapid.IntRange(0, 59).Draw(rt, label+"_sec")
	return time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC)
}

// Feature: ai-summary-enhancement, Property 2: Activity filter returns exactly matching todos
// **Validates: Requirements 1.4, 1.5**
//
// Property: For any user, set of todos, and time range [start, end], the activity filter
// SHALL return exactly those todos belonging to that user whose updated_at falls within
// [start, end], and no others.
func TestProperty_ActivityFilterReturnsExactlyMatchingTodos(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		db := setupStatusHistoryTestDB(t)
		todoRepo := NewTodoRepo(db)

		// Create two users to verify user isolation
		userA := model.User{AuthProvider: "test", AuthSubject: "userA", DisplayName: "User A"}
		if err := db.Create(&userA).Error; err != nil {
			rt.Fatalf("create user A: %v", err)
		}
		userB := model.User{AuthProvider: "test", AuthSubject: "userB", DisplayName: "User B"}
		if err := db.Create(&userB).Error; err != nil {
			rt.Fatalf("create user B: %v", err)
		}

		// Generate a random time range [start, end]
		t1 := genTimestamp(rt, "rangeT1")
		t2 := genTimestamp(rt, "rangeT2")
		start, end := t1, t2
		if end.Before(start) {
			start, end = end, start
		}

		categories := []string{"bug", "feature", "task"}

		// Generate todos for user A with random updated_at timestamps
		numTodosA := rapid.IntRange(2, 10).Draw(rt, "numTodosA")
		type todoRecord struct {
			id        uint
			updatedAt time.Time
		}
		todosA := make([]todoRecord, 0, numTodosA)

		for i := range numTodosA {
			updatedAt := genTimestamp(rt, fmt.Sprintf("todoA_updatedAt_%d", i))
			todo := model.Todo{
				UserID:    userA.ID,
				Code:      fmt.Sprintf("A-%d", i+1),
				Title:     fmt.Sprintf("Todo A %d", i+1),
				Category:  categories[i%3],
				Priority:  "p2",
				Status:    "open",
				UpdatedAt: updatedAt,
				CreatedAt: updatedAt,
			}
			if err := db.Create(&todo).Error; err != nil {
				rt.Fatalf("create todo A %d: %v", i, err)
			}
			// Force updated_at to the desired value (GORM may override it)
			if err := db.Exec("UPDATE todos SET updated_at = ? WHERE id = ?", updatedAt, todo.ID).Error; err != nil {
				rt.Fatalf("set updated_at for todo A %d: %v", i, err)
			}
			todosA = append(todosA, todoRecord{id: todo.ID, updatedAt: updatedAt})
		}

		// Generate todos for user B (should never appear in user A's results)
		numTodosB := rapid.IntRange(1, 5).Draw(rt, "numTodosB")
		for i := range numTodosB {
			updatedAt := genTimestamp(rt, fmt.Sprintf("todoB_updatedAt_%d", i))
			// Ensure some of user B's todos fall within the range
			if i == 0 {
				// Force at least one of user B's todos into the range
				mid := start.Add(end.Sub(start) / 2)
				updatedAt = mid
			}
			todo := model.Todo{
				UserID:    userB.ID,
				Code:      fmt.Sprintf("B-%d", i+1),
				Title:     fmt.Sprintf("Todo B %d", i+1),
				Category:  categories[i%3],
				Priority:  "p2",
				Status:    "open",
				UpdatedAt: updatedAt,
				CreatedAt: updatedAt,
			}
			if err := db.Create(&todo).Error; err != nil {
				rt.Fatalf("create todo B %d: %v", i, err)
			}
			if err := db.Exec("UPDATE todos SET updated_at = ? WHERE id = ?", updatedAt, todo.ID).Error; err != nil {
				rt.Fatalf("set updated_at for todo B %d: %v", i, err)
			}
		}

		// Query using the activity filter
		results, err := todoRepo.FindByUserAndUpdatedAtRange(nil, userA.ID, start, end)
		if err != nil {
			rt.Fatalf("FindByUserAndUpdatedAtRange: %v", err)
		}

		// Compute expected set: user A's todos whose updated_at is within [start, end]
		expectedIDs := make(map[uint]bool)
		for _, tr := range todosA {
			if !tr.updatedAt.Before(start) && !tr.updatedAt.After(end) {
				expectedIDs[tr.id] = true
			}
		}

		// Verify results match expected
		resultIDs := make(map[uint]bool)
		for _, todo := range results {
			resultIDs[todo.ID] = true
		}

		// No missing todos
		for id := range expectedIDs {
			if !resultIDs[id] {
				rt.Fatalf("expected todo ID %d in results but it was missing", id)
			}
		}

		// No extra todos
		for id := range resultIDs {
			if !expectedIDs[id] {
				rt.Fatalf("unexpected todo ID %d in results", id)
			}
		}

		// Verify user isolation: no user B todos in results
		for _, todo := range results {
			if todo.UserID != userA.ID {
				rt.Fatalf("result contains todo with user_id %d, expected only user %d", todo.UserID, userA.ID)
			}
		}
	})
}

// Feature: ai-summary-enhancement, Property 5: Status history time-range query correctness
// **Validates: Requirements 2.5**
//
// Property: For any set of status history records and any time range [start, end],
// querying by todo_id and time range SHALL return exactly those records whose changed_at
// falls within [start, end].
func TestProperty_StatusHistoryTimeRangeQueryCorrectness(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		db := setupStatusHistoryTestDB(t)
		historyRepo := NewStatusHistoryRepo(db)

		// Create a user and a todo for the history records
		user := model.User{AuthProvider: "test", AuthSubject: "user1", DisplayName: "User 1"}
		if err := db.Create(&user).Error; err != nil {
			rt.Fatalf("create user: %v", err)
		}
		todo := model.Todo{
			UserID:   user.ID,
			Code:     "T-1",
			Title:    "Test Todo",
			Category: "task",
			Priority: "p2",
			Status:   "open",
		}
		if err := db.Create(&todo).Error; err != nil {
			rt.Fatalf("create todo: %v", err)
		}

		// Also create a second todo to verify todo_id scoping
		todo2 := model.Todo{
			UserID:   user.ID,
			Code:     "T-2",
			Title:    "Other Todo",
			Category: "task",
			Priority: "p2",
			Status:   "open",
		}
		if err := db.Create(&todo2).Error; err != nil {
			rt.Fatalf("create todo2: %v", err)
		}

		statuses := []string{"open", "in_progress", "completed"}

		// Generate random status history records for todo 1
		numRecords := rapid.IntRange(3, 15).Draw(rt, "numRecords")
		type historyRecord struct {
			id        uint
			changedAt time.Time
		}
		records := make([]historyRecord, 0, numRecords)

		for i := range numRecords {
			changedAt := genTimestamp(rt, fmt.Sprintf("changedAt_%d", i))
			oldIdx := rapid.IntRange(0, 2).Draw(rt, fmt.Sprintf("oldStatus_%d", i))
			newIdx := rapid.IntRange(0, 2).Draw(rt, fmt.Sprintf("newStatus_%d", i))
			rec := &model.TodoStatusHistory{
				TodoID:    todo.ID,
				OldStatus: statuses[oldIdx],
				NewStatus: statuses[newIdx],
				ChangedAt: changedAt,
			}
			if err := historyRepo.Create(nil, rec); err != nil {
				rt.Fatalf("create history record %d: %v", i, err)
			}
			records = append(records, historyRecord{id: rec.ID, changedAt: changedAt})
		}

		// Generate some records for todo2 (should not appear in todo1's query)
		numRecords2 := rapid.IntRange(1, 5).Draw(rt, "numRecords2")
		for i := range numRecords2 {
			changedAt := genTimestamp(rt, fmt.Sprintf("todo2_changedAt_%d", i))
			rec := &model.TodoStatusHistory{
				TodoID:    todo2.ID,
				OldStatus: "open",
				NewStatus: "in_progress",
				ChangedAt: changedAt,
			}
			if err := historyRepo.Create(nil, rec); err != nil {
				rt.Fatalf("create history record for todo2 %d: %v", i, err)
			}
		}

		// Generate a random time range [start, end]
		t1 := genTimestamp(rt, "queryT1")
		t2 := genTimestamp(rt, "queryT2")
		start, end := t1, t2
		if end.Before(start) {
			start, end = end, start
		}

		// Query by todo_id and time range
		results, err := historyRepo.FindByTodoIDAndTimeRange(nil, todo.ID, start, end)
		if err != nil {
			rt.Fatalf("FindByTodoIDAndTimeRange: %v", err)
		}

		// Compute expected set: records for todo1 whose changed_at is within [start, end]
		expectedIDs := make(map[uint]bool)
		for _, rec := range records {
			if !rec.changedAt.Before(start) && !rec.changedAt.After(end) {
				expectedIDs[rec.id] = true
			}
		}

		// Verify results match expected
		resultIDs := make(map[uint]bool)
		for _, rec := range results {
			resultIDs[rec.ID] = true
		}

		// No missing records
		for id := range expectedIDs {
			if !resultIDs[id] {
				rt.Fatalf("expected history record ID %d in results but it was missing\nstart=%v end=%v",
					id, start, end)
			}
		}

		// No extra records
		for id := range resultIDs {
			if !expectedIDs[id] {
				rt.Fatalf("unexpected history record ID %d in results\nstart=%v end=%v",
					id, start, end)
			}
		}

		// Verify todo_id scoping: no records from todo2
		for _, rec := range results {
			if rec.TodoID != todo.ID {
				rt.Fatalf("result contains record with todo_id %d, expected only todo %d", rec.TodoID, todo.ID)
			}
		}
	})
}
