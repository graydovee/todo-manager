package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/graydovee/todolist/internal/config"
	"github.com/graydovee/todolist/internal/model"
	"github.com/graydovee/todolist/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"pgregory.net/rapid"
)

var servicePropertyTestDBCounter atomic.Int64

// setupServiceTestDB creates an in-memory SQLite database for service property tests.
func setupServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	id := servicePropertyTestDBCounter.Add(1)
	dsn := fmt.Sprintf("file:servicedb_%d?mode=memory&cache=shared", id)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	sqlDB, _ := db.DB()
	// Ensure connections are reused and the shared cache is accessible from all goroutines
	sqlDB.SetMaxOpenConns(1)
	tables := []string{
		`CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY AUTOINCREMENT, auth_provider TEXT NOT NULL DEFAULT '', auth_subject TEXT NOT NULL DEFAULT '', display_name TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS todos (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, code TEXT NOT NULL, title TEXT NOT NULL, description TEXT DEFAULT '', category TEXT NOT NULL CHECK(category IN ('bug','feature','task')), priority TEXT NOT NULL DEFAULT 'p2' CHECK(priority IN ('p0','p1','p2','p3')), status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open','in_progress','completed')), due_at DATETIME, pinned INTEGER NOT NULL DEFAULT 0, highlighted INTEGER NOT NULL DEFAULT 0, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, UNIQUE(user_id, code))`,
		`CREATE TABLE IF NOT EXISTS summaries (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, start_date DATE NOT NULL, end_date DATE NOT NULL, status TEXT NOT NULL DEFAULT 'analyzing', result_content TEXT, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY (user_id) REFERENCES users(id))`,
		`CREATE INDEX IF NOT EXISTS idx_summaries_user_id ON summaries(user_id)`,
	}
	for _, ddl := range tables {
		if _, err := sqlDB.Exec(ddl); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	return db
}

// createServiceTestUser inserts a user record and returns the user ID.
func createServiceTestUser(t *testing.T, db *gorm.DB) uint {
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

// Feature: ai-summary, Property 1: Date range validation rejects invalid inputs
// **Validates: Requirements 2.4, 2.5, 8.2**
//
// Property: For any pair of dates where end_date is earlier than start_date, OR where
// either date is in the future (after the current date), the summary creation service
// SHALL reject the input with a validation error and not create a database record.
func TestProperty_DateRangeValidationRejectsInvalidInputs(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		db := setupServiceTestDB(t)
		summaryRepo := repository.NewSummaryRepo(db)
		todoRepo := repository.NewTodoRepo(db)
		userID := createServiceTestUser(t, db)

		// Create a valid LLM config so validation doesn't fail on config
		llmCfg := &config.LLMConfig{
			Model:   "test-model",
			BaseURL: "http://localhost:9999",
			APIKey:  "test-key",
			Timeout: 5,
		}
		llmClient := NewLLMClient(llmCfg)

		svc := NewSummaryService(db, summaryRepo, todoRepo, llmClient, llmCfg)

		// Choose which invalid scenario to test
		scenario := rapid.IntRange(0, 2).Draw(rt, "scenario")

		var startDate, endDate time.Time
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		switch scenario {
		case 0:
			// end_date is earlier than start_date (both in the past)
			// Generate a start date in the past
			daysAgoStart := rapid.IntRange(1, 365).Draw(rt, "daysAgoStart")
			startDate = today.AddDate(0, 0, -daysAgoStart)
			// end_date must be before start_date
			daysBeforeStart := rapid.IntRange(1, 365).Draw(rt, "daysBeforeStart")
			endDate = startDate.AddDate(0, 0, -daysBeforeStart)

		case 1:
			// start_date is in the future
			daysInFuture := rapid.IntRange(1, 365).Draw(rt, "daysInFutureStart")
			startDate = today.AddDate(0, 0, daysInFuture)
			// end_date can be anything >= start_date (also in future)
			extraDays := rapid.IntRange(0, 30).Draw(rt, "extraDays")
			endDate = startDate.AddDate(0, 0, extraDays)

		case 2:
			// end_date is in the future (start_date in the past)
			daysAgo := rapid.IntRange(1, 365).Draw(rt, "daysAgoForStart")
			startDate = today.AddDate(0, 0, -daysAgo)
			daysInFuture := rapid.IntRange(1, 365).Draw(rt, "daysInFutureEnd")
			endDate = today.AddDate(0, 0, daysInFuture)
		}

		// Call CreateSummary - should return an error
		result, err := svc.CreateSummary(userID, startDate, endDate)
		if err == nil {
			rt.Fatalf("expected validation error for scenario %d (start=%v, end=%v), got nil with result: %+v",
				scenario, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"), result)
		}

		// Verify no database record was created
		var count int64
		db.Model(&model.Summary{}).Where("user_id = ?", userID).Count(&count)
		if count != 0 {
			rt.Fatalf("expected no summary records for scenario %d, but found %d", scenario, count)
		}
	})
}

// Feature: ai-summary, Property 8: Analysis completion updates status and content
// **Validates: Requirements 8.8, 8.9**
//
// Property: For any summary in "analyzing" status, when the background analysis
// completes successfully, the status SHALL be updated to "completed" and result_content
// SHALL contain the LLM response. When the analysis fails, the status SHALL be updated
// to "error" and result_content SHALL contain an error description.
func TestProperty_AnalysisCompletionUpdatesStatusAndContent(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		db := setupServiceTestDB(t)
		summaryRepo := repository.NewSummaryRepo(db)
		todoRepo := repository.NewTodoRepo(db)
		userID := createServiceTestUser(t, db)

		// Decide if LLM should succeed or fail
		shouldSucceed := rapid.Bool().Draw(rt, "shouldSucceed")

		// Generate a random LLM response content
		llmResponse := rapid.StringMatching(`[a-zA-Z0-9 ]{10,100}`).Draw(rt, "llmResponse")

		// Generate a random error status code for failure case
		errorStatusCode := rapid.SampledFrom([]int{
			http.StatusBadRequest,
			http.StatusUnauthorized,
			http.StatusForbidden,
			http.StatusInternalServerError,
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
		}).Draw(rt, "errorStatusCode")

		// Create a mock LLM server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if shouldSucceed {
				resp := ChatCompletionResponse{
					Choices: []struct {
						Message ChatMessage `json:"message"`
					}{
						{Message: ChatMessage{Role: "assistant", Content: llmResponse}},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(resp)
			} else {
				w.WriteHeader(errorStatusCode)
				w.Write([]byte(`{"error": "something went wrong"}`))
			}
		}))
		defer server.Close()

		llmCfg := &config.LLMConfig{
			Model:   "test-model",
			BaseURL: server.URL,
			APIKey:  "test-api-key",
			Timeout: 10,
		}
		llmClient := NewLLMClient(llmCfg)

		svc := NewSummaryService(db, summaryRepo, todoRepo, llmClient, llmCfg)

		// Use a past date range so validation passes
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		startDate := today.AddDate(0, 0, -30)
		endDate := today.AddDate(0, 0, -1)

		// Create a todo in the date range so the prompt has content
		todo := &model.Todo{
			UserID:    userID,
			Code:      fmt.Sprintf("T-%d", time.Now().UnixNano()),
			Title:     "Test Todo",
			Category:  "task",
			Priority:  "p2",
			Status:    "open",
			CreatedAt: startDate.Add(24 * time.Hour),
			UpdatedAt: startDate.Add(24 * time.Hour),
		}
		if err := db.Create(todo).Error; err != nil {
			rt.Fatalf("failed to create test todo: %v", err)
		}

		// Call CreateSummary - this spawns the background goroutine
		summary, err := svc.CreateSummary(userID, startDate, endDate)
		if err != nil {
			rt.Fatalf("CreateSummary failed: %v", err)
		}

		// The initial status should be "analyzing"
		if summary.Status != model.SummaryStatusAnalyzing {
			rt.Fatalf("expected initial status %q, got %q", model.SummaryStatusAnalyzing, summary.Status)
		}

		// Wait for the background goroutine to complete
		// Poll the database until status changes from "analyzing"
		var updatedSummary *model.Summary
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			updatedSummary, err = summaryRepo.FindByID(nil, summary.ID, userID)
			if err != nil {
				rt.Fatalf("FindByID failed: %v", err)
			}
			if updatedSummary.Status != model.SummaryStatusAnalyzing {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}

		if updatedSummary.Status == model.SummaryStatusAnalyzing {
			rt.Fatalf("summary status still 'analyzing' after timeout")
		}

		if shouldSucceed {
			// Status should be "completed" and result_content should contain the LLM response
			if updatedSummary.Status != model.SummaryStatusCompleted {
				rt.Fatalf("expected status %q on success, got %q", model.SummaryStatusCompleted, updatedSummary.Status)
			}
			if updatedSummary.ResultContent != llmResponse {
				rt.Fatalf("expected result_content to be %q, got %q", llmResponse, updatedSummary.ResultContent)
			}
		} else {
			// Status should be "error" and result_content should contain an error description
			if updatedSummary.Status != model.SummaryStatusError {
				rt.Fatalf("expected status %q on failure, got %q", model.SummaryStatusError, updatedSummary.Status)
			}
			if updatedSummary.ResultContent == "" {
				rt.Fatalf("expected non-empty result_content on error")
			}
			// The error description should mention the failure
			if !containsAny(updatedSummary.ResultContent, "failed", "error", "LLM") {
				rt.Fatalf("error result_content should describe the failure, got: %q", updatedSummary.ResultContent)
			}
		}
	})
}

// containsAny checks if the string contains any of the given substrings (case-insensitive).
func containsAny(s string, substrs ...string) bool {
	lower := fmt.Sprintf("%s", s)
	for _, sub := range substrs {
		if len(sub) > 0 && contains(lower, sub) {
			return true
		}
	}
	return false
}

// contains checks if s contains substr (case-insensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsCI(s, substr))
}

// containsCI performs a case-insensitive contains check.
func containsCI(s, substr string) bool {
	s = fmt.Sprintf("%s", s)
	substr = fmt.Sprintf("%s", substr)
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := range len(substr) {
			sc := s[i+j]
			tc := substr[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if tc >= 'A' && tc <= 'Z' {
				tc += 32
			}
			if sc != tc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
