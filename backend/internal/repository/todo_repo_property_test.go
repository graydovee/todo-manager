package repository

import (
	"fmt"
	"sync/atomic"
	"testing"

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
		`CREATE TABLE IF NOT EXISTS todos (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, code TEXT NOT NULL, title TEXT NOT NULL, description TEXT DEFAULT '', category TEXT NOT NULL CHECK(category IN ('bug','feature','task')), priority TEXT NOT NULL DEFAULT 'p2' CHECK(priority IN ('p0','p1','p2','p3')), status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open','in_progress','completed')), due_at DATETIME, pinned INTEGER NOT NULL DEFAULT 0, highlighted INTEGER NOT NULL DEFAULT 0, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, UNIQUE(user_id, code))`,
		`CREATE TABLE IF NOT EXISTS todo_tags (id INTEGER PRIMARY KEY AUTOINCREMENT, todo_id INTEGER NOT NULL, tag TEXT NOT NULL, UNIQUE(todo_id, tag))`,
	}
	for _, ddl := range tables {
		if _, err := sqlDB.Exec(ddl); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	return db
}

// Feature: todo-enhancements, Property 3: Tag filter correctness
// **Validates: Requirements 2.5**
//
// Property: For any set of todos with various tags and any non-empty subset of
// filter tags, the filtered result SHALL contain exactly those todos that have at
// least one tag present in the filter set — no todos with a matching tag are
// excluded, and no todos without a matching tag are included.
func TestTagFilterCorrectness(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		db := setupRepoTestDB(t)
		todoRepo := NewTodoRepo(db)
		tagRepo := NewTagRepo(db)

		userID := uint(1)
		categories := []string{"bug", "feature", "task"}

		// Generate a pool of possible tags (3 to 8 distinct tags)
		numPoolTags := rapid.IntRange(3, 8).Draw(rt, "numPoolTags")
		tagPool := make([]string, 0, numPoolTags)
		tagPoolSet := make(map[string]bool)
		tagGen := rapid.StringMatching(`[a-z]{1,10}`)
		for len(tagPool) < numPoolTags {
			tag := tagGen.Draw(rt, fmt.Sprintf("poolTag_%d", len(tagPool)))
			if !tagPoolSet[tag] {
				tagPool = append(tagPool, tag)
				tagPoolSet[tag] = true
			}
		}

		// Generate random todos (2 to 10) with random subsets of the tag pool
		numTodos := rapid.IntRange(2, 10).Draw(rt, "numTodos")

		type todoInfo struct {
			id   uint
			tags map[string]bool
		}
		todos := make([]todoInfo, 0, numTodos)

		for i := range numTodos {
			cat := categories[i%3]
			todo := model.Todo{
				UserID:   userID,
				Code:     fmt.Sprintf("%d", i+1),
				Title:    fmt.Sprintf("Todo %d", i+1),
				Category: cat,
				Priority: "p2",
				Status:   "open",
			}
			if err := db.Create(&todo).Error; err != nil {
				rt.Fatalf("create todo %d: %v", i, err)
			}

			// Each todo gets a random subset of the tag pool (using Bool per tag)
			selectedTags := make(map[string]bool)
			for j, tag := range tagPool {
				if rapid.Bool().Draw(rt, fmt.Sprintf("include_tag_%d_%d", i, j)) {
					selectedTags[tag] = true
				}
			}

			tagList := make([]string, 0, len(selectedTags))
			for tag := range selectedTags {
				tagList = append(tagList, tag)
			}

			if err := tagRepo.ReplaceTags(nil, todo.ID, tagList); err != nil {
				rt.Fatalf("replace tags for todo %d: %v", i, err)
			}

			todos = append(todos, todoInfo{id: todo.ID, tags: selectedTags})
		}

		// Pick a random non-empty subset of the tag pool as the filter
		// Use Bool per tag, but ensure at least one is selected
		filterTags := make([]string, 0)
		filterTagSet := make(map[string]bool)
		for j, tag := range tagPool {
			if rapid.Bool().Draw(rt, fmt.Sprintf("filter_tag_%d", j)) {
				filterTags = append(filterTags, tag)
				filterTagSet[tag] = true
			}
		}
		// Ensure non-empty filter: if nothing was selected, pick the first tag
		if len(filterTags) == 0 {
			idx := rapid.IntRange(0, len(tagPool)-1).Draw(rt, "fallbackFilterIdx")
			filterTags = append(filterTags, tagPool[idx])
			filterTagSet[tagPool[idx]] = true
		}

		// Call TodoRepo.List with the tag filter
		filters := TodoFilters{
			Tags:     filterTags,
			Page:     1,
			PageSize: 100, // large enough to get all results
		}
		results, _, err := todoRepo.List(nil, userID, filters)
		if err != nil {
			rt.Fatalf("List with tag filter: %v", err)
		}

		// Compute expected set: todos that have at least one tag in the filter set
		expectedIDs := make(map[uint]bool)
		for _, ti := range todos {
			for tag := range ti.tags {
				if filterTagSet[tag] {
					expectedIDs[ti.id] = true
					break
				}
			}
		}

		// Verify: result contains exactly the expected todos
		resultIDs := make(map[uint]bool)
		for _, todo := range results {
			resultIDs[todo.ID] = true
		}

		// No missing todos
		for id := range expectedIDs {
			if !resultIDs[id] {
				rt.Fatalf("expected todo ID %d in results but it was missing\nfilter tags: %v\nexpected IDs: %v\ngot IDs: %v",
					id, filterTags, expectedIDs, resultIDs)
			}
		}

		// No extra todos
		for id := range resultIDs {
			if !expectedIDs[id] {
				rt.Fatalf("unexpected todo ID %d in results\nfilter tags: %v\nexpected IDs: %v\ngot IDs: %v",
					id, filterTags, expectedIDs, resultIDs)
			}
		}
	})
}
