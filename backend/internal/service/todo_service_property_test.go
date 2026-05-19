package service

import (
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite"
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
	// Each call gets a unique in-memory database by using a unique file name
	id := propertyTestDBCounter.Add(1)
	dsn := fmt.Sprintf("file:propdb_%d?mode=memory", id)
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

func setupPropertyService(t *testing.T) *TodoService {
	t.Helper()
	db := setupPropertyTestDB(t)
	return NewTodoService(
		db,
		repository.NewTodoRepo(db),
		repository.NewTagRepo(db),
		repository.NewRelationRepo(db),
		repository.NewCodeCounterRepo(db),
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
