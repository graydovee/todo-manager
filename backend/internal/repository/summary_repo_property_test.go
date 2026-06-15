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

// setupSummaryTestDB creates an in-memory SQLite database for summary repository property tests.
func setupSummaryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	id := repoPropertyTestDBCounter.Add(1)
	dsn := fmt.Sprintf("file:summarydb_%d?mode=memory", id)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	sqlDB, _ := db.DB()
	tables := []string{
		`CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY AUTOINCREMENT, auth_provider TEXT NOT NULL DEFAULT '', auth_subject TEXT NOT NULL DEFAULT '', display_name TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS summaries (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, start_date DATE NOT NULL, end_date DATE NOT NULL, status TEXT NOT NULL DEFAULT 'analyzing', result_content TEXT, todo_ids TEXT, language VARCHAR(20) DEFAULT '', custom_prompt TEXT, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY (user_id) REFERENCES users(id))`,
		`CREATE INDEX IF NOT EXISTS idx_summaries_user_id ON summaries(user_id)`,
	}
	for _, ddl := range tables {
		if _, err := sqlDB.Exec(ddl); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	return db
}

// createTestUser inserts a user record and returns the user ID.
func createTestUser(t *testing.T, db *gorm.DB) uint {
	t.Helper()
	user := model.User{
		AuthProvider: "test",
		AuthSubject:  fmt.Sprintf("subject_%d", time.Now().UnixNano()),
		DisplayName:  "Test User",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create test user: %v", err)
	}
	return user.ID
}

// genDate generates a random date between 2020-01-01 and 2024-12-31.
func genDate(rt *rapid.T, label string) time.Time {
	year := rapid.IntRange(2020, 2024).Draw(rt, label+"_year")
	month := rapid.IntRange(1, 12).Draw(rt, label+"_month")
	day := rapid.IntRange(1, 28).Draw(rt, label+"_day") // use 28 to avoid invalid dates
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

// Feature: ai-summary, Property 5: Summary creation persists all fields correctly
// **Validates: Requirements 8.1, 9.1, 9.2**
//
// Property: For any valid summary creation request (valid user_id, start_date, end_date),
// the created database record SHALL contain the correct user_id, start_date, end_date,
// initial status of "analyzing", and valid created_at/updated_at timestamps. Reading the
// record back SHALL return all persisted fields unchanged.
func TestSummaryCreationPersistsAllFields(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		db := setupSummaryTestDB(t)
		repo := NewSummaryRepo(db)
		userID := createTestUser(t, db)

		startDate := genDate(rt, "start")
		endDate := genDate(rt, "end")
		// Ensure end >= start
		if endDate.Before(startDate) {
			startDate, endDate = endDate, startDate
		}

		summary := &model.Summary{
			UserID:    userID,
			StartDate: startDate,
			EndDate:   endDate,
			Status:    model.SummaryStatusAnalyzing,
		}

		err := repo.Create(nil, summary)
		if err != nil {
			rt.Fatalf("Create failed: %v", err)
		}

		// ID should be assigned
		if summary.ID == 0 {
			rt.Fatal("expected non-zero ID after creation")
		}

		// Read back the record
		got, err := repo.FindByID(nil, summary.ID, userID)
		if err != nil {
			rt.Fatalf("FindByID failed: %v", err)
		}

		// Verify all fields
		if got.UserID != userID {
			rt.Fatalf("user_id mismatch: got %d, want %d", got.UserID, userID)
		}
		if !got.StartDate.Equal(startDate) {
			rt.Fatalf("start_date mismatch: got %v, want %v", got.StartDate, startDate)
		}
		if !got.EndDate.Equal(endDate) {
			rt.Fatalf("end_date mismatch: got %v, want %v", got.EndDate, endDate)
		}
		if got.Status != model.SummaryStatusAnalyzing {
			rt.Fatalf("status mismatch: got %q, want %q", got.Status, model.SummaryStatusAnalyzing)
		}
		if got.CreatedAt.IsZero() {
			rt.Fatal("created_at should not be zero")
		}
		if got.UpdatedAt.IsZero() {
			rt.Fatal("updated_at should not be zero")
		}
	})
}

// Feature: ai-summary, Property 6: Summary list returns entries in descending creation order with limit
// **Validates: Requirements 8.3**
//
// Property: For any set of summary entries belonging to a user, the list endpoint SHALL
// return them ordered by created_at descending, and SHALL return at most 50 entries.
func TestSummaryListDescendingOrderWithLimit(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		db := setupSummaryTestDB(t)
		repo := NewSummaryRepo(db)
		userID := createTestUser(t, db)

		// Generate between 1 and 60 summaries to test the limit of 50
		numEntries := rapid.IntRange(1, 60).Draw(rt, "numEntries")

		for i := range numEntries {
			startDate := genDate(rt, fmt.Sprintf("start_%d", i))
			endDate := genDate(rt, fmt.Sprintf("end_%d", i))
			if endDate.Before(startDate) {
				startDate, endDate = endDate, startDate
			}

			summary := &model.Summary{
				UserID:    userID,
				StartDate: startDate,
				EndDate:   endDate,
				Status:    model.SummaryStatusAnalyzing,
			}
			if err := repo.Create(nil, summary); err != nil {
				rt.Fatalf("Create summary %d failed: %v", i, err)
			}
		}

		// List with default limit (0 means default 50)
		results, err := repo.ListByUser(nil, userID, 0)
		if err != nil {
			rt.Fatalf("ListByUser failed: %v", err)
		}

		// Verify limit: at most 50 entries
		if len(results) > 50 {
			rt.Fatalf("expected at most 50 entries, got %d", len(results))
		}

		// Verify expected count
		expectedCount := numEntries
		if expectedCount > 50 {
			expectedCount = 50
		}
		if len(results) != expectedCount {
			rt.Fatalf("expected %d entries, got %d", expectedCount, len(results))
		}

		// Verify descending order by created_at
		for i := 1; i < len(results); i++ {
			if results[i].CreatedAt.After(results[i-1].CreatedAt) {
				rt.Fatalf("entries not in descending order at index %d: %v > %v",
					i, results[i].CreatedAt, results[i-1].CreatedAt)
			}
		}
	})
}

// Feature: ai-summary, Property 7: User isolation — only own entries are accessible
// **Validates: Requirements 8.5, 9.3, 9.5**
//
// Property: For any two distinct users A and B, user A SHALL only see their own summary
// entries when listing, and SHALL receive a not-found error when attempting to view or
// delete a summary belonging to user B.
func TestSummaryUserIsolation(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		db := setupSummaryTestDB(t)
		repo := NewSummaryRepo(db)
		userA := createTestUser(t, db)
		userB := createTestUser(t, db)

		// Create summaries for user A
		numA := rapid.IntRange(1, 5).Draw(rt, "numA")
		userAIDs := make([]uint, 0, numA)
		for i := range numA {
			startDate := genDate(rt, fmt.Sprintf("a_start_%d", i))
			endDate := genDate(rt, fmt.Sprintf("a_end_%d", i))
			if endDate.Before(startDate) {
				startDate, endDate = endDate, startDate
			}
			s := &model.Summary{
				UserID:    userA,
				StartDate: startDate,
				EndDate:   endDate,
				Status:    model.SummaryStatusAnalyzing,
			}
			if err := repo.Create(nil, s); err != nil {
				rt.Fatalf("Create for user A failed: %v", err)
			}
			userAIDs = append(userAIDs, s.ID)
		}

		// Create summaries for user B
		numB := rapid.IntRange(1, 5).Draw(rt, "numB")
		userBIDs := make([]uint, 0, numB)
		for i := range numB {
			startDate := genDate(rt, fmt.Sprintf("b_start_%d", i))
			endDate := genDate(rt, fmt.Sprintf("b_end_%d", i))
			if endDate.Before(startDate) {
				startDate, endDate = endDate, startDate
			}
			s := &model.Summary{
				UserID:    userB,
				StartDate: startDate,
				EndDate:   endDate,
				Status:    model.SummaryStatusAnalyzing,
			}
			if err := repo.Create(nil, s); err != nil {
				rt.Fatalf("Create for user B failed: %v", err)
			}
			userBIDs = append(userBIDs, s.ID)
		}

		// User A listing should only contain user A's entries
		listA, err := repo.ListByUser(nil, userA, 50)
		if err != nil {
			rt.Fatalf("ListByUser for A failed: %v", err)
		}
		if len(listA) != numA {
			rt.Fatalf("user A list: expected %d entries, got %d", numA, len(listA))
		}
		for _, s := range listA {
			if s.UserID != userA {
				rt.Fatalf("user A list contains entry with user_id %d", s.UserID)
			}
		}

		// User B listing should only contain user B's entries
		listB, err := repo.ListByUser(nil, userB, 50)
		if err != nil {
			rt.Fatalf("ListByUser for B failed: %v", err)
		}
		if len(listB) != numB {
			rt.Fatalf("user B list: expected %d entries, got %d", numB, len(listB))
		}
		for _, s := range listB {
			if s.UserID != userB {
				rt.Fatalf("user B list contains entry with user_id %d", s.UserID)
			}
		}

		// User A cannot view user B's entries
		for _, bID := range userBIDs {
			_, err := repo.FindByID(nil, bID, userA)
			if err == nil {
				rt.Fatalf("user A should not be able to view user B's summary %d", bID)
			}
		}

		// User B cannot view user A's entries
		for _, aID := range userAIDs {
			_, err := repo.FindByID(nil, aID, userB)
			if err == nil {
				rt.Fatalf("user B should not be able to view user A's summary %d", aID)
			}
		}

		// User A cannot delete user B's entries (delete should not affect the record)
		for _, bID := range userBIDs {
			_ = repo.Delete(nil, bID, userA)
			// Verify user B can still see it
			got, err := repo.FindByID(nil, bID, userB)
			if err != nil {
				rt.Fatalf("user B's summary %d was deleted by user A: %v", bID, err)
			}
			if got.ID != bID {
				rt.Fatalf("unexpected ID after delete attempt: got %d, want %d", got.ID, bID)
			}
		}
	})
}

// Feature: ai-summary, Property 9: Delete permanently removes the record
// **Validates: Requirements 9.4**
//
// Property: For any existing summary entry, after deletion, attempting to retrieve it
// by ID SHALL return a not-found error, and it SHALL not appear in the user's list.
func TestSummaryDeletePermanentlyRemoves(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		db := setupSummaryTestDB(t)
		repo := NewSummaryRepo(db)
		userID := createTestUser(t, db)

		// Create some summaries
		numEntries := rapid.IntRange(1, 10).Draw(rt, "numEntries")
		ids := make([]uint, 0, numEntries)
		for i := range numEntries {
			startDate := genDate(rt, fmt.Sprintf("start_%d", i))
			endDate := genDate(rt, fmt.Sprintf("end_%d", i))
			if endDate.Before(startDate) {
				startDate, endDate = endDate, startDate
			}
			s := &model.Summary{
				UserID:    userID,
				StartDate: startDate,
				EndDate:   endDate,
				Status:    model.SummaryStatusCompleted,
			}
			if err := repo.Create(nil, s); err != nil {
				rt.Fatalf("Create summary %d failed: %v", i, err)
			}
			ids = append(ids, s.ID)
		}

		// Pick a random entry to delete
		deleteIdx := rapid.IntRange(0, len(ids)-1).Draw(rt, "deleteIdx")
		deleteID := ids[deleteIdx]

		// Delete the entry
		err := repo.Delete(nil, deleteID, userID)
		if err != nil {
			rt.Fatalf("Delete failed: %v", err)
		}

		// Attempting to retrieve by ID should return an error
		_, err = repo.FindByID(nil, deleteID, userID)
		if err == nil {
			rt.Fatalf("expected not-found error after deleting summary %d, but got nil", deleteID)
		}

		// The deleted entry should not appear in the user's list
		list, err := repo.ListByUser(nil, userID, 50)
		if err != nil {
			rt.Fatalf("ListByUser failed: %v", err)
		}
		for _, s := range list {
			if s.ID == deleteID {
				rt.Fatalf("deleted summary %d still appears in user's list", deleteID)
			}
		}

		// Remaining entries should still be accessible
		expectedRemaining := numEntries - 1
		if len(list) != expectedRemaining {
			rt.Fatalf("expected %d remaining entries, got %d", expectedRemaining, len(list))
		}
	})
}
