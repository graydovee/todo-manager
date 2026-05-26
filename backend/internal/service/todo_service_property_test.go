package service

import (
	"fmt"
	"strconv"
	"strings"
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

var propertyTestDBCounter atomic.Int64

// setupPropertyTestDB creates an in-memory SQLite database with the new schema
// (code_counters without category column, single counter per user).
func setupPropertyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	// Each call gets a unique in-memory database by using a unique file name.
	// cache=shared ensures multiple connections see the same in-memory state.
	id := propertyTestDBCounter.Add(1)
	dsn := fmt.Sprintf("file:propdb_%d?mode=memory&cache=shared", id)
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
		`CREATE TABLE IF NOT EXISTS todo_relations (id INTEGER PRIMARY KEY AUTOINCREMENT, source_id INTEGER NOT NULL, target_id INTEGER NOT NULL, relation_type TEXT NOT NULL CHECK(relation_type IN ('depends_on','duplicate_of')), UNIQUE(source_id, target_id, relation_type))`,
		`CREATE TABLE IF NOT EXISTS code_counters (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, last_code INTEGER NOT NULL DEFAULT 0, UNIQUE(user_id))`,
		`CREATE TABLE IF NOT EXISTS sessions (id INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL, user_id INTEGER NOT NULL, data BLOB, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, expires_at DATETIME NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS comments (id INTEGER PRIMARY KEY AUTOINCREMENT, todo_id INTEGER NOT NULL, user_id INTEGER NOT NULL, content TEXT NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS todo_status_history (id INTEGER PRIMARY KEY AUTOINCREMENT, todo_id INTEGER NOT NULL, old_status TEXT NOT NULL, new_status TEXT NOT NULL, changed_at DATETIME NOT NULL)`,
	}
	for _, ddl := range tables {
		if _, err := sqlDB.Exec(ddl); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	return db
}

func setupPropertyService(t *testing.T) *TodoService {
	t.Helper()
	db := setupPropertyTestDB(t)
	return NewTodoService(
		db,
		repository.NewTodoRepo(db),
		repository.NewTagRepo(db),
		repository.NewRelationRepo(db),
		repository.NewCodeCounterRepo(db),
		repository.NewStatusHistoryRepo(db),
	)
}

// Feature: numbering-system-refactor, Property 1: Sequential code generation
// **Validates: Requirements 1.1, 1.3, 1.4**
//
// Property: For any user and any sequence of N todo creations (with arbitrary
// categories), the assigned codes SHALL be the strings "1", "2", "3", ..., "N"
// in creation order, forming a contiguous sequence with no gaps.
func TestProperty_SequentialCodeGeneration(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random number of todos to create (1 to 50)
		n := rapid.IntRange(1, 50).Draw(rt, "numTodos")

		// Generate random categories for each todo
		categories := make([]string, n)
		validCats := []string{"bug", "feature", "task"}
		for i := 0; i < n; i++ {
			categories[i] = rapid.SampledFrom(validCats).Draw(rt, fmt.Sprintf("category_%d", i))
		}

		// Each iteration gets a fresh isolated in-memory database
		svc := setupPropertyService(t)

		// Create N todos with random categories for a single user
		userID := uint(1)
		codes := make([]string, n)
		for i := 0; i < n; i++ {
			todo, err := svc.CreateTodo(userID, CreateTodoInput{
				Title:    fmt.Sprintf("Todo %d", i+1),
				Category: categories[i],
			})
			if err != nil {
				rt.Fatalf("failed to create todo %d: %v", i+1, err)
			}
			codes[i] = todo.Code
		}

		// Verify: codes must be "1", "2", "3", ..., "N" in creation order
		for i := 0; i < n; i++ {
			expected := strconv.Itoa(i + 1)
			if codes[i] != expected {
				rt.Fatalf("todo %d: expected code %q, got %q (categories: %v)", i+1, expected, codes[i], categories)
			}
		}
	})
}

// Feature: numbering-system-refactor, Property 3: Codes are never reused after deletion
// **Validates: Requirements 1.6**
//
// Property: For any sequence of create and delete operations for a user, if a
// todo with code "K" is deleted and a new todo is subsequently created, the new
// todo's code SHALL be strictly greater than K (as integers).
func TestProperty_CodesNeverReusedAfterDeletion(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Each iteration gets a fresh isolated in-memory database
		svc := setupPropertyService(t)
		userID := uint(1)

		categories := []string{"bug", "feature", "task"}

		// Generate a random sequence of operations (create and delete)
		numOps := rapid.IntRange(3, 20).Draw(rt, "numOps")

		type todoRecord struct {
			id   uint
			code int
		}

		var activeTodos []todoRecord
		var deletedCodes []int

		for i := 0; i < numOps; i++ {
			canDelete := len(activeTodos) > 0
			shouldCreate := !canDelete || rapid.Bool().Draw(rt, fmt.Sprintf("op_%d_is_create", i))

			if shouldCreate {
				cat := categories[rapid.IntRange(0, 2).Draw(rt, fmt.Sprintf("cat_%d", i))]
				todo, err := svc.CreateTodo(userID, CreateTodoInput{
					Title:    fmt.Sprintf("Todo %d", i),
					Category: cat,
				})
				if err != nil {
					rt.Fatalf("create todo at op %d: %v", i, err)
				}
				codeInt, err := strconv.Atoi(todo.Code)
				if err != nil {
					rt.Fatalf("code is not numeric: %s", todo.Code)
				}
				activeTodos = append(activeTodos, todoRecord{id: todo.ID, code: codeInt})
			} else {
				// Delete a random existing todo
				idx := rapid.IntRange(0, len(activeTodos)-1).Draw(rt, fmt.Sprintf("del_idx_%d", i))
				todoToDelete := activeTodos[idx]

				err := svc.DeleteTodo(userID, todoToDelete.id)
				if err != nil {
					rt.Fatalf("delete todo at op %d: %v", i, err)
				}

				deletedCodes = append(deletedCodes, todoToDelete.code)
				// Remove from active list
				activeTodos = append(activeTodos[:idx], activeTodos[idx+1:]...)
			}
		}

		// After the sequence, if any deletions occurred, create a new todo
		// and verify its code is strictly greater than all deleted codes
		if len(deletedCodes) > 0 {
			todo, err := svc.CreateTodo(userID, CreateTodoInput{
				Title:    "Final todo after deletions",
				Category: "bug",
			})
			if err != nil {
				rt.Fatalf("create final todo: %v", err)
			}
			newCode, err := strconv.Atoi(todo.Code)
			if err != nil {
				rt.Fatalf("final code is not numeric: %s", todo.Code)
			}

			for _, deletedCode := range deletedCodes {
				if newCode <= deletedCode {
					rt.Fatalf("code reuse detected: new code %d is not strictly greater than deleted code %d", newCode, deletedCode)
				}
			}
		}
	})
}

// Feature: todo-enhancements, Property 5: Tag normalization from input
// **Validates: Requirements 3.4, 3.5**
//
// Property: For any comma-separated input string, the resulting tag list SHALL
// be equivalent to: splitting by comma, trimming whitespace from each segment,
// lowercasing each segment, discarding empty segments, truncating each to 100
// characters, and removing duplicates — preserving first-occurrence order.
func TestProperty_TagNormalization(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random number of segments (0 to 20)
		numSegments := rapid.IntRange(0, 20).Draw(rt, "numSegments")

		// Build segments with random whitespace, mixed case, possible duplicates, and long strings
		segments := make([]string, numSegments)
		for i := 0; i < numSegments; i++ {
			// Choose segment type
			segType := rapid.IntRange(0, 5).Draw(rt, fmt.Sprintf("segType_%d", i))
			switch segType {
			case 0:
				// Normal tag with mixed case
				segments[i] = rapid.StringMatching(`[a-zA-Z0-9_-]{1,20}`).Draw(rt, fmt.Sprintf("tag_%d", i))
			case 1:
				// Tag with leading/trailing whitespace
				tag := rapid.StringMatching(`[a-zA-Z0-9]{1,10}`).Draw(rt, fmt.Sprintf("tag_%d", i))
				leadingSpaces := rapid.StringMatching(`[ \t]{0,5}`).Draw(rt, fmt.Sprintf("lead_%d", i))
				trailingSpaces := rapid.StringMatching(`[ \t]{0,5}`).Draw(rt, fmt.Sprintf("trail_%d", i))
				segments[i] = leadingSpaces + tag + trailingSpaces
			case 2:
				// Empty or whitespace-only segment
				segments[i] = rapid.StringMatching(`[ \t]{0,5}`).Draw(rt, fmt.Sprintf("empty_%d", i))
			case 3:
				// Long segment (over 100 chars)
				segments[i] = rapid.StringMatching(`[a-z]{101,150}`).Draw(rt, fmt.Sprintf("long_%d", i))
			case 4:
				// Duplicate of a previous segment (if possible)
				if i > 0 {
					dupIdx := rapid.IntRange(0, i-1).Draw(rt, fmt.Sprintf("dupIdx_%d", i))
					segments[i] = segments[dupIdx]
				} else {
					segments[i] = rapid.StringMatching(`[a-z]{1,10}`).Draw(rt, fmt.Sprintf("tag_%d", i))
				}
			case 5:
				// Mixed case variant of a previous segment (to test case-insensitive dedup)
				if i > 0 {
					dupIdx := rapid.IntRange(0, i-1).Draw(rt, fmt.Sprintf("dupIdx_%d", i))
					segments[i] = strings.ToUpper(segments[dupIdx])
				} else {
					segments[i] = rapid.StringMatching(`[A-Z]{1,10}`).Draw(rt, fmt.Sprintf("tag_%d", i))
				}
			}
		}

		// Join segments with commas to form the input string
		input := strings.Join(segments, ",")

		// Split by comma to get the tag slice (simulating what the handler does)
		var inputTags []string
		if input != "" {
			inputTags = strings.Split(input, ",")
		}

		// Call the actual normalizeTags function
		actual := normalizeTags(inputTags)

		// Compute expected result using the reference implementation
		expected := referenceNormalize(inputTags)

		// Verify lengths match
		if len(actual) != len(expected) {
			rt.Fatalf("length mismatch: got %d, want %d\ninput: %q\nactual: %v\nexpected: %v",
				len(actual), len(expected), input, actual, expected)
		}

		// Verify each element matches
		for i := range expected {
			if actual[i] != expected[i] {
				rt.Fatalf("mismatch at index %d: got %q, want %q\ninput: %q\nactual: %v\nexpected: %v",
					i, actual[i], expected[i], input, actual, expected)
			}
		}

		// Additional property checks:
		// 1. All results are trimmed (no leading/trailing whitespace)
		for i, tag := range actual {
			if tag != strings.TrimSpace(tag) {
				rt.Fatalf("tag at index %d has untrimmed whitespace: %q", i, tag)
			}
		}

		// 2. All results are lowercase
		for i, tag := range actual {
			if tag != strings.ToLower(tag) {
				rt.Fatalf("tag at index %d is not lowercase: %q", i, tag)
			}
		}

		// 3. No empty strings in result
		for i, tag := range actual {
			if tag == "" {
				rt.Fatalf("tag at index %d is empty", i)
			}
		}

		// 4. No duplicates in result
		seen := make(map[string]bool)
		for i, tag := range actual {
			if seen[tag] {
				rt.Fatalf("duplicate tag at index %d: %q", i, tag)
			}
			seen[tag] = true
		}

		// 5. All tags are at most 100 runes
		for i, tag := range actual {
			if len([]rune(tag)) > 100 {
				rt.Fatalf("tag at index %d exceeds 100 characters: len=%d", i, len([]rune(tag)))
			}
		}
	})
}

// referenceNormalize is the reference implementation of the tag normalization
// property: split, trim, lowercase, discard empty, truncate to 100 chars, dedup
// preserving first-occurrence order.
func referenceNormalize(tags []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, tag := range tags {
		t := strings.TrimSpace(strings.ToLower(tag))
		if t == "" {
			continue
		}
		// Truncate to 100 runes
		runes := []rune(t)
		if len(runes) > 100 {
			t = string(runes[:100])
		}
		if !seen[t] {
			seen[t] = true
			result = append(result, t)
		}
	}
	return result
}

// Feature: ai-summary-enhancement, Property 1: Activity refreshes updated_at
// **Validates: Requirements 1.1, 1.2, 1.3**
//
// Property: For any todo and any activity (status change, comment addition, or
// field update), the todo's updated_at timestamp SHALL be greater than or equal
// to the timestamp before the activity occurred.
func TestProperty_ActivityRefreshesUpdatedAt(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		db := setupPropertyTestDB(t)
		todoRepo := repository.NewTodoRepo(db)
		tagRepo := repository.NewTagRepo(db)
		relationRepo := repository.NewRelationRepo(db)
		counterRepo := repository.NewCodeCounterRepo(db)
		statusHistoryRepo := repository.NewStatusHistoryRepo(db)
		commentRepo := repository.NewCommentRepo(db)

		todoSvc := NewTodoService(db, todoRepo, tagRepo, relationRepo, counterRepo, statusHistoryRepo)
		commentSvc := NewCommentService(db, commentRepo, todoRepo)

		userID := uint(1)
		categories := []string{"bug", "feature", "task"}
		cat := rapid.SampledFrom(categories).Draw(rt, "category")

		// Create a todo
		todo, err := todoSvc.CreateTodo(userID, CreateTodoInput{
			Title:    "Test Todo",
			Category: cat,
		})
		if err != nil {
			rt.Fatalf("failed to create todo: %v", err)
		}

		// Choose a random activity type: 0=status change, 1=comment addition, 2=field update
		activityType := rapid.IntRange(0, 2).Draw(rt, "activityType")

		// Record the updated_at before the activity
		beforeTodo, err := todoRepo.FindByID(nil, todo.ID, userID)
		if err != nil {
			rt.Fatalf("failed to find todo: %v", err)
		}
		updatedAtBefore := beforeTodo.UpdatedAt

		// Small sleep to ensure time difference is measurable
		time.Sleep(time.Millisecond)

		switch activityType {
		case 0:
			// Status change: open -> in_progress
			err = todoSvc.StartTodo(userID, todo.ID)
			if err != nil {
				rt.Fatalf("failed to start todo: %v", err)
			}
		case 1:
			// Comment addition
			content := rapid.StringMatching(`[a-zA-Z0-9 ]{1,50}`).Draw(rt, "commentContent")
			_, err = commentSvc.Create(userID, todo.ID, content)
			if err != nil {
				rt.Fatalf("failed to create comment: %v", err)
			}
		case 2:
			// Field update (title change)
			newTitle := rapid.StringMatching(`[a-zA-Z0-9 ]{1,50}`).Draw(rt, "newTitle")
			_, err = todoSvc.UpdateTodo(userID, todo.ID, UpdateTodoInput{
				Title: &newTitle,
			})
			if err != nil {
				rt.Fatalf("failed to update todo: %v", err)
			}
		}

		// Verify updated_at is refreshed
		afterTodo, err := todoRepo.FindByID(nil, todo.ID, userID)
		if err != nil {
			rt.Fatalf("failed to find todo after activity: %v", err)
		}

		if afterTodo.UpdatedAt.Before(updatedAtBefore) {
			rt.Fatalf("updated_at was not refreshed: before=%v, after=%v, activityType=%d",
				updatedAtBefore, afterTodo.UpdatedAt, activityType)
		}
	})
}

// Feature: ai-summary-enhancement, Property 3: Status history records transition on status change
// **Validates: Requirements 2.1**
//
// Property: For any todo and any valid status transition, a TodoStatusHistory
// record SHALL be created with the correct todo_id, old_status equal to the
// previous status, new_status equal to the new status, and changed_at equal to
// the time of the change.
func TestProperty_StatusHistoryRecordsTransition(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		db := setupPropertyTestDB(t)
		todoRepo := repository.NewTodoRepo(db)
		tagRepo := repository.NewTagRepo(db)
		relationRepo := repository.NewRelationRepo(db)
		counterRepo := repository.NewCodeCounterRepo(db)
		statusHistoryRepo := repository.NewStatusHistoryRepo(db)

		todoSvc := NewTodoService(db, todoRepo, tagRepo, relationRepo, counterRepo, statusHistoryRepo)

		userID := uint(1)
		categories := []string{"bug", "feature", "task"}
		cat := rapid.SampledFrom(categories).Draw(rt, "category")

		// Create a todo (starts as "open")
		todo, err := todoSvc.CreateTodo(userID, CreateTodoInput{
			Title:    "Test Todo",
			Category: cat,
		})
		if err != nil {
			rt.Fatalf("failed to create todo: %v", err)
		}

		// Choose a random valid transition from the current status
		// Valid transitions: open->in_progress, open->completed, in_progress->completed,
		// in_progress->open (via SetStatus), completed->open (via Reopen)
		type transition struct {
			from string
			to   string
		}
		validTransitions := []transition{
			{model.StatusOpen, model.StatusInProgress},
			{model.StatusOpen, model.StatusCompleted},
			{model.StatusInProgress, model.StatusCompleted},
			{model.StatusInProgress, model.StatusOpen},
		}

		chosenTransition := rapid.SampledFrom(validTransitions).Draw(rt, "transition")

		// First, get the todo to the "from" state if needed
		if chosenTransition.from == model.StatusInProgress {
			// Need to start the todo first
			err = todoSvc.StartTodo(userID, todo.ID)
			if err != nil {
				rt.Fatalf("failed to start todo: %v", err)
			}
		}

		// Record time before the transition
		timeBefore := time.Now()
		time.Sleep(time.Millisecond)

		// Perform the transition
		switch {
		case chosenTransition.from == model.StatusOpen && chosenTransition.to == model.StatusInProgress:
			err = todoSvc.StartTodo(userID, todo.ID)
		case chosenTransition.from == model.StatusOpen && chosenTransition.to == model.StatusCompleted:
			_, err = todoSvc.CompleteTodo(userID, todo.ID, false)
		case chosenTransition.from == model.StatusInProgress && chosenTransition.to == model.StatusCompleted:
			_, err = todoSvc.CompleteTodo(userID, todo.ID, false)
		case chosenTransition.from == model.StatusInProgress && chosenTransition.to == model.StatusOpen:
			err = todoSvc.SetStatus(userID, todo.ID, model.StatusOpen)
		}
		if err != nil {
			rt.Fatalf("failed to perform transition %s->%s: %v", chosenTransition.from, chosenTransition.to, err)
		}

		timeAfter := time.Now()

		// Verify the status history record was created
		records, err := statusHistoryRepo.FindByTodoID(nil, todo.ID)
		if err != nil {
			rt.Fatalf("failed to find status history: %v", err)
		}

		// Find the record matching our transition (last record should be it)
		var found bool
		for _, rec := range records {
			if rec.OldStatus == chosenTransition.from && rec.NewStatus == chosenTransition.to {
				// Verify todo_id
				if rec.TodoID != todo.ID {
					rt.Fatalf("history record has wrong todo_id: got %d, want %d", rec.TodoID, todo.ID)
				}
				// Verify changed_at is within the expected time window
				if rec.ChangedAt.Before(timeBefore) || rec.ChangedAt.After(timeAfter) {
					rt.Fatalf("history record changed_at %v is outside expected range [%v, %v]",
						rec.ChangedAt, timeBefore, timeAfter)
				}
				found = true
				break
			}
		}

		if !found {
			rt.Fatalf("no status history record found for transition %s->%s (todo_id=%d, total records=%d)",
				chosenTransition.from, chosenTransition.to, todo.ID, len(records))
		}
	})
}

// Feature: ai-summary-enhancement, Property 4: Status history records initial status on creation
// **Validates: Requirements 2.3**
//
// Property: For any newly created todo, a TodoStatusHistory record SHALL exist
// with old_status as empty string and new_status equal to the todo's initial
// status value.
func TestProperty_StatusHistoryRecordsInitialStatus(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		db := setupPropertyTestDB(t)
		todoRepo := repository.NewTodoRepo(db)
		tagRepo := repository.NewTagRepo(db)
		relationRepo := repository.NewRelationRepo(db)
		counterRepo := repository.NewCodeCounterRepo(db)
		statusHistoryRepo := repository.NewStatusHistoryRepo(db)

		todoSvc := NewTodoService(db, todoRepo, tagRepo, relationRepo, counterRepo, statusHistoryRepo)

		userID := uint(1)
		categories := []string{"bug", "feature", "task"}
		cat := rapid.SampledFrom(categories).Draw(rt, "category")
		title := rapid.StringMatching(`[a-zA-Z0-9 ]{1,50}`).Draw(rt, "title")

		// Create a todo
		todo, err := todoSvc.CreateTodo(userID, CreateTodoInput{
			Title:    title,
			Category: cat,
		})
		if err != nil {
			rt.Fatalf("failed to create todo: %v", err)
		}

		// Verify a status history record exists with old_status="" and new_status=initial status
		records, err := statusHistoryRepo.FindByTodoID(nil, todo.ID)
		if err != nil {
			rt.Fatalf("failed to find status history: %v", err)
		}

		if len(records) == 0 {
			rt.Fatalf("no status history records found for newly created todo (id=%d)", todo.ID)
		}

		// The first record should be the initial status entry
		initialRecord := records[0]
		if initialRecord.OldStatus != "" {
			rt.Fatalf("initial status history record has non-empty old_status: %q", initialRecord.OldStatus)
		}
		if initialRecord.NewStatus != model.StatusOpen {
			rt.Fatalf("initial status history record has wrong new_status: got %q, want %q",
				initialRecord.NewStatus, model.StatusOpen)
		}
		if initialRecord.TodoID != todo.ID {
			rt.Fatalf("initial status history record has wrong todo_id: got %d, want %d",
				initialRecord.TodoID, todo.ID)
		}
	})
}

// Feature: todo-filter-duplicate, Property 5: Completing canonical cascades to all duplicates
// **Validates: Requirements 3.7**
//
// Property: For any canonical todo with 1-10 duplicates, when the canonical todo
// is completed via CompleteTodo, all duplicate todos SHALL have their status set
// to "completed".
func TestProperty_CascadeCompleteDuplicates(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc := setupPropertyService(t)
		userID := uint(1)

		// Create the canonical todo
		canonical, err := svc.CreateTodo(userID, CreateTodoInput{
			Title:    "Canonical Todo",
			Category: "task",
		})
		if err != nil {
			rt.Fatalf("failed to create canonical todo: %v", err)
		}

		// Generate 1-10 duplicate todos pointing to the canonical
		numDuplicates := rapid.IntRange(1, 10).Draw(rt, "numDuplicates")
		duplicateIDs := make([]uint, 0, numDuplicates)

		for i := range numDuplicates {
			dup, err := svc.CreateTodo(userID, CreateTodoInput{
				Title:         fmt.Sprintf("Duplicate %d", i+1),
				Category:      "task",
				DuplicateOfID: &canonical.ID,
			})
			if err != nil {
				rt.Fatalf("failed to create duplicate %d: %v", i+1, err)
			}
			// Verify the duplicate was created with duplicate status
			if dup.Status != model.StatusDuplicate {
				rt.Fatalf("duplicate %d should have status 'duplicate', got %q", i+1, dup.Status)
			}
			duplicateIDs = append(duplicateIDs, dup.ID)
		}

		// Complete the canonical todo
		_, err = svc.CompleteTodo(userID, canonical.ID, false)
		if err != nil {
			rt.Fatalf("failed to complete canonical todo: %v", err)
		}

		// Verify all duplicates are now completed
		for i, dupID := range duplicateIDs {
			dupTodo, err := svc.GetTodo(userID, dupID)
			if err != nil {
				rt.Fatalf("failed to get duplicate %d (id=%d): %v", i+1, dupID, err)
			}
			if dupTodo.Status != model.StatusCompleted {
				rt.Fatalf("duplicate %d (id=%d) should have status 'completed' after canonical completion, got %q",
					i+1, dupID, dupTodo.Status)
			}
		}

		// Also verify the canonical itself is completed
		canonicalAfter, err := svc.GetTodo(userID, canonical.ID)
		if err != nil {
			rt.Fatalf("failed to get canonical todo after completion: %v", err)
		}
		if canonicalAfter.Status != model.StatusCompleted {
			rt.Fatalf("canonical todo should have status 'completed', got %q", canonicalAfter.Status)
		}
	})
}

// Feature: todo-filter-duplicate, Property 6: Reopening canonical cascades to all duplicates
// **Validates: Requirements 3.8**
//
// Property: For any canonical todo that has one or more completed duplicate todos,
// when the canonical todo is reopened via ReopenTodo, all of its duplicate todos
// SHALL have their status set to "open".
func TestProperty_CascadeReopenDuplicates(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc := setupPropertyService(t)
		userID := uint(1)

		// Create the canonical todo
		canonical, err := svc.CreateTodo(userID, CreateTodoInput{
			Title:    "Canonical Todo",
			Category: "task",
		})
		if err != nil {
			rt.Fatalf("failed to create canonical todo: %v", err)
		}

		// Generate 1-10 duplicate todos pointing to the canonical
		numDuplicates := rapid.IntRange(1, 10).Draw(rt, "numDuplicates")
		duplicateIDs := make([]uint, 0, numDuplicates)

		for i := range numDuplicates {
			dup, err := svc.CreateTodo(userID, CreateTodoInput{
				Title:         fmt.Sprintf("Duplicate %d", i+1),
				Category:      "task",
				DuplicateOfID: &canonical.ID,
			})
			if err != nil {
				rt.Fatalf("failed to create duplicate %d: %v", i+1, err)
			}
			// Verify the duplicate was created with duplicate status
			if dup.Status != model.StatusDuplicate {
				rt.Fatalf("duplicate %d should have status 'duplicate', got %q", i+1, dup.Status)
			}
			duplicateIDs = append(duplicateIDs, dup.ID)
		}

		// Complete the canonical todo (this cascades to all duplicates)
		_, err = svc.CompleteTodo(userID, canonical.ID, false)
		if err != nil {
			rt.Fatalf("failed to complete canonical todo: %v", err)
		}

		// Verify all duplicates are now completed (precondition for reopen test)
		for i, dupID := range duplicateIDs {
			dupTodo, err := svc.GetTodo(userID, dupID)
			if err != nil {
				rt.Fatalf("failed to get duplicate %d (id=%d) after complete: %v", i+1, dupID, err)
			}
			if dupTodo.Status != model.StatusCompleted {
				rt.Fatalf("duplicate %d (id=%d) should be 'completed' before reopen, got %q",
					i+1, dupID, dupTodo.Status)
			}
		}

		// Reopen the canonical todo (this should cascade to all duplicates)
		_, err = svc.ReopenTodo(userID, canonical.ID, false)
		if err != nil {
			rt.Fatalf("failed to reopen canonical todo: %v", err)
		}

		// Verify all duplicates are now open
		for i, dupID := range duplicateIDs {
			dupTodo, err := svc.GetTodo(userID, dupID)
			if err != nil {
				rt.Fatalf("failed to get duplicate %d (id=%d) after reopen: %v", i+1, dupID, err)
			}
			if dupTodo.Status != model.StatusOpen {
				rt.Fatalf("duplicate %d (id=%d) should have status 'open' after canonical reopen, got %q",
					i+1, dupID, dupTodo.Status)
			}
		}

		// Also verify the canonical itself is open
		canonicalAfter, err := svc.GetTodo(userID, canonical.ID)
		if err != nil {
			rt.Fatalf("failed to get canonical todo after reopen: %v", err)
		}
		if canonicalAfter.Status != model.StatusOpen {
			rt.Fatalf("canonical todo should have status 'open', got %q", canonicalAfter.Status)
		}
	})
}
