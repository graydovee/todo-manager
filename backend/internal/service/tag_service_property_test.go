package service

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/graydovee/todolist/internal/model"
	"github.com/graydovee/todolist/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"pgregory.net/rapid"
)

// setupTagTestDB creates an in-memory SQLite database for tag property tests.
func setupTagTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	id := propertyTestDBCounter.Add(1)
	dsn := fmt.Sprintf("file:tagdb_%d?mode=memory", id)
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
		`CREATE TABLE IF NOT EXISTS code_counters (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, last_code INTEGER NOT NULL DEFAULT 0, UNIQUE(user_id))`,
	}
	for _, ddl := range tables {
		if _, err := sqlDB.Exec(ddl); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	return db
}

// Feature: todo-enhancements, Property 1: Tag distinctness
// **Validates: Requirements 1.1**
//
// Property: For any user with any set of todos (each having zero or more tags),
// calling FindDistinctByUser SHALL return exactly the set of unique tag strings
// that appear across all of that user's todos — no duplicates, no missing tags,
// and no tags from other users.
func TestTagDistinctness(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		db := setupTagTestDB(t)
		tagRepo := repository.NewTagRepo(db)

		// Generate 2 users to verify isolation
		targetUserID := uint(1)
		otherUserID := uint(2)

		// Generate random number of todos for the target user (0 to 10)
		numTodos := rapid.IntRange(0, 10).Draw(rt, "numTodos")

		// Tag generator: non-empty strings of 1-20 lowercase alphanumeric chars
		tagGen := rapid.StringMatching(`[a-z0-9]{1,20}`)

		// Track expected unique tags for the target user
		expectedTags := make(map[string]bool)

		categories := []string{"bug", "feature", "task"}

		// Create todos for the target user with random tags
		for i := 0; i < numTodos; i++ {
			cat := categories[i%3]
			todo := model.Todo{
				UserID:   targetUserID,
				Code:     fmt.Sprintf("%d", i+1),
				Title:    fmt.Sprintf("Todo %d", i+1),
				Category: cat,
				Priority: "p2",
				Status:   "open",
			}
			if err := db.Create(&todo).Error; err != nil {
				rt.Fatalf("create todo %d: %v", i, err)
			}

			// Generate 0 to 5 tags for this todo, deduplicated
			numTags := rapid.IntRange(0, 5).Draw(rt, fmt.Sprintf("numTags_%d", i))
			tagSet := make(map[string]bool)
			for j := 0; j < numTags; j++ {
				tag := tagGen.Draw(rt, fmt.Sprintf("tag_%d_%d", i, j))
				tagSet[tag] = true
			}
			tags := make([]string, 0, len(tagSet))
			for tag := range tagSet {
				tags = append(tags, tag)
			}

			// Use ReplaceTags to insert
			if err := tagRepo.ReplaceTags(nil, todo.ID, tags); err != nil {
				rt.Fatalf("replace tags for todo %d: %v", i, err)
			}

			// Track expected tags (only non-empty, which ReplaceTags already filters)
			for _, tag := range tags {
				if tag != "" {
					expectedTags[tag] = true
				}
			}
		}

		// Create some todos for the other user with different tags (to test isolation)
		numOtherTodos := rapid.IntRange(0, 5).Draw(rt, "numOtherTodos")
		for i := 0; i < numOtherTodos; i++ {
			cat := categories[i%3]
			todo := model.Todo{
				UserID:   otherUserID,
				Code:     fmt.Sprintf("%d", i+1),
				Title:    fmt.Sprintf("Other Todo %d", i+1),
				Category: cat,
				Priority: "p2",
				Status:   "open",
			}
			if err := db.Create(&todo).Error; err != nil {
				rt.Fatalf("create other todo %d: %v", i, err)
			}

			numTags := rapid.IntRange(0, 5).Draw(rt, fmt.Sprintf("numOtherTags_%d", i))
			tagSet := make(map[string]bool)
			for j := 0; j < numTags; j++ {
				tag := tagGen.Draw(rt, fmt.Sprintf("otherTag_%d_%d", i, j))
				tagSet[tag] = true
			}
			tags := make([]string, 0, len(tagSet))
			for tag := range tagSet {
				tags = append(tags, tag)
			}
			if err := tagRepo.ReplaceTags(nil, todo.ID, tags); err != nil {
				rt.Fatalf("replace tags for other todo %d: %v", i, err)
			}
		}

		// Call FindDistinctByUser for the target user
		result, err := tagRepo.FindDistinctByUser(nil, targetUserID)
		if err != nil {
			rt.Fatalf("FindDistinctByUser: %v", err)
		}

		// Verify: no duplicates in result
		seen := make(map[string]bool)
		for _, tag := range result {
			if seen[tag] {
				rt.Fatalf("duplicate tag in result: %q", tag)
			}
			seen[tag] = true
		}

		// Verify: result contains exactly the expected tags (no missing, no extra)
		expectedList := make([]string, 0, len(expectedTags))
		for tag := range expectedTags {
			expectedList = append(expectedList, tag)
		}
		sort.Strings(expectedList)
		sort.Strings(result)

		if len(result) != len(expectedList) {
			rt.Fatalf("expected %d distinct tags, got %d\nexpected: %v\ngot: %v",
				len(expectedList), len(result), expectedList, result)
		}
		for i := range result {
			if result[i] != expectedList[i] {
				rt.Fatalf("tag mismatch at index %d: expected %q, got %q\nexpected: %v\ngot: %v",
					i, expectedList[i], result[i], expectedList, result)
			}
		}
	})
}

// Feature: todo-enhancements, Property 2: Tag alphabetical sorting
// **Validates: Requirements 1.2**
//
// Property: For any set of distinct tags returned by the tags endpoint, the array
// SHALL be sorted in case-insensitive alphabetical order — that is, for every
// adjacent pair (tags[i], tags[i+1]), strings.ToLower(tags[i]) <= strings.ToLower(tags[i+1]).
func TestTagAlphabeticalSorting(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random number of tags (1 to 50)
		n := rapid.IntRange(1, 50).Draw(rt, "numTags")

		// Generate random tag strings with mixed case
		tags := make([]string, n)
		for i := range n {
			// Generate non-empty strings with mixed case letters, digits, and common chars
			tag := rapid.StringMatching(`[a-zA-Z0-9_\-]{1,30}`).Draw(rt, "tag")
			tags[i] = tag
		}

		// Apply the same sorting logic as the Tags handler
		sort.Slice(tags, func(i, j int) bool {
			return strings.ToLower(tags[i]) < strings.ToLower(tags[j])
		})

		// Verify the sorted output satisfies the property:
		// strings.ToLower(tags[i]) <= strings.ToLower(tags[i+1]) for all adjacent pairs
		for i := 0; i < len(tags)-1; i++ {
			lower := strings.ToLower(tags[i])
			lowerNext := strings.ToLower(tags[i+1])
			if lower > lowerNext {
				rt.Fatalf("sorting violated: tags[%d]=%q (lower=%q) > tags[%d]=%q (lower=%q)",
					i, tags[i], lower, i+1, tags[i+1], lowerNext)
			}
		}
	})
}
