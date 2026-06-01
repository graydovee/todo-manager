package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

// chatCompletionRequest is the request body for the chat completions endpoint (test helper).
type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

// chatCompletionResponse is the response body from the chat completions endpoint (test helper).
type chatCompletionResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
}

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
		`CREATE TABLE IF NOT EXISTS todos (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, code TEXT NOT NULL, title TEXT NOT NULL, description TEXT DEFAULT '', category TEXT NOT NULL CHECK(category IN ('bug','feature','task')), priority TEXT NOT NULL DEFAULT 'p2' CHECK(priority IN ('p0','p1','p2','p3')), status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open','in_progress','completed','duplicate')), due_at DATETIME, pinned INTEGER NOT NULL DEFAULT 0, highlighted INTEGER NOT NULL DEFAULT 0, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, UNIQUE(user_id, code))`,
		`CREATE TABLE IF NOT EXISTS todo_tags (id INTEGER PRIMARY KEY AUTOINCREMENT, todo_id INTEGER NOT NULL, tag TEXT NOT NULL, UNIQUE(todo_id, tag))`,
		`CREATE TABLE IF NOT EXISTS todo_relations (id INTEGER PRIMARY KEY AUTOINCREMENT, source_id INTEGER NOT NULL, target_id INTEGER NOT NULL, relation_type TEXT NOT NULL CHECK(relation_type IN ('depends_on','duplicate_of')), UNIQUE(source_id, target_id, relation_type))`,
		`CREATE TABLE IF NOT EXISTS comments (id INTEGER PRIMARY KEY AUTOINCREMENT, todo_id INTEGER NOT NULL, user_id INTEGER NOT NULL, content TEXT NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS todo_status_history (id INTEGER PRIMARY KEY AUTOINCREMENT, todo_id INTEGER NOT NULL, old_status TEXT NOT NULL, new_status TEXT NOT NULL, changed_at DATETIME NOT NULL)`,
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

		commentRepo := repository.NewCommentRepo(db)
		relationRepo := repository.NewRelationRepo(db)
		statusHistoryRepo := repository.NewStatusHistoryRepo(db)
		svc := NewSummaryService(db, summaryRepo, todoRepo, commentRepo, relationRepo, statusHistoryRepo, llmClient, llmCfg)

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
				resp := chatCompletionResponse{
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

		commentRepo := repository.NewCommentRepo(db)
		relationRepo := repository.NewRelationRepo(db)
		statusHistoryRepo := repository.NewStatusHistoryRepo(db)
		svc := NewSummaryService(db, summaryRepo, todoRepo, commentRepo, relationRepo, statusHistoryRepo, llmClient, llmCfg)

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

// Feature: ai-summary-streaming, Property 6: Todo ID validation and error reporting
// **Validates: Requirements 6.3, 6.4**
//
// Property: For any set of todo IDs submitted in a create-summary request, if any ID
// does not exist or does not belong to the authenticated user, the service SHALL return
// an error that identifies the specific invalid IDs.
func TestProperty_TodoIDValidationAndErrorReporting(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		db := setupServiceTestDB(t)
		summaryRepo := repository.NewSummaryRepo(db)
		todoRepo := repository.NewTodoRepo(db)
		commentRepo := repository.NewCommentRepo(db)
		relationRepo := repository.NewRelationRepo(db)
		statusHistoryRepo := repository.NewStatusHistoryRepo(db)

		// Create two users: the authenticated user and another user
		userID := createServiceTestUser(t, db)
		otherUserID := createServiceTestUser(t, db)

		// Create a valid LLM config so validation doesn't fail on config
		llmCfg := &config.LLMConfig{
			Model:   "test-model",
			BaseURL: "http://localhost:9999",
			APIKey:  "test-key",
			Timeout: 5,
		}
		llmClient := NewLLMClient(llmCfg)
		svc := NewSummaryService(db, summaryRepo, todoRepo, commentRepo, relationRepo, statusHistoryRepo, llmClient, llmCfg)

		// Create some todos belonging to the authenticated user
		numUserTodos := rapid.IntRange(1, 5).Draw(rt, "numUserTodos")
		validIDs := make([]uint, numUserTodos)
		for i := range numUserTodos {
			todo := &model.Todo{
				UserID:    userID,
				Code:      fmt.Sprintf("T-%d-%d", time.Now().UnixNano(), i),
				Title:     fmt.Sprintf("User Todo %d", i),
				Category:  "task",
				Priority:  "p2",
				Status:    "open",
				CreatedAt: time.Now().AddDate(0, 0, -10),
				UpdatedAt: time.Now().AddDate(0, 0, -10),
			}
			if err := db.Create(todo).Error; err != nil {
				rt.Fatalf("failed to create user todo: %v", err)
			}
			validIDs[i] = todo.ID
		}

		// Create some todos belonging to another user
		numOtherTodos := rapid.IntRange(1, 3).Draw(rt, "numOtherTodos")
		otherUserIDs := make([]uint, numOtherTodos)
		for i := range numOtherTodos {
			todo := &model.Todo{
				UserID:    otherUserID,
				Code:      fmt.Sprintf("O-%d-%d", time.Now().UnixNano(), i),
				Title:     fmt.Sprintf("Other User Todo %d", i),
				Category:  "task",
				Priority:  "p2",
				Status:    "open",
				CreatedAt: time.Now().AddDate(0, 0, -10),
				UpdatedAt: time.Now().AddDate(0, 0, -10),
			}
			if err := db.Create(todo).Error; err != nil {
				rt.Fatalf("failed to create other user todo: %v", err)
			}
			otherUserIDs[i] = todo.ID
		}

		// Generate a set of IDs to submit: mix of valid, other-user, and non-existent
		includeInvalid := rapid.Bool().Draw(rt, "includeInvalid")

		// Pick a subset of valid IDs
		numValidPicked := rapid.IntRange(0, len(validIDs)).Draw(rt, "numValidPicked")
		pickedValid := make([]uint, 0, numValidPicked)
		// Use a shuffled copy of validIDs and take the first numValidPicked
		shuffled := rapid.Permutation(validIDs).Draw(rt, "validPerm")
		for i := range numValidPicked {
			pickedValid = append(pickedValid, shuffled[i])
		}

		var submittedIDs []uint
		var expectedInvalidIDs []uint

		if includeInvalid {
			// Add some invalid IDs: either from other user or non-existent
			numInvalid := rapid.IntRange(1, 4).Draw(rt, "numInvalid")
			for i := range numInvalid {
				invalidType := rapid.IntRange(0, 1).Draw(rt, fmt.Sprintf("invalidType_%d", i))
				switch invalidType {
				case 0:
					// ID belonging to another user
					idx := rapid.IntRange(0, len(otherUserIDs)-1).Draw(rt, fmt.Sprintf("otherIdx_%d", i))
					expectedInvalidIDs = append(expectedInvalidIDs, otherUserIDs[idx])
				case 1:
					// Non-existent ID (use a large number unlikely to exist)
					nonExistentID := uint(rapid.IntRange(10000, 99999).Draw(rt, fmt.Sprintf("nonExistentID_%d", i)))
					expectedInvalidIDs = append(expectedInvalidIDs, nonExistentID)
				}
			}
			submittedIDs = append(pickedValid, expectedInvalidIDs...)
		} else {
			// All valid IDs only
			submittedIDs = pickedValid
		}

		// If submittedIDs is empty, ensure at least one ID is present
		if len(submittedIDs) == 0 {
			if includeInvalid {
				nonExistentID := uint(rapid.IntRange(10000, 99999).Draw(rt, "fallbackNonExistent"))
				submittedIDs = []uint{nonExistentID}
				expectedInvalidIDs = []uint{nonExistentID}
			} else {
				// Pick at least one valid ID
				submittedIDs = []uint{validIDs[0]}
			}
		}

		// Use a past date range so date validation passes
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		startDate := today.AddDate(0, 0, -30)
		endDate := today.AddDate(0, 0, -1)

		// Call CreateSummaryWithTodos
		result, err := svc.CreateSummaryWithTodos(userID, startDate, endDate, submittedIDs, "", "")

		if includeInvalid {
			// Should return an error
			if err == nil {
				rt.Fatalf("expected error for invalid IDs %v, got nil with result: %+v", expectedInvalidIDs, result)
			}

			// The error message should contain each invalid ID
			errMsg := err.Error()
			for _, invalidID := range expectedInvalidIDs {
				idStr := fmt.Sprintf("%d", invalidID)
				if !strings.Contains(errMsg, idStr) {
					rt.Fatalf("error message %q does not contain invalid ID %s", errMsg, idStr)
				}
			}

			// No summary record should be created
			var count int64
			db.Model(&model.Summary{}).Where("user_id = ?", userID).Count(&count)
			if count != 0 {
				rt.Fatalf("expected no summary records when IDs are invalid, but found %d", count)
			}
		} else {
			// Should succeed (no error)
			if err != nil {
				rt.Fatalf("expected no error for valid IDs %v, got: %v", submittedIDs, err)
			}
			if result == nil {
				rt.Fatalf("expected non-nil result for valid IDs %v", submittedIDs)
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

// Feature: ai-summary-streaming, Property 3: Chunk-based timeout behavior
// **Validates: Requirements 3.1, 3.2, 3.3**
//
// Property: For any sequence of chunks where each inter-chunk interval is less than
// the configured timeout duration, the stream SHALL complete without a timeout error
// regardless of total elapsed time. Conversely, for any silence period exceeding the
// configured timeout duration, the stream SHALL terminate with a timeout error.
func TestProperty_ChunkBasedTimeoutBehavior(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Choose scenario: all intervals within timeout, or at least one exceeds timeout
		shouldTimeout := rapid.Bool().Draw(rt, "shouldTimeout")

		// Use a short timeout for fast test execution
		timeout := 50 * time.Millisecond

		// Generate a random number of chunks (1 to 8)
		numChunks := rapid.IntRange(1, 8).Draw(rt, "numChunks")

		// Generate inter-chunk delays
		delays := make([]time.Duration, numChunks)
		if shouldTimeout {
			// Pick a random position where the timeout will occur (0 = before first chunk)
			timeoutPos := rapid.IntRange(0, numChunks-1).Draw(rt, "timeoutPos")
			for i := range numChunks {
				if i == timeoutPos {
					// This delay exceeds the timeout (timeout + 20ms to 2*timeout)
					extraMs := rapid.IntRange(20, 50).Draw(rt, fmt.Sprintf("extraMs_%d", i))
					delays[i] = timeout + time.Duration(extraMs)*time.Millisecond
				} else {
					// Safe delay: 0 to timeout/2
					safeMs := rapid.IntRange(0, 20).Draw(rt, fmt.Sprintf("safeMs_%d", i))
					delays[i] = time.Duration(safeMs) * time.Millisecond
				}
			}
		} else {
			// All delays are safely within the timeout
			for i := range numChunks {
				safeMs := rapid.IntRange(0, 20).Draw(rt, fmt.Sprintf("safeMs_%d", i))
				delays[i] = time.Duration(safeMs) * time.Millisecond
			}
		}

		// Generate random chunk content
		chunks := make([]string, numChunks)
		for i := range numChunks {
			chunks[i] = rapid.StringMatching(`[a-zA-Z0-9 ]{1,20}`).Draw(rt, fmt.Sprintf("chunk_%d", i))
		}

		// Create a minimal SummaryService (nil fields are fine since wrapWithChunkTimeout doesn't use them)
		svc := &SummaryService{}

		// Create contexts: parentCtx is the overall context, llmCtx/llmCancel is for the LLM call.
		// In real usage, wrapWithChunkTimeout receives the parent context and the LLM cancel func.
		// The cancel func aborts the LLM streaming, not the parent context.
		parentCtx, parentCancel := context.WithCancel(context.Background())
		defer parentCancel()
		llmCtx, llmCancel := context.WithCancel(parentCtx)
		_ = llmCtx

		// Create input channel and feed chunks with delays in a goroutine
		input := make(chan StreamChunk, numChunks+1)
		go func() {
			defer close(input)
			for i, chunk := range chunks {
				time.Sleep(delays[i])
				select {
				case input <- StreamChunk{Content: chunk}:
				case <-parentCtx.Done():
					return
				}
			}
			// Send Done signal after all chunks
			select {
			case input <- StreamChunk{Done: true}:
			case <-parentCtx.Done():
			}
		}()

		// Wrap with chunk timeout — pass parentCtx and llmCancel
		output := svc.wrapWithChunkTimeout(parentCtx, input, timeout, llmCancel)

		// Collect output chunks
		var received []StreamChunk
		for chunk := range output {
			received = append(received, chunk)
		}

		if shouldTimeout {
			// Expect a timeout error in the received chunks.
			// Since the cancel func only cancels the LLM context (not the parent context
			// passed to wrapWithChunkTimeout), the timeout error chunk is always delivered.
			hasTimeoutErr := false
			for _, chunk := range received {
				if chunk.Err != nil && strings.Contains(chunk.Err.Error(), "timeout") {
					hasTimeoutErr = true
					break
				}
			}
			if !hasTimeoutErr {
				rt.Fatalf("expected timeout error when a delay exceeds timeout, but got none. "+
					"delays=%v, received=%d chunks", delays, len(received))
			}
		} else {
			// Expect all chunks forwarded successfully with a Done signal, no errors
			hasErr := false
			var errMsg string
			for _, chunk := range received {
				if chunk.Err != nil {
					hasErr = true
					errMsg = chunk.Err.Error()
					break
				}
			}
			if hasErr {
				rt.Fatalf("expected no timeout error when all delays < timeout, but got error: %s. delays=%v",
					errMsg, delays)
			}

			// Verify all content chunks were forwarded
			var contentChunks []string
			hasDone := false
			for _, chunk := range received {
				if chunk.Done {
					hasDone = true
				} else if chunk.Content != "" {
					contentChunks = append(contentChunks, chunk.Content)
				}
			}
			if !hasDone {
				rt.Fatalf("expected Done signal when stream completes without timeout")
			}
			if len(contentChunks) != numChunks {
				rt.Fatalf("expected %d content chunks, got %d", numChunks, len(contentChunks))
			}
			for i, got := range contentChunks {
				if got != chunks[i] {
					rt.Fatalf("chunk %d: expected %q, got %q", i, chunks[i], got)
				}
			}
		}
	})
}

// Feature: ai-summary-streaming, Property 8: Todo IDs persistence
// **Validates: Requirements 6.6**
//
// Property: For any set of todo IDs provided in a create-summary request, the persisted
// Summary record SHALL store exactly those IDs (same set, regardless of order).
func TestProperty_TodoIDsPersistence(t *testing.T) {
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

		commentRepo := repository.NewCommentRepo(db)
		relationRepo := repository.NewRelationRepo(db)
		statusHistoryRepo := repository.NewStatusHistoryRepo(db)
		svc := NewSummaryService(db, summaryRepo, todoRepo, commentRepo, relationRepo, statusHistoryRepo, llmClient, llmCfg)

		// Create a pool of todos belonging to the test user
		numTodos := rapid.IntRange(1, 10).Draw(rt, "numTodos")
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		allTodoIDs := make([]uint, 0, numTodos)
		for i := range numTodos {
			todo := &model.Todo{
				UserID:    userID,
				Code:      fmt.Sprintf("T-%d-%d", time.Now().UnixNano(), i),
				Title:     fmt.Sprintf("Todo %d", i),
				Category:  "task",
				Priority:  "p2",
				Status:    "open",
				CreatedAt: today.AddDate(0, 0, -10),
				UpdatedAt: today.AddDate(0, 0, -10),
			}
			if err := db.Create(todo).Error; err != nil {
				rt.Fatalf("failed to create todo: %v", err)
			}
			allTodoIDs = append(allTodoIDs, todo.ID)
		}

		// Generate a random non-empty subset of the valid todo IDs
		// Use a boolean for each ID to decide inclusion
		selectedIDs := make([]uint, 0)
		for i, id := range allTodoIDs {
			include := rapid.Bool().Draw(rt, fmt.Sprintf("include_%d", i))
			if include {
				selectedIDs = append(selectedIDs, id)
			}
		}
		// Ensure at least one ID is selected
		if len(selectedIDs) == 0 {
			selectedIDs = append(selectedIDs, allTodoIDs[0])
		}

		// Use a past date range so validation passes
		startDate := today.AddDate(0, 0, -30)
		endDate := today.AddDate(0, 0, -1)

		// Call CreateSummaryWithTodos
		summary, err := svc.CreateSummaryWithTodos(userID, startDate, endDate, selectedIDs, "", "")
		if err != nil {
			rt.Fatalf("CreateSummaryWithTodos failed: %v", err)
		}

		// Read back the persisted Summary record from the database
		persisted, err := summaryRepo.FindByID(nil, summary.ID, userID)
		if err != nil {
			rt.Fatalf("FindByID failed: %v", err)
		}

		// Deserialize the stored TodoIDs JSON string back to a set of uint IDs
		storedIDs, err := deserializeTodoIDs(persisted.TodoIDs)
		if err != nil {
			rt.Fatalf("failed to deserialize todo_ids %q: %v", persisted.TodoIDs, err)
		}

		// Verify: the deserialized set equals the original set (order-independent)
		if len(storedIDs) != len(selectedIDs) {
			rt.Fatalf("expected %d todo IDs, got %d (stored: %v, selected: %v)",
				len(selectedIDs), len(storedIDs), storedIDs, selectedIDs)
		}

		selectedSet := make(map[uint]bool, len(selectedIDs))
		for _, id := range selectedIDs {
			selectedSet[id] = true
		}
		storedSet := make(map[uint]bool, len(storedIDs))
		for _, id := range storedIDs {
			storedSet[id] = true
		}

		for id := range selectedSet {
			if !storedSet[id] {
				rt.Fatalf("selected ID %d not found in stored IDs (stored: %v, selected: %v)",
					id, storedIDs, selectedIDs)
			}
		}
		for id := range storedSet {
			if !selectedSet[id] {
				rt.Fatalf("stored ID %d not found in selected IDs (stored: %v, selected: %v)",
					id, storedIDs, selectedIDs)
			}
		}
	})
}

// deserializeTodoIDs parses a JSON array string like "[1,2,3]" into a slice of uint.
func deserializeTodoIDs(s string) ([]uint, error) {
	if s == "" {
		return nil, nil
	}
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil, fmt.Errorf("invalid format: %q", s)
	}
	inner := s[1 : len(s)-1]
	if inner == "" {
		return nil, nil
	}
	parts := strings.Split(inner, ",")
	ids := make([]uint, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		var id uint
		if _, err := fmt.Sscanf(p, "%d", &id); err != nil {
			return nil, fmt.Errorf("invalid ID %q: %w", p, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// Feature: ai-summary-streaming, Property 7: Specified todos analysis scope
// **Validates: Requirements 6.2**
//
// Property: For any non-empty set of valid todo IDs provided in a create-summary request,
// the analysis SHALL use exactly those todos (and no others) regardless of the date range
// parameters. This means the stored todo_ids in the summary record contains exactly the
// specified IDs even when the todos' updated_at falls outside the date range.
func TestProperty_SpecifiedTodosAnalysisScope(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		db := setupServiceTestDB(t)
		summaryRepo := repository.NewSummaryRepo(db)
		todoRepo := repository.NewTodoRepo(db)
		userID := createServiceTestUser(t, db)

		// Create a valid LLM config
		llmCfg := &config.LLMConfig{
			Model:   "test-model",
			BaseURL: "http://localhost:9999",
			APIKey:  "test-key",
			Timeout: 5,
		}
		llmClient := NewLLMClient(llmCfg)

		commentRepo := repository.NewCommentRepo(db)
		relationRepo := repository.NewRelationRepo(db)
		statusHistoryRepo := repository.NewStatusHistoryRepo(db)
		svc := NewSummaryService(db, summaryRepo, todoRepo, commentRepo, relationRepo, statusHistoryRepo, llmClient, llmCfg)

		// Generate a date range in the past
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		rangeStartDaysAgo := rapid.IntRange(30, 60).Draw(rt, "rangeStartDaysAgo")
		rangeEndDaysAgo := rapid.IntRange(1, 29).Draw(rt, "rangeEndDaysAgo")
		startDate := today.AddDate(0, 0, -rangeStartDaysAgo)
		endDate := today.AddDate(0, 0, -rangeEndDaysAgo)

		// Generate todos: some inside the date range, some outside
		numInsideRange := rapid.IntRange(1, 5).Draw(rt, "numInsideRange")
		numOutsideRange := rapid.IntRange(1, 5).Draw(rt, "numOutsideRange")
		totalTodos := numInsideRange + numOutsideRange

		var allTodoIDs []uint
		for i := 0; i < totalTodos; i++ {
			var updatedAt time.Time
			if i < numInsideRange {
				// Inside the date range
				offsetDays := rapid.IntRange(0, rangeStartDaysAgo-rangeEndDaysAgo).Draw(rt, fmt.Sprintf("insideOffset_%d", i))
				updatedAt = startDate.Add(time.Duration(offsetDays) * 24 * time.Hour)
			} else {
				// Outside the date range (before start or after end but still in the past)
				if rapid.Bool().Draw(rt, fmt.Sprintf("beforeOrAfter_%d", i)) {
					// Before start date
					daysBefore := rapid.IntRange(1, 100).Draw(rt, fmt.Sprintf("daysBefore_%d", i))
					updatedAt = startDate.AddDate(0, 0, -daysBefore)
				} else {
					// After end date but still in the past (between endDate and today)
					if rangeEndDaysAgo > 1 {
						daysAfter := rapid.IntRange(1, rangeEndDaysAgo-1).Draw(rt, fmt.Sprintf("daysAfter_%d", i))
						updatedAt = endDate.Add(time.Duration(daysAfter) * 24 * time.Hour)
					} else {
						// If endDate is yesterday, put it before startDate instead
						daysBefore := rapid.IntRange(1, 100).Draw(rt, fmt.Sprintf("daysBeforeFallback_%d", i))
						updatedAt = startDate.AddDate(0, 0, -daysBefore)
					}
				}
			}

			todo := &model.Todo{
				UserID:    userID,
				Code:      fmt.Sprintf("T-%d-%d", time.Now().UnixNano(), i),
				Title:     fmt.Sprintf("Todo %d", i),
				Category:  "task",
				Priority:  "p2",
				Status:    "open",
				CreatedAt: updatedAt,
				UpdatedAt: updatedAt,
			}
			if err := db.Create(todo).Error; err != nil {
				rt.Fatalf("failed to create todo: %v", err)
			}
			allTodoIDs = append(allTodoIDs, todo.ID)
		}

		// Select a random non-empty subset of all todo IDs (mix of inside and outside range)
		// Ensure we include at least one todo that is outside the date range
		subsetSize := rapid.IntRange(1, totalTodos).Draw(rt, "subsetSize")
		// Shuffle and pick
		perm := rapid.Permutation(allTodoIDs).Draw(rt, "perm")
		selectedIDs := make([]uint, subsetSize)
		for i := 0; i < subsetSize; i++ {
			selectedIDs[i] = perm[i]
		}

		// Call CreateSummaryWithTodos with the selected IDs and the date range
		summary, err := svc.CreateSummaryWithTodos(userID, startDate, endDate, selectedIDs, "", "")
		if err != nil {
			rt.Fatalf("CreateSummaryWithTodos failed: %v", err)
		}

		// Verify: the summary stores exactly those todo IDs regardless of date range
		// Parse the stored todo_ids JSON
		if summary.TodoIDs == "" {
			rt.Fatalf("expected non-empty todo_ids in summary, got empty")
		}

		var storedIDs []uint
		if err := json.Unmarshal([]byte(summary.TodoIDs), &storedIDs); err != nil {
			rt.Fatalf("failed to parse stored todo_ids %q: %v", summary.TodoIDs, err)
		}

		// Build sets for comparison (order-independent)
		selectedSet := make(map[uint]bool, len(selectedIDs))
		for _, id := range selectedIDs {
			selectedSet[id] = true
		}
		storedSet := make(map[uint]bool, len(storedIDs))
		for _, id := range storedIDs {
			storedSet[id] = true
		}

		// Verify same set of IDs
		if len(storedSet) != len(selectedSet) {
			rt.Fatalf("stored IDs count %d != selected IDs count %d\nstored: %v\nselected: %v",
				len(storedSet), len(selectedSet), storedIDs, selectedIDs)
		}
		for id := range selectedSet {
			if !storedSet[id] {
				rt.Fatalf("selected ID %d not found in stored IDs %v", id, storedIDs)
			}
		}
		for id := range storedSet {
			if !selectedSet[id] {
				rt.Fatalf("stored ID %d not found in selected IDs %v", id, selectedIDs)
			}
		}

		// Also verify via database read
		dbSummary, err := summaryRepo.FindByID(nil, summary.ID, userID)
		if err != nil {
			rt.Fatalf("FindByID failed: %v", err)
		}
		if dbSummary.TodoIDs != summary.TodoIDs {
			rt.Fatalf("DB todo_ids %q != returned todo_ids %q", dbSummary.TodoIDs, summary.TodoIDs)
		}
	})
}

// Feature: ai-summary-enhancement, Property 6: Enriched prompt contains all relevant data
// **Validates: Requirements 3.1, 3.2, 3.3, 3.4, 3.5**
//
// Property: For any todo with non-empty fields, the built prompt SHALL contain:
// the todo's title, code, category, priority, status, description, all associated
// comments (with content and timestamp), all dependency relationships (with code,
// title, and status of related todos), all in-range status history entries (with
// old_status, new_status, and changed_at), and all tags.
func TestProperty_SummaryEnrichedPromptContainsAllRelevantData(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a todo with non-empty fields
		title := rapid.StringMatching(`[A-Za-z0-9 ]{5,30}`).Draw(rt, "title")
		code := rapid.StringMatching(`T-[0-9]{3,6}`).Draw(rt, "code")
		category := rapid.SampledFrom([]string{"bug", "feature", "task"}).Draw(rt, "category")
		priority := rapid.SampledFrom([]string{"p0", "p1", "p2", "p3"}).Draw(rt, "priority")
		status := rapid.SampledFrom([]string{"open", "in_progress", "completed"}).Draw(rt, "status")
		description := rapid.StringMatching(`[A-Za-z0-9 ]{10,50}`).Draw(rt, "description")

		// Generate tags
		numTags := rapid.IntRange(1, 3).Draw(rt, "numTags")
		tags := make([]model.TodoTag, numTags)
		for i := range numTags {
			tags[i] = model.TodoTag{
				TodoID: 1,
				Tag:    rapid.StringMatching(`[a-z]{3,10}`).Draw(rt, fmt.Sprintf("tag_%d", i)),
			}
		}

		todo := &model.Todo{
			ID:          1,
			UserID:      1,
			Code:        code,
			Title:       title,
			Description: description,
			Category:    category,
			Priority:    priority,
			Status:      status,
			Tags:        tags,
		}

		// Generate comments
		numComments := rapid.IntRange(1, 3).Draw(rt, "numComments")
		comments := make([]*model.Comment, numComments)
		for i := range numComments {
			comments[i] = &model.Comment{
				ID:        uint(i + 1),
				TodoID:    1,
				UserID:    1,
				Content:   rapid.StringMatching(`[A-Za-z0-9 ]{5,30}`).Draw(rt, fmt.Sprintf("commentContent_%d", i)),
				CreatedAt: time.Date(2024, 6, rapid.IntRange(1, 28).Draw(rt, fmt.Sprintf("commentDay_%d", i)), 10, 0, 0, 0, time.UTC),
			}
		}

		// Generate dependency references
		numPrereqs := rapid.IntRange(1, 2).Draw(rt, "numPrereqs")
		prereqs := make([]TodoDependencyRef, numPrereqs)
		for i := range numPrereqs {
			prereqs[i] = TodoDependencyRef{
				Code:   rapid.StringMatching(`T-[0-9]{3}`).Draw(rt, fmt.Sprintf("prereqCode_%d", i)),
				Title:  rapid.StringMatching(`[A-Za-z0-9 ]{5,20}`).Draw(rt, fmt.Sprintf("prereqTitle_%d", i)),
				Status: rapid.SampledFrom([]string{"open", "in_progress", "completed"}).Draw(rt, fmt.Sprintf("prereqStatus_%d", i)),
			}
		}

		numDependents := rapid.IntRange(1, 2).Draw(rt, "numDependents")
		dependents := make([]TodoDependencyRef, numDependents)
		for i := range numDependents {
			dependents[i] = TodoDependencyRef{
				Code:   rapid.StringMatching(`T-[0-9]{3}`).Draw(rt, fmt.Sprintf("depCode_%d", i)),
				Title:  rapid.StringMatching(`[A-Za-z0-9 ]{5,20}`).Draw(rt, fmt.Sprintf("depTitle_%d", i)),
				Status: rapid.SampledFrom([]string{"open", "in_progress", "completed"}).Draw(rt, fmt.Sprintf("depStatus_%d", i)),
			}
		}

		// Generate status history entries
		numHistory := rapid.IntRange(1, 3).Draw(rt, "numHistory")
		history := make([]*model.TodoStatusHistory, numHistory)
		for i := range numHistory {
			history[i] = &model.TodoStatusHistory{
				ID:        uint(i + 1),
				TodoID:    1,
				OldStatus: rapid.SampledFrom([]string{"", "open", "in_progress"}).Draw(rt, fmt.Sprintf("histOld_%d", i)),
				NewStatus: rapid.SampledFrom([]string{"open", "in_progress", "completed"}).Draw(rt, fmt.Sprintf("histNew_%d", i)),
				ChangedAt: time.Date(2024, 6, rapid.IntRange(1, 28).Draw(rt, fmt.Sprintf("histDay_%d", i)), 14, 0, 0, 0, time.UTC),
			}
		}

		// Build enriched data
		enrichedData := &EnrichedTodoData{
			Comments: map[uint][]*model.Comment{
				1: comments,
			},
			Dependencies: map[uint]*DependencyInfo{
				1: {
					Prerequisites: prereqs,
					Dependents:    dependents,
				},
			},
			StatusHistory: map[uint][]*model.TodoStatusHistory{
				1: history,
			},
		}

		// Build the prompt
		language := "English"
		startDate := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2024, 6, 30, 23, 59, 59, 0, time.UTC)

		svc := &SummaryService{}
		prompt := svc.buildEnrichedPrompt([]*model.Todo{todo}, enrichedData, language, startDate, endDate, "")

		// Verify basic info is present
		if !strings.Contains(prompt, title) {
			rt.Fatalf("prompt missing title %q", title)
		}
		if !strings.Contains(prompt, code) {
			rt.Fatalf("prompt missing code %q", code)
		}
		if !strings.Contains(prompt, category) {
			rt.Fatalf("prompt missing category %q", category)
		}
		if !strings.Contains(prompt, priority) {
			rt.Fatalf("prompt missing priority %q", priority)
		}
		if !strings.Contains(prompt, status) {
			rt.Fatalf("prompt missing status %q", status)
		}
		if !strings.Contains(prompt, description) {
			rt.Fatalf("prompt missing description %q", description)
		}

		// Verify all tags are present
		for _, tag := range tags {
			if !strings.Contains(prompt, tag.Tag) {
				rt.Fatalf("prompt missing tag %q", tag.Tag)
			}
		}

		// Verify all comments are present (content and timestamp)
		for _, c := range comments {
			if !strings.Contains(prompt, c.Content) {
				rt.Fatalf("prompt missing comment content %q", c.Content)
			}
			ts := c.CreatedAt.Format("2006-01-02 15:04:05")
			if !strings.Contains(prompt, ts) {
				rt.Fatalf("prompt missing comment timestamp %q", ts)
			}
		}

		// Verify all dependency relationships are present
		for _, p := range prereqs {
			if !strings.Contains(prompt, p.Code) {
				rt.Fatalf("prompt missing prerequisite code %q", p.Code)
			}
			if !strings.Contains(prompt, p.Title) {
				rt.Fatalf("prompt missing prerequisite title %q", p.Title)
			}
			if !strings.Contains(prompt, p.Status) {
				rt.Fatalf("prompt missing prerequisite status %q", p.Status)
			}
		}
		for _, d := range dependents {
			if !strings.Contains(prompt, d.Code) {
				rt.Fatalf("prompt missing dependent code %q", d.Code)
			}
			if !strings.Contains(prompt, d.Title) {
				rt.Fatalf("prompt missing dependent title %q", d.Title)
			}
			if !strings.Contains(prompt, d.Status) {
				rt.Fatalf("prompt missing dependent status %q", d.Status)
			}
		}

		// Verify all status history entries are present
		for _, h := range history {
			if !strings.Contains(prompt, h.OldStatus) && h.OldStatus != "" {
				rt.Fatalf("prompt missing history old_status %q", h.OldStatus)
			}
			if !strings.Contains(prompt, h.NewStatus) {
				rt.Fatalf("prompt missing history new_status %q", h.NewStatus)
			}
			ts := h.ChangedAt.Format("2006-01-02 15:04:05")
			if !strings.Contains(prompt, ts) {
				rt.Fatalf("prompt missing history changed_at %q", ts)
			}
		}
	})
}

// Feature: ai-summary-enhancement, Property 7: Empty sections are omitted from prompt
// **Validates: Requirements 3.6, 3.7, 6.1**
//
// Property: For any todo where a data section (comments, dependencies, or status
// history) is empty, the built prompt SHALL NOT contain the section header or
// placeholder for that section.
func TestProperty_SummaryEmptySectionsOmittedFromPrompt(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a todo with basic fields
		todo := &model.Todo{
			ID:          1,
			UserID:      1,
			Code:        rapid.StringMatching(`T-[0-9]{3}`).Draw(rt, "code"),
			Title:       rapid.StringMatching(`[A-Za-z0-9 ]{5,20}`).Draw(rt, "title"),
			Description: rapid.StringMatching(`[A-Za-z0-9 ]{5,20}`).Draw(rt, "description"),
			Category:    rapid.SampledFrom([]string{"bug", "feature", "task"}).Draw(rt, "category"),
			Priority:    rapid.SampledFrom([]string{"p0", "p1", "p2", "p3"}).Draw(rt, "priority"),
			Status:      rapid.SampledFrom([]string{"open", "in_progress", "completed"}).Draw(rt, "status"),
		}

		// Randomly decide which sections are empty
		hasComments := rapid.Bool().Draw(rt, "hasComments")
		hasDeps := rapid.Bool().Draw(rt, "hasDeps")
		hasHistory := rapid.Bool().Draw(rt, "hasHistory")

		// Ensure at least one section is empty for this property to be meaningful
		if hasComments && hasDeps && hasHistory {
			// Force at least one to be empty
			switch rapid.IntRange(0, 2).Draw(rt, "forceEmpty") {
			case 0:
				hasComments = false
			case 1:
				hasDeps = false
			case 2:
				hasHistory = false
			}
		}

		enrichedData := &EnrichedTodoData{
			Comments:      make(map[uint][]*model.Comment),
			Dependencies:  make(map[uint]*DependencyInfo),
			StatusHistory: make(map[uint][]*model.TodoStatusHistory),
		}

		if hasComments {
			enrichedData.Comments[1] = []*model.Comment{
				{ID: 1, TodoID: 1, UserID: 1, Content: "test comment", CreatedAt: time.Now()},
			}
		}
		if hasDeps {
			enrichedData.Dependencies[1] = &DependencyInfo{
				Prerequisites: []TodoDependencyRef{{Code: "T-001", Title: "Prereq", Status: "open"}},
			}
		}
		if hasHistory {
			enrichedData.StatusHistory[1] = []*model.TodoStatusHistory{
				{ID: 1, TodoID: 1, OldStatus: "open", NewStatus: "in_progress", ChangedAt: time.Now()},
			}
		}

		svc := &SummaryService{}
		startDate := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2024, 6, 30, 23, 59, 59, 0, time.UTC)
		prompt := svc.buildEnrichedPrompt([]*model.Todo{todo}, enrichedData, "English", startDate, endDate, "")

		// Verify empty sections are omitted
		if !hasComments {
			if strings.Contains(prompt, "Comments:") {
				rt.Fatalf("prompt should NOT contain 'Comments:' header when comments are empty")
			}
		} else {
			if !strings.Contains(prompt, "Comments:") {
				rt.Fatalf("prompt should contain 'Comments:' header when comments are present")
			}
		}

		if !hasDeps {
			if strings.Contains(prompt, "Dependencies:") {
				rt.Fatalf("prompt should NOT contain 'Dependencies:' header when dependencies are empty")
			}
			if strings.Contains(prompt, "Prerequisites:") {
				rt.Fatalf("prompt should NOT contain 'Prerequisites:' header when dependencies are empty")
			}
			if strings.Contains(prompt, "Dependents:") {
				rt.Fatalf("prompt should NOT contain 'Dependents:' header when dependencies are empty")
			}
		} else {
			if !strings.Contains(prompt, "Dependencies:") {
				rt.Fatalf("prompt should contain 'Dependencies:' header when dependencies are present")
			}
		}

		if !hasHistory {
			if strings.Contains(prompt, "Status History:") {
				rt.Fatalf("prompt should NOT contain 'Status History:' header when status history is empty")
			}
		} else {
			if !strings.Contains(prompt, "Status History:") {
				rt.Fatalf("prompt should contain 'Status History:' header when status history is present")
			}
		}
	})
}

// Feature: ai-summary-enhancement, Property 8: Two-step language detection flow
// **Validates: Requirements 7.1, 7.2, 7.3**
//
// Property: For any summary generation with a non-empty todo set, the service SHALL
// make exactly two LLM calls: the first containing todo titles and descriptions for
// language detection, and the second containing the detected language identifier in
// its system prompt instruction.
func TestProperty_SummaryTwoStepLanguageDetectionFlow(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		db := setupServiceTestDB(t)
		summaryRepo := repository.NewSummaryRepo(db)
		todoRepo := repository.NewTodoRepo(db)
		userID := createServiceTestUser(t, db)

		// Generate random todo data
		numTodos := rapid.IntRange(1, 5).Draw(rt, "numTodos")
		type todoData struct {
			title       string
			description string
		}
		todosData := make([]todoData, numTodos)
		for i := range numTodos {
			todosData[i] = todoData{
				title:       rapid.StringMatching(`[A-Za-z0-9 ]{5,20}`).Draw(rt, fmt.Sprintf("todoTitle_%d", i)),
				description: rapid.StringMatching(`[A-Za-z0-9 ]{10,30}`).Draw(rt, fmt.Sprintf("todoDesc_%d", i)),
			}
		}

		// Randomly choose the language the mock LLM will detect
		detectedLanguage := rapid.SampledFrom([]string{"Chinese", "English"}).Draw(rt, "detectedLanguage")

		// Create todos in the database within the date range
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		startDate := today.AddDate(0, 0, -30)
		endDate := today.AddDate(0, 0, -1)
		midDate := startDate.Add(48 * time.Hour)

		for i, td := range todosData {
			todo := &model.Todo{
				UserID:      userID,
				Code:        fmt.Sprintf("T-%d-%d", time.Now().UnixNano(), i),
				Title:       td.title,
				Description: td.description,
				Category:    "task",
				Priority:    "p2",
				Status:      "open",
				CreatedAt:   midDate,
				UpdatedAt:   midDate,
			}
			if err := db.Create(todo).Error; err != nil {
				rt.Fatalf("failed to create todo: %v", err)
			}
		}

		// Track LLM calls
		var llmCalls [][]ChatMessage
		callCount := 0

		// Create a mock LLM server that records calls
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req chatCompletionRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			llmCalls = append(llmCalls, req.Messages)
			callCount++

			var responseContent string
			if callCount == 1 {
				// First call: language detection - return the detected language
				responseContent = detectedLanguage
			} else {
				// Second call: analysis - return a summary
				responseContent = "## Summary\nThis is a test summary."
			}

			resp := chatCompletionResponse{
				Choices: []struct {
					Message ChatMessage `json:"message"`
				}{
					{Message: ChatMessage{Role: "assistant", Content: responseContent}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		llmCfg := &config.LLMConfig{
			Model:   "test-model",
			BaseURL: server.URL,
			APIKey:  "test-api-key",
			Timeout: 10,
		}
		llmClient := NewLLMClient(llmCfg)

		commentRepo := repository.NewCommentRepo(db)
		relationRepo := repository.NewRelationRepo(db)
		statusHistoryRepo := repository.NewStatusHistoryRepo(db)
		svc := NewSummaryService(db, summaryRepo, todoRepo, commentRepo, relationRepo, statusHistoryRepo, llmClient, llmCfg)

		// Call CreateSummary
		summary, err := svc.CreateSummary(userID, startDate, endDate)
		if err != nil {
			rt.Fatalf("CreateSummary failed: %v", err)
		}

		// Wait for background analysis to complete
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			updated, err := summaryRepo.FindByID(nil, summary.ID, userID)
			if err != nil {
				rt.Fatalf("FindByID failed: %v", err)
			}
			if updated.Status != model.SummaryStatusAnalyzing {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}

		// Verify exactly 2 LLM calls were made
		if len(llmCalls) != 2 {
			rt.Fatalf("expected exactly 2 LLM calls, got %d", len(llmCalls))
		}

		// Verify first call contains todo titles and descriptions (language detection)
		firstCallMessages := llmCalls[0]
		firstCallUserContent := ""
		for _, msg := range firstCallMessages {
			if msg.Role == "user" {
				firstCallUserContent = msg.Content
			}
		}
		if firstCallUserContent == "" {
			rt.Fatalf("first LLM call has no user message")
		}
		// The first call should contain all todo titles and descriptions
		for _, td := range todosData {
			if !strings.Contains(firstCallUserContent, td.title) {
				rt.Fatalf("first LLM call missing todo title %q", td.title)
			}
			if !strings.Contains(firstCallUserContent, td.description) {
				rt.Fatalf("first LLM call missing todo description %q", td.description)
			}
		}

		// Verify second call's system message contains the detected language
		secondCallMessages := llmCalls[1]
		secondCallSystemContent := ""
		for _, msg := range secondCallMessages {
			if msg.Role == "system" {
				secondCallSystemContent = msg.Content
			}
		}
		if secondCallSystemContent == "" {
			rt.Fatalf("second LLM call has no system message")
		}
		if !strings.Contains(secondCallSystemContent, detectedLanguage) {
			rt.Fatalf("second LLM call system message should contain detected language %q, got: %q",
				detectedLanguage, secondCallSystemContent)
		}
	})
}

// Feature: ai-summary-streaming, Property 2: Stream concatenation persistence (Round-trip)
// **Validates: Requirements 2.5**
//
// Property: For any sequence of text chunks delivered during a streaming analysis,
// the concatenation of all chunks in order SHALL equal the result_content persisted
// in the Summary record upon successful completion.
func TestProperty_StreamConcatenationPersistence(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		db := setupServiceTestDB(t)
		summaryRepo := repository.NewSummaryRepo(db)
		todoRepo := repository.NewTodoRepo(db)
		commentRepo := repository.NewCommentRepo(db)
		relationRepo := repository.NewRelationRepo(db)
		statusHistoryRepo := repository.NewStatusHistoryRepo(db)
		userID := createServiceTestUser(t, db)

		// Generate a random number of text chunks (1 to 10)
		numChunks := rapid.IntRange(1, 10).Draw(rt, "numChunks")
		chunks := make([]string, numChunks)
		for i := range numChunks {
			// Generate arbitrary text content including unicode and special characters
			chunks[i] = rapid.StringMatching(`[a-zA-Z0-9 \.\,\!\?\n]{1,50}`).Draw(rt, fmt.Sprintf("chunk_%d", i))
		}

		// Track request count to distinguish language detection from streaming
		var requestCount atomic.Int64

		// Create a mock LLM server that handles both non-streaming and streaming requests
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read the request body to determine if it's a streaming request
			var reqBody struct {
				Stream bool `json:"stream"`
			}
			bodyBytes, _ := readRequestBody(r)
			json.Unmarshal(bodyBytes, &reqBody)

			count := requestCount.Add(1)

			if !reqBody.Stream {
				// Non-streaming request (language detection) - return "English"
				resp := chatCompletionResponse{
					Choices: []struct {
						Message ChatMessage `json:"message"`
					}{
						{Message: ChatMessage{Role: "assistant", Content: "English"}},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(resp)
				return
			}

			// Streaming request - return SSE formatted chunks
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.WriteHeader(http.StatusOK)

			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "streaming not supported", http.StatusInternalServerError)
				return
			}

			_ = count // suppress unused warning

			// Send each chunk as an SSE event in OpenAI streaming format
			for i, chunk := range chunks {
				sseData := fmt.Sprintf(`{"id":"chatcmpl-%d","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":%s},"finish_reason":null}]}`,
					i, jsonEscapeString(chunk))
				fmt.Fprintf(w, "data: %s\n\n", sseData)
				flusher.Flush()
			}

			// Send the [DONE] marker
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
		}))
		defer server.Close()

		llmCfg := &config.LLMConfig{
			Model:   "test-model",
			BaseURL: server.URL,
			APIKey:  "test-api-key",
			Timeout: 30,
		}
		llmClient := NewLLMClient(llmCfg)

		svc := NewSummaryService(db, summaryRepo, todoRepo, commentRepo, relationRepo, statusHistoryRepo, llmClient, llmCfg)

		// Create a todo so the analysis has content to work with
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		startDate := today.AddDate(0, 0, -30)
		endDate := today.AddDate(0, 0, -1)
		midDate := startDate.Add(48 * time.Hour)

		todo := &model.Todo{
			UserID:    userID,
			Code:      fmt.Sprintf("T-%d", time.Now().UnixNano()),
			Title:     "Test Todo for Streaming",
			Category:  "task",
			Priority:  "p2",
			Status:    "open",
			CreatedAt: midDate,
			UpdatedAt: midDate,
		}
		if err := db.Create(todo).Error; err != nil {
			rt.Fatalf("failed to create test todo: %v", err)
		}

		// Create a summary record in "analyzing" status
		summary := &model.Summary{
			UserID:    userID,
			StartDate: startDate,
			EndDate:   endDate,
			Status:    model.SummaryStatusAnalyzing,
			TodoIDs:   fmt.Sprintf("[%d]", todo.ID),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := db.Create(summary).Error; err != nil {
			rt.Fatalf("failed to create summary: %v", err)
		}

		// Call StreamAnalysis
		ctx := context.Background()
		outCh, err := svc.StreamAnalysis(ctx, summary.ID, userID)
		if err != nil {
			rt.Fatalf("StreamAnalysis failed: %v", err)
		}

		// Consume all chunks from the output channel and concatenate content
		var receivedContent strings.Builder
		for chunk := range outCh {
			if chunk.Err != nil {
				rt.Fatalf("unexpected error chunk: %v", chunk.Err)
			}
			if chunk.Done {
				break
			}
			receivedContent.WriteString(chunk.Content)
		}

		concatenated := receivedContent.String()

		// Read back the persisted Summary record from the database
		// Give a brief moment for the goroutine to persist
		time.Sleep(50 * time.Millisecond)

		persisted, err := summaryRepo.FindByID(nil, summary.ID, userID)
		if err != nil {
			rt.Fatalf("failed to read persisted summary: %v", err)
		}

		// Verify: status should be "completed"
		if persisted.Status != model.SummaryStatusCompleted {
			rt.Fatalf("expected status %q, got %q (result_content: %q)",
				model.SummaryStatusCompleted, persisted.Status, persisted.ResultContent)
		}

		// Verify: concatenation of all received chunks equals the persisted result_content
		if concatenated != persisted.ResultContent {
			rt.Fatalf("concatenated chunks != persisted result_content\nconcatenated (%d chars): %q\npersisted (%d chars): %q",
				len(concatenated), concatenated, len(persisted.ResultContent), persisted.ResultContent)
		}

		// Also verify the concatenation matches the expected full content
		expectedFull := strings.Join(chunks, "")
		if concatenated != expectedFull {
			rt.Fatalf("concatenated chunks != expected full content\nconcatenated: %q\nexpected: %q",
				concatenated, expectedFull)
		}
	})
}

// readRequestBody reads and returns the request body bytes, then restores the body for re-reading.
func readRequestBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	bodyBytes := make([]byte, 0)
	buf := make([]byte, 1024)
	for {
		n, err := r.Body.Read(buf)
		if n > 0 {
			bodyBytes = append(bodyBytes, buf[:n]...)
		}
		if err != nil {
			break
		}
	}
	r.Body = nopCloser{strings.NewReader(string(bodyBytes))}
	return bodyBytes, nil
}

// nopCloser wraps a reader with a no-op Close method.
type nopCloser struct {
	*strings.Reader
}

func (nopCloser) Close() error { return nil }

// jsonEscapeString returns a JSON-encoded string value (with quotes).
func jsonEscapeString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
