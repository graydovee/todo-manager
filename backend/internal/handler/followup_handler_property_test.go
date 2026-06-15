package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/graydovee/todo-manager/internal/config"
	"github.com/graydovee/todo-manager/internal/middleware"
	"github.com/graydovee/todo-manager/internal/model"
	"github.com/graydovee/todo-manager/internal/repository"
	"github.com/graydovee/todo-manager/internal/service"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"pgregory.net/rapid"
)

var followupHandlerTestDBCounter atomic.Int64

// setupFollowupHandlerTestDB creates an in-memory SQLite database for handler property tests.
func setupFollowupHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	id := followupHandlerTestDBCounter.Add(1)
	dsn := fmt.Sprintf("file:followuphandlerdb_%d?mode=memory&cache=shared", id)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)

	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("failed to enable foreign keys: %v", err)
	}

	tables := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			auth_provider TEXT NOT NULL DEFAULT '',
			auth_subject TEXT NOT NULL DEFAULT '',
			display_name TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS summaries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			start_date DATE NOT NULL,
			end_date DATE NOT NULL,
			status TEXT NOT NULL DEFAULT 'analyzing',
			result_content TEXT,
			todo_ids TEXT,
			language VARCHAR(20) DEFAULT '',
			custom_prompt TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_summaries_user_id ON summaries(user_id)`,
		`CREATE TABLE IF NOT EXISTS followup_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			summary_id INTEGER NOT NULL,
			question TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (summary_id) REFERENCES summaries(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_followup_messages_summary_id ON followup_messages(summary_id)`,
		`CREATE TABLE IF NOT EXISTS followup_message_versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			followup_message_id INTEGER NOT NULL,
			content TEXT NOT NULL,
			version_number INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (followup_message_id) REFERENCES followup_messages(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_fmv_followup_message_id ON followup_message_versions(followup_message_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_fmv_message_version ON followup_message_versions(followup_message_id, version_number)`,
	}
	for _, ddl := range tables {
		if _, err := sqlDB.Exec(ddl); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	return db
}

// setupFollowupHandlerTest creates a test Echo instance and FollowupHandler
// backed by a real in-memory SQLite database with a completed summary for user 1.
func setupFollowupHandlerTest(t *testing.T) (*echo.Echo, *FollowupHandler) {
	t.Helper()

	db := setupFollowupHandlerTestDB(t)

	// Create a user
	user := model.User{
		AuthProvider: "test",
		AuthSubject:  fmt.Sprintf("subject_%d", time.Now().UnixNano()),
		DisplayName:  "Test User",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create a completed summary for user 1
	summary := &model.Summary{
		UserID:        user.ID,
		StartDate:     time.Now().AddDate(0, 0, -30),
		EndDate:       time.Now().AddDate(0, 0, -1),
		Status:        model.SummaryStatusCompleted,
		ResultContent: "Test summary content for followup handler testing.",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := db.Create(summary).Error; err != nil {
		t.Fatalf("failed to create test summary: %v", err)
	}

	// Set up repos and service
	summaryRepo := repository.NewSummaryRepo(db)
	followupRepo := repository.NewFollowupRepo(db)

	llmCfg := &config.LLMConfig{
		Model:   "test-model",
		BaseURL: "http://localhost:9999",
		APIKey:  "test-key",
		Timeout: 5,
	}

	followupService := service.NewFollowupService(db, followupRepo, summaryRepo, nil, llmCfg)

	e := echo.New()
	h := NewFollowupHandler(followupService, followupRepo)

	return e, h
}

// Feature: ai-summary-followup, Property 2: Custom prompt length validation
// **Validates: Requirements 1.7, 2.2**
//
// Property: For any string exceeding 500 characters provided as custom_prompt
// in a create summary request, the backend SHALL reject the request with an
// HTTP 400 response.
func TestProperty_CustomPromptLengthValidation(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a string that exceeds 500 characters
		length := rapid.IntRange(501, 2000).Draw(rt, "length")

		// Build a long prompt of the exact length
		longPrompt := strings.Repeat("a", length)

		// Build a JSON request body with valid dates and the oversized custom_prompt
		reqBody := fmt.Sprintf(`{"start_date":"2024-01-01","end_date":"2024-01-31","custom_prompt":%s}`,
			jsonMarshalStr(longPrompt))

		// Create an HTTP request
		req := httptest.NewRequest(http.MethodPost, "/api/v1/summaries", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Set up Echo context with a mock user
		e := echo.New()
		c := e.NewContext(req, rec)
		c.Set(middleware.UserContextKey, &model.User{ID: 1})

		// Create handler with nil service (should not be reached due to validation)
		h := &SummaryHandler{summaryService: nil}

		// Call the handler
		err := h.Create(c)
		if err != nil {
			rt.Fatalf("handler returned error: %v", err)
		}

		// Verify HTTP 400 status
		if rec.Code != http.StatusBadRequest {
			rt.Fatalf("expected status 400 for custom_prompt of length %d, got %d\nbody: %s",
				length, rec.Code, rec.Body.String())
		}

		// Verify error message
		body := rec.Body.String()
		expectedMsg := "custom prompt exceeds maximum length of 500 characters"
		if !strings.Contains(body, expectedMsg) {
			rt.Fatalf("expected error message %q in response body, got: %s",
				expectedMsg, body)
		}
	})
}

// Feature: ai-summary-followup, Property 6: Followup input validation
// **Validates: Requirements 5.7, 6.1, 6.5, 6.7, 6.9**
//
// Property: For any question string that is empty, contains only whitespace,
// or exceeds 1000 characters, the followup endpoint SHALL reject with HTTP 400.
// For any context_messages array exceeding 20 items or containing entries with
// invalid roles (not "user" or "assistant") or content exceeding 2000 characters,
// the endpoint SHALL reject with HTTP 400.

// TestProperty_FollowupEmptyQuestion tests that empty or whitespace-only questions
// are rejected with HTTP 400.
func TestProperty_FollowupEmptyQuestion(t *testing.T) {
	e, h := setupFollowupHandlerTest(t)

	rapid.Check(t, func(rt *rapid.T) {
		// Generate empty or whitespace-only strings
		emptyQuestion := rapid.OneOf(
			rapid.Just(""),
			rapid.Just(" "),
			rapid.Just("  "),
			rapid.Just("\t"),
			rapid.Just("\n"),
			rapid.Just("\t\n  \t"),
			rapid.StringMatching(`[ \t\n\r]{1,50}`),
		).Draw(rt, "emptyQuestion")

		reqBody := fmt.Sprintf(`{"question":%s}`, jsonMarshalStr(emptyQuestion))

		req := httptest.NewRequest(http.MethodPost, "/api/v1/summaries/1/followup", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("1")
		c.Set(middleware.UserContextKey, &model.User{ID: 1})

		err := h.Followup(c)
		if err != nil {
			rt.Fatalf("handler returned error: %v", err)
		}

		if rec.Code != http.StatusBadRequest {
			rt.Fatalf("expected status 400 for empty/whitespace question %q, got %d\nbody: %s",
				emptyQuestion, rec.Code, rec.Body.String())
		}

		body := rec.Body.String()
		expectedMsg := "a non-empty question is required"
		if !strings.Contains(body, expectedMsg) {
			rt.Fatalf("expected error message %q in response body, got: %s",
				expectedMsg, body)
		}
	})
}

// TestProperty_FollowupQuestionTooLong tests that questions exceeding 1000
// characters are rejected with HTTP 400.
func TestProperty_FollowupQuestionTooLong(t *testing.T) {
	e, h := setupFollowupHandlerTest(t)

	rapid.Check(t, func(rt *rapid.T) {
		// Generate a question that exceeds 1000 characters
		length := rapid.IntRange(1001, 3000).Draw(rt, "length")
		longQuestion := strings.Repeat("q", length)

		reqBody := fmt.Sprintf(`{"question":%s}`, jsonMarshalStr(longQuestion))

		req := httptest.NewRequest(http.MethodPost, "/api/v1/summaries/1/followup", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("1")
		c.Set(middleware.UserContextKey, &model.User{ID: 1})

		err := h.Followup(c)
		if err != nil {
			rt.Fatalf("handler returned error: %v", err)
		}

		if rec.Code != http.StatusBadRequest {
			rt.Fatalf("expected status 400 for question of length %d, got %d\nbody: %s",
				length, rec.Code, rec.Body.String())
		}

		body := rec.Body.String()
		expectedMsg := "question exceeds maximum length of 1000 characters"
		if !strings.Contains(body, expectedMsg) {
			rt.Fatalf("expected error message %q in response body, got: %s",
				expectedMsg, body)
		}
	})
}

// TestProperty_FollowupContextMessagesTooMany tests that context_messages arrays
// exceeding 20 items are rejected with HTTP 400.
func TestProperty_FollowupContextMessagesTooMany(t *testing.T) {
	e, h := setupFollowupHandlerTest(t)

	rapid.Check(t, func(rt *rapid.T) {
		// Generate context_messages with more than 20 items
		count := rapid.IntRange(21, 50).Draw(rt, "count")

		messages := make([]service.ContextMessage, count)
		for i := range count {
			role := "user"
			if i%2 == 1 {
				role = "assistant"
			}
			messages[i] = service.ContextMessage{
				Role:    role,
				Content: "hello",
			}
		}

		reqObj := FollowupRequest{
			Question:        "valid question here",
			ContextMessages: messages,
		}
		reqBytes, _ := json.Marshal(reqObj)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/summaries/1/followup", strings.NewReader(string(reqBytes)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("1")
		c.Set(middleware.UserContextKey, &model.User{ID: 1})

		err := h.Followup(c)
		if err != nil {
			rt.Fatalf("handler returned error: %v", err)
		}

		if rec.Code != http.StatusBadRequest {
			rt.Fatalf("expected status 400 for %d context_messages, got %d\nbody: %s",
				count, rec.Code, rec.Body.String())
		}

		body := rec.Body.String()
		expectedMsg := "context_messages exceeds maximum of 20 items"
		if !strings.Contains(body, expectedMsg) {
			rt.Fatalf("expected error message %q in response body, got: %s",
				expectedMsg, body)
		}
	})
}

// TestProperty_FollowupContextMessagesInvalidRole tests that context_messages
// with invalid roles are rejected with HTTP 400.
func TestProperty_FollowupContextMessagesInvalidRole(t *testing.T) {
	e, h := setupFollowupHandlerTest(t)

	rapid.Check(t, func(rt *rapid.T) {
		// Generate an invalid role (not "user" or "assistant")
		invalidRole := rapid.OneOf(
			rapid.StringMatching(`[a-zA-Z]{1,20}`),
			rapid.SampledFrom([]string{
				"system", "admin", "bot", "User", "Assistant",
				"ASSISTANT", "USER", "moderator", "tool",
				"function", "developer", "operator",
			}),
		).Draw(rt, "invalidRole")

		// Filter out valid roles
		if invalidRole == "user" || invalidRole == "assistant" {
			rt.Skip("generated a valid role, skipping")
		}

		messages := []service.ContextMessage{
			{Role: invalidRole, Content: "some content"},
		}

		reqObj := FollowupRequest{
			Question:        "valid question here",
			ContextMessages: messages,
		}
		reqBytes, _ := json.Marshal(reqObj)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/summaries/1/followup", strings.NewReader(string(reqBytes)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("1")
		c.Set(middleware.UserContextKey, &model.User{ID: 1})

		err := h.Followup(c)
		if err != nil {
			rt.Fatalf("handler returned error: %v", err)
		}

		if rec.Code != http.StatusBadRequest {
			rt.Fatalf("expected status 400 for invalid role %q, got %d\nbody: %s",
				invalidRole, rec.Code, rec.Body.String())
		}

		body := rec.Body.String()
		expectedMsg := "context_messages role must be 'user' or 'assistant'"
		if !strings.Contains(body, expectedMsg) {
			rt.Fatalf("expected error message %q in response body, got: %s",
				expectedMsg, body)
		}
	})
}

// TestProperty_FollowupContextMessagesContentTooLong tests that context_messages
// with content exceeding 2000 characters are rejected with HTTP 400.
func TestProperty_FollowupContextMessagesContentTooLong(t *testing.T) {
	e, h := setupFollowupHandlerTest(t)

	rapid.Check(t, func(rt *rapid.T) {
		// Generate content that exceeds 2000 characters
		length := rapid.IntRange(2001, 5000).Draw(rt, "length")
		longContent := strings.Repeat("c", length)

		messages := []service.ContextMessage{
			{Role: "user", Content: longContent},
		}

		reqObj := FollowupRequest{
			Question:        "valid question here",
			ContextMessages: messages,
		}
		reqBytes, _ := json.Marshal(reqObj)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/summaries/1/followup", strings.NewReader(string(reqBytes)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("1")
		c.Set(middleware.UserContextKey, &model.User{ID: 1})

		err := h.Followup(c)
		if err != nil {
			rt.Fatalf("handler returned error: %v", err)
		}

		if rec.Code != http.StatusBadRequest {
			rt.Fatalf("expected status 400 for content of length %d, got %d\nbody: %s",
				length, rec.Code, rec.Body.String())
		}

		body := rec.Body.String()
		expectedMsg := "context_messages content exceeds maximum length of 2000 characters"
		if !strings.Contains(body, expectedMsg) {
			rt.Fatalf("expected error message %q in response body, got: %s",
				expectedMsg, body)
		}
	})
}

// jsonMarshalStr marshals a string to JSON (with proper escaping).
func jsonMarshalStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
