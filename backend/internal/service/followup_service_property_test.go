package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/graydovee/todo-manager/internal/config"
	"github.com/graydovee/todo-manager/internal/model"
	"github.com/graydovee/todo-manager/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"pgregory.net/rapid"
)

// setupFollowupServiceTestDB creates an in-memory SQLite database for followup service property tests.
func setupFollowupServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	id := servicePropertyTestDBCounter.Add(1)
	dsn := fmt.Sprintf("file:followupservicedb_%d?mode=memory&cache=shared", id)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)

	// Enable foreign key enforcement for SQLite
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
		`CREATE TABLE IF NOT EXISTS todos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			code TEXT NOT NULL,
			title TEXT NOT NULL,
			description TEXT DEFAULT '',
			category TEXT NOT NULL CHECK(category IN ('bug','feature','task')),
			priority TEXT NOT NULL DEFAULT 'p2' CHECK(priority IN ('p0','p1','p2','p3')),
			status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open','in_progress','completed','duplicate')),
			due_at DATETIME,
			pinned INTEGER NOT NULL DEFAULT 0,
			highlighted INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, code)
		)`,
		`CREATE TABLE IF NOT EXISTS todo_tags (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			todo_id INTEGER NOT NULL,
			tag TEXT NOT NULL,
			UNIQUE(todo_id, tag)
		)`,
		`CREATE TABLE IF NOT EXISTS todo_relations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_id INTEGER NOT NULL,
			target_id INTEGER NOT NULL,
			relation_type TEXT NOT NULL CHECK(relation_type IN ('depends_on','duplicate_of')),
			UNIQUE(source_id, target_id, relation_type)
		)`,
		`CREATE TABLE IF NOT EXISTS comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			todo_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS todo_status_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			todo_id INTEGER NOT NULL,
			old_status TEXT NOT NULL,
			new_status TEXT NOT NULL,
			changed_at DATETIME NOT NULL
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

// createFollowupTestUser inserts a user record and returns the user ID.
func createFollowupTestUser(t *testing.T, db *gorm.DB) uint {
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

// createCompletedSummary creates a completed summary with the given custom prompt.
func createCompletedSummary(t *testing.T, db *gorm.DB, userID uint, customPrompt string) *model.Summary {
	t.Helper()
	summary := &model.Summary{
		UserID:        userID,
		StartDate:     time.Now().AddDate(0, 0, -30),
		EndDate:       time.Now().AddDate(0, 0, -1),
		Status:        model.SummaryStatusCompleted,
		ResultContent: "This is a test summary content for followup testing.",
		CustomPrompt:  customPrompt,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := db.Create(summary).Error; err != nil {
		t.Fatalf("create test summary: %v", err)
	}
	return summary
}

// Feature: ai-summary-followup, Property 1: Custom prompt persistence round-trip
// **Validates: Requirements 2.3, 2.4**
//
// Property: For any non-whitespace string of at most 500 characters provided as
// custom_prompt, creating a summary and then retrieving it SHALL return the same
// trimmed custom_prompt value. For any whitespace-only string or empty string,
// the persisted value SHALL be empty.
func TestProperty_CustomPromptPersistenceRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		db := setupFollowupServiceTestDB(t)
		summaryRepo := repository.NewSummaryRepo(db)
		todoRepo := repository.NewTodoRepo(db)
		userID := createFollowupTestUser(t, db)

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

		// Choose scenario: non-whitespace string or whitespace-only/empty
		scenario := rapid.IntRange(0, 2).Draw(rt, "scenario")

		var customPrompt string
		switch scenario {
		case 0:
			// Non-whitespace string of at most 500 characters
			content := rapid.StringMatching(`[a-zA-Z0-9\x{4e00}-\x{9fff} ,.!?]{1,100}`).Draw(rt, "content")
			// Add optional leading/trailing whitespace
			leading := rapid.StringMatching(`[ \t]{0,5}`).Draw(rt, "leading")
			trailing := rapid.StringMatching(`[ \t]{0,5}`).Draw(rt, "trailing")
			customPrompt = leading + content + trailing
		case 1:
			// Whitespace-only string
			customPrompt = rapid.StringMatching(`[ \t\n]{1,20}`).Draw(rt, "whitespace")
		case 2:
			// Empty string
			customPrompt = ""
		}

		// Create a todo so the service doesn't fail on empty todo list
		todo := &model.Todo{
			UserID:    userID,
			Code:      fmt.Sprintf("T-%d", time.Now().UnixNano()),
			Title:     "Test Todo",
			Category:  "task",
			Priority:  "p2",
			Status:    "open",
			CreatedAt: time.Now().AddDate(0, 0, -10),
			UpdatedAt: time.Now().AddDate(0, 0, -10),
		}
		if err := db.Create(todo).Error; err != nil {
			rt.Fatalf("failed to create test todo: %v", err)
		}

		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		startDate := today.AddDate(0, 0, -30)
		endDate := today.AddDate(0, 0, -1)

		// Trim the custom prompt as the handler would
		trimmed := strings.TrimSpace(customPrompt)
		inputPrompt := trimmed

		// Call CreateSummaryWithTodos
		summary, err := svc.CreateSummaryWithTodos(userID, startDate, endDate, []uint{todo.ID}, "", inputPrompt)
		if err != nil {
			rt.Fatalf("CreateSummaryWithTodos failed: %v", err)
		}

		// Retrieve the summary
		persisted, err := summaryRepo.FindByID(nil, summary.ID, userID)
		if err != nil {
			rt.Fatalf("FindByID failed: %v", err)
		}

		switch scenario {
		case 0:
			// Non-whitespace: persisted value should equal the trimmed input
			if persisted.CustomPrompt != trimmed {
				rt.Fatalf("expected custom_prompt %q, got %q", trimmed, persisted.CustomPrompt)
			}
		case 1, 2:
			// Whitespace-only or empty: persisted value should be empty
			if persisted.CustomPrompt != "" {
				rt.Fatalf("expected empty custom_prompt for whitespace/empty input, got %q", persisted.CustomPrompt)
			}
		}
	})
}

// Feature: ai-summary-followup, Property 3: Custom prompt inclusion in LLM prompt
// **Validates: Requirements 2.5, 2.6**
//
// Property: For any Summary record with a non-empty custom_prompt value, the constructed
// LLM user prompt SHALL contain the substring "Additional user requirements: " followed
// by the custom_prompt content. For any Summary record with an empty custom_prompt, the
// constructed LLM user prompt SHALL NOT contain "Additional user requirements:".
func TestProperty_CustomPromptInclusionInLLMPrompt(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		db := setupFollowupServiceTestDB(t)
		todoRepo := repository.NewTodoRepo(db)
		summaryRepo := repository.NewSummaryRepo(db)
		commentRepo := repository.NewCommentRepo(db)
		relationRepo := repository.NewRelationRepo(db)
		statusHistoryRepo := repository.NewStatusHistoryRepo(db)

		llmCfg := &config.LLMConfig{
			Model:   "test-model",
			BaseURL: "http://localhost:9999",
			APIKey:  "test-key",
			Timeout: 5,
		}
		llmClient := NewLLMClient(llmCfg)
		svc := NewSummaryService(db, summaryRepo, todoRepo, commentRepo, relationRepo, statusHistoryRepo, llmClient, llmCfg)

		// Choose scenario: non-empty custom prompt or empty
		hasCustomPrompt := rapid.Bool().Draw(rt, "hasCustomPrompt")

		var customPrompt string
		if hasCustomPrompt {
			customPrompt = rapid.StringMatching(`[a-zA-Z0-9 ,.!?]{1,100}`).Draw(rt, "customPrompt")
		} else {
			customPrompt = ""
		}

		// Create a minimal todo for the prompt builder
		userID := createFollowupTestUser(t, db)
		todo := &model.Todo{
			UserID:   userID,
			Code:     fmt.Sprintf("T-%d", time.Now().UnixNano()),
			Title:    "Test Todo",
			Category: "task",
			Priority: "p2",
			Status:   "open",
		}

		todos := []*model.Todo{todo}
		enrichedData := &EnrichedTodoData{
			Comments:      make(map[uint][]*model.Comment),
			Dependencies:  make(map[uint]*DependencyInfo),
			StatusHistory: make(map[uint][]*model.TodoStatusHistory),
		}

		now := time.Now()
		startDate := now.AddDate(0, 0, -30)
		endDate := now.AddDate(0, 0, -1)

		// Call buildEnrichedPrompt
		prompt := svc.buildEnrichedPrompt(todos, enrichedData, "English", startDate, endDate, customPrompt)

		expectedSubstring := "Additional user requirements: " + customPrompt

		if hasCustomPrompt {
			if !strings.Contains(prompt, expectedSubstring) {
				rt.Fatalf("expected prompt to contain %q, but it did not.\nPrompt: %s", expectedSubstring, prompt)
			}
		} else {
			if strings.Contains(prompt, "Additional user requirements:") {
				rt.Fatalf("expected prompt NOT to contain 'Additional user requirements:' for empty custom_prompt.\nPrompt: %s", prompt)
			}
		}
	})
}

// Feature: ai-summary-followup, Property 4: Followup context message array construction
// **Validates: Requirements 5.1, 5.2**
//
// Property: For any conversation history of N Q&A pairs (where N ≤ 20) and a new question,
// the constructed LLM message array SHALL have the structure:
// [system message with summary content, user₁, assistant₁, ..., userₙ, assistantₙ, new question].
// For any conversation history exceeding 20 pairs, only the most recent 20 pairs SHALL be included.
func TestProperty_FollowupContextMessageArrayConstruction(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a FollowupService instance (only buildFollowupMessages and truncateContext are needed)
		svc := &FollowupService{}

		// Generate a random summary content
		summaryContent := rapid.StringMatching(`[a-zA-Z0-9 ]{10,200}`).Draw(rt, "summaryContent")

		// Generate a random number of Q&A pairs (0 to 30, to test both within and exceeding 20)
		numPairs := rapid.IntRange(0, 30).Draw(rt, "numPairs")

		// Build context messages as alternating user/assistant pairs
		contextMessages := make([]ContextMessage, 0, numPairs*2)
		for i := 0; i < numPairs; i++ {
			q := rapid.StringMatching(`[a-zA-Z0-9 ?]{5,50}`).Draw(rt, fmt.Sprintf("q_%d", i))
			a := rapid.StringMatching(`[a-zA-Z0-9 .]{10,100}`).Draw(rt, fmt.Sprintf("a_%d", i))
			contextMessages = append(contextMessages, ContextMessage{Role: "user", Content: q})
			contextMessages = append(contextMessages, ContextMessage{Role: "assistant", Content: a})
		}

		// Generate a new question
		newQuestion := rapid.StringMatching(`[a-zA-Z0-9 ?]{5,50}`).Draw(rt, "newQuestion")

		// Truncate context first (as the service does)
		truncated := svc.truncateContext(contextMessages)

		// Build the message array
		messages := svc.buildFollowupMessages(summaryContent, truncated, newQuestion)

		// Verify structure:
		// 1. First message is system message containing summary content
		if len(messages) == 0 {
			rt.Fatalf("expected non-empty message array")
		}
		if messages[0].Role != "system" {
			rt.Fatalf("expected first message role to be 'system', got %q", messages[0].Role)
		}
		if !strings.Contains(messages[0].Content, summaryContent) {
			rt.Fatalf("expected system message to contain summary content")
		}

		// 2. Last message is the new question
		lastMsg := messages[len(messages)-1]
		if lastMsg.Role != "user" {
			rt.Fatalf("expected last message role to be 'user', got %q", lastMsg.Role)
		}
		if lastMsg.Content != newQuestion {
			rt.Fatalf("expected last message content to be %q, got %q", newQuestion, lastMsg.Content)
		}

		// 3. Middle messages are the context (truncated to max 20 pairs = 40 messages)
		expectedContextLen := len(truncated)
		actualContextLen := len(messages) - 2 // minus system and new question
		if actualContextLen != expectedContextLen {
			rt.Fatalf("expected %d context messages, got %d", expectedContextLen, actualContextLen)
		}

		// 4. If numPairs > 20, only the most recent 20 pairs should be included
		if numPairs > 20 {
			if len(truncated) != 40 {
				rt.Fatalf("expected truncated context to have 40 messages (20 pairs), got %d", len(truncated))
			}
		}

		// 5. Verify alternating user/assistant pattern in context
		for i := 1; i < len(messages)-1; i++ {
			expectedRole := "user"
			if (i-1)%2 == 1 {
				expectedRole = "assistant"
			}
			if messages[i].Role != expectedRole {
				rt.Fatalf("message[%d] expected role %q, got %q", i, expectedRole, messages[i].Role)
			}
		}
	})
}

// Feature: ai-summary-followup, Property 5: Context truncation on edit
// **Validates: Requirements 4.4, 5.3**
//
// Property: For any conversation with K messages where the user edits message at
// position P (1 ≤ P ≤ K), the context sent to the LLM SHALL contain only the Q&A
// pairs preceding position P.
func TestProperty_ContextTruncationOnEdit(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc := &FollowupService{}

		summaryContent := rapid.StringMatching(`[a-zA-Z0-9 ]{10,100}`).Draw(rt, "summaryContent")

		// Generate a conversation with K Q&A pairs (2 to 15)
		totalPairs := rapid.IntRange(2, 15).Draw(rt, "totalPairs")

		// Build full conversation history
		fullHistory := make([]ContextMessage, 0, totalPairs*2)
		for i := 0; i < totalPairs; i++ {
			q := rapid.StringMatching(`[a-zA-Z0-9 ?]{5,30}`).Draw(rt, fmt.Sprintf("q_%d", i))
			a := rapid.StringMatching(`[a-zA-Z0-9 .]{10,50}`).Draw(rt, fmt.Sprintf("a_%d", i))
			fullHistory = append(fullHistory, ContextMessage{Role: "user", Content: q})
			fullHistory = append(fullHistory, ContextMessage{Role: "assistant", Content: a})
		}

		// User edits message at position P (1-indexed, 1 ≤ P ≤ totalPairs)
		editPosition := rapid.IntRange(1, totalPairs).Draw(rt, "editPosition")

		// When editing at position P, context should only include pairs before P
		// Position P means the P-th Q&A pair (1-indexed)
		// Context = pairs 1..P-1 = messages[0..2*(P-1)-1]
		contextBeforeEdit := fullHistory[:2*(editPosition-1)]

		// The edited question
		editedQuestion := rapid.StringMatching(`[a-zA-Z0-9 ?]{5,30}`).Draw(rt, "editedQuestion")

		// Truncate context (should be a no-op for ≤ 20 pairs)
		truncated := svc.truncateContext(contextBeforeEdit)

		// Build the message array as the service would
		messages := svc.buildFollowupMessages(summaryContent, truncated, editedQuestion)

		// Verify: total messages = 1 (system) + 2*(P-1) (context) + 1 (edited question)
		expectedLen := 1 + 2*(editPosition-1) + 1
		if len(messages) != expectedLen {
			rt.Fatalf("expected %d messages for edit at position %d (totalPairs=%d), got %d",
				expectedLen, editPosition, totalPairs, len(messages))
		}

		// Verify: system message is first
		if messages[0].Role != "system" {
			rt.Fatalf("expected first message to be system, got %q", messages[0].Role)
		}

		// Verify: last message is the edited question
		lastMsg := messages[len(messages)-1]
		if lastMsg.Role != "user" || lastMsg.Content != editedQuestion {
			rt.Fatalf("expected last message to be edited question %q, got role=%q content=%q",
				editedQuestion, lastMsg.Role, lastMsg.Content)
		}

		// Verify: context messages match the pairs before the edit position
		for i := 0; i < 2*(editPosition-1); i++ {
			if messages[i+1].Role != contextBeforeEdit[i].Role {
				rt.Fatalf("context message[%d] role mismatch: expected %q, got %q",
					i, contextBeforeEdit[i].Role, messages[i+1].Role)
			}
			if messages[i+1].Content != contextBeforeEdit[i].Content {
				rt.Fatalf("context message[%d] content mismatch: expected %q, got %q",
					i, contextBeforeEdit[i].Content, messages[i+1].Content)
			}
		}

		// Verify: no messages from position P onwards are included
		// (the full history after 2*(P-1) should NOT appear)
		if editPosition < totalPairs {
			excludedContent := fullHistory[2*editPosition].Content // first message after edit
			for _, msg := range messages[1 : len(messages)-1] {
				if msg.Content == excludedContent && msg.Role == "user" {
					// Check it's not coincidentally the same as a prior message
					found := false
					for _, ctx := range contextBeforeEdit {
						if ctx.Content == excludedContent {
							found = true
							break
						}
					}
					if !found {
						rt.Fatalf("found excluded content %q in messages after edit", excludedContent)
					}
				}
			}
		}
	})
}

// Feature: ai-summary-followup, Property 7: Version number monotonic increment
// **Validates: Requirements 4.5, 4.7**
//
// Property: For any followup message, each regeneration SHALL create a new
// Message_Version with version_number equal to the current maximum version_number + 1,
// and all previously existing versions SHALL remain unchanged.
func TestProperty_VersionNumberMonotonicIncrement(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		db := setupFollowupServiceTestDB(t)
		followupRepo := repository.NewFollowupRepo(db)

		userID := createFollowupTestUser(t, db)
		summary := createCompletedSummary(t, db, userID, "")

		// Create a followup message
		msg := &model.FollowupMessage{
			SummaryID: summary.ID,
			Question:  "Test question",
			CreatedAt: time.Now(),
		}
		if err := followupRepo.CreateMessage(nil, msg); err != nil {
			rt.Fatalf("failed to create message: %v", err)
		}

		// Generate a random number of regenerations (2 to 10)
		numRegenerations := rapid.IntRange(2, 10).Draw(rt, "numRegenerations")

		// Track all created versions
		type versionSnapshot struct {
			ID            uint
			VersionNumber int
			Content       string
		}
		var allVersions []versionSnapshot

		for i := 0; i < numRegenerations; i++ {
			// Get next version number (as the service does)
			nextVersion, err := followupRepo.GetNextVersionNumber(nil, msg.ID)
			if err != nil {
				rt.Fatalf("GetNextVersionNumber failed at iteration %d: %v", i, err)
			}

			// Verify next version is i+1
			expectedVersion := i + 1
			if nextVersion != expectedVersion {
				rt.Fatalf("iteration %d: expected next version %d, got %d", i, expectedVersion, nextVersion)
			}

			// Create the version
			content := rapid.StringMatching(`[a-zA-Z0-9 .]{10,100}`).Draw(rt, fmt.Sprintf("content_%d", i))
			ver := &model.FollowupMessageVersion{
				FollowupMessageID: msg.ID,
				Content:           content,
				VersionNumber:     nextVersion,
				CreatedAt:         time.Now(),
			}
			if err := followupRepo.CreateVersion(nil, ver); err != nil {
				rt.Fatalf("CreateVersion failed at iteration %d: %v", i, err)
			}

			allVersions = append(allVersions, versionSnapshot{
				ID:            ver.ID,
				VersionNumber: nextVersion,
				Content:       content,
			})
		}

		// Verify all previously existing versions remain unchanged
		var storedVersions []model.FollowupMessageVersion
		if err := db.Where("followup_message_id = ?", msg.ID).
			Order("version_number ASC").
			Find(&storedVersions).Error; err != nil {
			rt.Fatalf("failed to query versions: %v", err)
		}

		if len(storedVersions) != numRegenerations {
			rt.Fatalf("expected %d versions, got %d", numRegenerations, len(storedVersions))
		}

		for i, stored := range storedVersions {
			expected := allVersions[i]
			if stored.VersionNumber != expected.VersionNumber {
				rt.Fatalf("version[%d]: expected version_number %d, got %d",
					i, expected.VersionNumber, stored.VersionNumber)
			}
			if stored.Content != expected.Content {
				rt.Fatalf("version[%d]: expected content %q, got %q",
					i, expected.Content, stored.Content)
			}
		}

		// Verify monotonic increment: each version_number = previous + 1
		for i := 1; i < len(storedVersions); i++ {
			if storedVersions[i].VersionNumber != storedVersions[i-1].VersionNumber+1 {
				rt.Fatalf("version numbers not monotonically incrementing: %d followed by %d",
					storedVersions[i-1].VersionNumber, storedVersions[i].VersionNumber)
			}
		}
	})
}

// Feature: ai-summary-followup, Property 11: Error response sanitization
// **Validates: Requirements 5.6**
//
// Property: For any LLM error response containing the configured API key or base URL,
// the error message returned to the client SHALL NOT contain the API key value or
// internal endpoint details.
func TestProperty_ErrorResponseSanitization(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random API key and base URL
		apiKey := rapid.StringMatching(`sk-[a-zA-Z0-9]{20,48}`).Draw(rt, "apiKey")
		baseURL := fmt.Sprintf("https://%s.openai.com/v1",
			rapid.StringMatching(`[a-z]{3,10}`).Draw(rt, "subdomain"))

		llmCfg := &config.LLMConfig{
			Model:   "test-model",
			BaseURL: baseURL,
			APIKey:  apiKey,
			Timeout: 5,
		}

		svc := &FollowupService{llmCfg: llmCfg}

		// Generate error messages that contain the API key and/or base URL
		scenario := rapid.IntRange(0, 3).Draw(rt, "scenario")

		var errMsg string
		switch scenario {
		case 0:
			// Error contains API key
			prefix := rapid.StringMatching(`[a-zA-Z ]{5,30}`).Draw(rt, "prefix")
			suffix := rapid.StringMatching(`[a-zA-Z ]{5,30}`).Draw(rt, "suffix")
			errMsg = prefix + apiKey + suffix
		case 1:
			// Error contains base URL
			prefix := rapid.StringMatching(`[a-zA-Z ]{5,30}`).Draw(rt, "prefix")
			suffix := rapid.StringMatching(`[a-zA-Z ]{5,30}`).Draw(rt, "suffix")
			errMsg = prefix + baseURL + suffix
		case 2:
			// Error contains both API key and base URL
			errMsg = fmt.Sprintf("request to %s failed with key %s: connection refused", baseURL, apiKey)
		case 3:
			// Error contains neither (should pass through unchanged)
			errMsg = rapid.StringMatching(`[a-zA-Z0-9 ]{10,50}`).Draw(rt, "safeError")
		}

		// Call sanitizeError
		inputErr := fmt.Errorf("%s", errMsg)
		sanitized := svc.sanitizeError(inputErr)

		if sanitized == nil {
			rt.Fatalf("sanitizeError returned nil for non-nil input")
		}

		sanitizedMsg := sanitized.Error()

		// Verify: sanitized message does NOT contain the API key
		if strings.Contains(sanitizedMsg, apiKey) {
			rt.Fatalf("sanitized error still contains API key %q.\nOriginal: %q\nSanitized: %q",
				apiKey, errMsg, sanitizedMsg)
		}

		// Verify: sanitized message does NOT contain the base URL
		if strings.Contains(sanitizedMsg, baseURL) {
			rt.Fatalf("sanitized error still contains base URL %q.\nOriginal: %q\nSanitized: %q",
				baseURL, errMsg, sanitizedMsg)
		}

		// Verify: for scenario 3 (no sensitive data), the error should pass through
		if scenario == 3 {
			if sanitizedMsg != errMsg {
				rt.Fatalf("expected error to pass through unchanged for safe message.\nExpected: %q\nGot: %q",
					errMsg, sanitizedMsg)
			}
		}

		// Verify: sanitizeError(nil) returns nil
		if svc.sanitizeError(nil) != nil {
			rt.Fatalf("sanitizeError(nil) should return nil")
		}
	})
}
