package service

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/graydovee/todo-manager/internal/model"
	"github.com/graydovee/todo-manager/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"pgregory.net/rapid"
)

var duplicatePropertyTestDBCounter atomic.Int64

// setupDuplicatePropertyTestDB creates an in-memory SQLite database with the
// schema that includes 'duplicate' in the status CHECK constraint.
func setupDuplicatePropertyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	id := duplicatePropertyTestDBCounter.Add(1)
	dsn := fmt.Sprintf("file:dupdb_%d?mode=memory&cache=shared", id)
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
		`CREATE TABLE IF NOT EXISTS todo_status_history (id INTEGER PRIMARY KEY AUTOINCREMENT, todo_id INTEGER NOT NULL, old_status TEXT NOT NULL, new_status TEXT NOT NULL, changed_at DATETIME NOT NULL)`,
	}
	for _, ddl := range tables {
		if _, err := sqlDB.Exec(ddl); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	return db
}

func setupDuplicatePropertyService(t *testing.T) (*TodoService, *gorm.DB) {
	t.Helper()
	db := setupDuplicatePropertyTestDB(t)
	svc := NewTodoService(
		db,
		repository.NewTodoRepo(db),
		repository.NewTagRepo(db),
		repository.NewRelationRepo(db),
		repository.NewCodeCounterRepo(db),
		repository.NewStatusHistoryRepo(db),
	)
	return svc, db
}

// Feature: todo-filter-duplicate, Property 3: Duplicate target must be canonical
// **Validates: Requirements 3.3, 3.9**
//
// Property: For any todo A and for any todo B, if B already has a duplicate_of
// relation pointing to another todo, then attempting to mark A as a duplicate of B
// SHALL be rejected with an error "duplicate target must be canonical".
func TestProperty_DuplicateTargetMustBeCanonical(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, _ := setupDuplicatePropertyService(t)
		userID := uint(1)

		categories := []string{"bug", "feature", "task"}
		priorities := []string{"p0", "p1", "p2", "p3"}

		// Generate random attributes for three todos
		catC := rapid.SampledFrom(categories).Draw(rt, "catC")
		catB := rapid.SampledFrom(categories).Draw(rt, "catB")
		catA := rapid.SampledFrom(categories).Draw(rt, "catA")
		priC := rapid.SampledFrom(priorities).Draw(rt, "priC")
		priB := rapid.SampledFrom(priorities).Draw(rt, "priB")
		priA := rapid.SampledFrom(priorities).Draw(rt, "priA")
		titleC := rapid.StringMatching(`[a-zA-Z0-9 ]{1,30}`).Draw(rt, "titleC")
		titleB := rapid.StringMatching(`[a-zA-Z0-9 ]{1,30}`).Draw(rt, "titleB")
		titleA := rapid.StringMatching(`[a-zA-Z0-9 ]{1,30}`).Draw(rt, "titleA")

		// Create todo C (the canonical root)
		todoC, err := svc.CreateTodo(userID, CreateTodoInput{
			Title:    titleC,
			Category: catC,
			Priority: priC,
		})
		if err != nil {
			rt.Fatalf("failed to create todo C: %v", err)
		}

		// Create todo B
		todoB, err := svc.CreateTodo(userID, CreateTodoInput{
			Title:    titleB,
			Category: catB,
			Priority: priB,
		})
		if err != nil {
			rt.Fatalf("failed to create todo B: %v", err)
		}

		// Create todo A
		todoA, err := svc.CreateTodo(userID, CreateTodoInput{
			Title:    titleA,
			Category: catA,
			Priority: priA,
		})
		if err != nil {
			rt.Fatalf("failed to create todo A: %v", err)
		}

		// Mark B as duplicate of C (B is now non-canonical)
		dupTargetC := todoC.ID
		_, err = svc.UpdateTodo(userID, todoB.ID, UpdateTodoInput{
			DuplicateOfID: &dupTargetC,
		})
		if err != nil {
			rt.Fatalf("failed to mark B as duplicate of C: %v", err)
		}

		// Attempt to mark A as duplicate of B (B is non-canonical, should be rejected)
		dupTargetB := todoB.ID
		_, err = svc.UpdateTodo(userID, todoA.ID, UpdateTodoInput{
			DuplicateOfID: &dupTargetB,
		})
		if err == nil {
			rt.Fatalf("expected error when marking A as duplicate of non-canonical B, but got nil")
		}

		// Verify the error message
		expectedMsg := "duplicate target must be canonical"
		if err.Error() != expectedMsg {
			rt.Fatalf("expected error %q, got %q", expectedMsg, err.Error())
		}
	})
}

// Feature: todo-filter-duplicate, Property 4: Marking as duplicate creates relation and sets status
// **Validates: Requirements 3.4**
//
// Property: For any todo and any valid canonical target (exists, belongs to user,
// is not itself a duplicate, is not the same todo), marking the todo as a duplicate
// of the target SHALL result in: (a) a duplicate_of relation existing from source
// to target, and (b) the source todo's status being set to 'duplicate'.
func TestProperty_MarkDuplicateCreatesRelationAndStatus(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, db := setupDuplicatePropertyService(t)
		userID := uint(1)

		categories := []string{"bug", "feature", "task"}
		priorities := []string{"p0", "p1", "p2", "p3"}

		// Generate random categories and priorities for source and target
		sourceCat := rapid.SampledFrom(categories).Draw(rt, "sourceCat")
		targetCat := rapid.SampledFrom(categories).Draw(rt, "targetCat")
		sourcePri := rapid.SampledFrom(priorities).Draw(rt, "sourcePri")
		targetPri := rapid.SampledFrom(priorities).Draw(rt, "targetPri")

		// Generate random titles
		sourceTitle := rapid.StringMatching(`[a-zA-Z0-9 ]{1,30}`).Draw(rt, "sourceTitle")
		targetTitle := rapid.StringMatching(`[a-zA-Z0-9 ]{1,30}`).Draw(rt, "targetTitle")

		// Create the target (canonical) todo first
		targetTodo, err := svc.CreateTodo(userID, CreateTodoInput{
			Title:    targetTitle,
			Category: targetCat,
			Priority: targetPri,
		})
		if err != nil {
			rt.Fatalf("failed to create target todo: %v", err)
		}

		// Create the source todo
		sourceTodo, err := svc.CreateTodo(userID, CreateTodoInput{
			Title:    sourceTitle,
			Category: sourceCat,
			Priority: sourcePri,
		})
		if err != nil {
			rt.Fatalf("failed to create source todo: %v", err)
		}

		// Mark source as duplicate of target via UpdateTodo with duplicate_of_id
		dupID := targetTodo.ID
		updatedTodo, err := svc.UpdateTodo(userID, sourceTodo.ID, UpdateTodoInput{
			DuplicateOfID: &dupID,
		})
		if err != nil {
			rt.Fatalf("failed to mark as duplicate: %v", err)
		}

		// Verify (a): a duplicate_of relation exists from source to target
		relationRepo := repository.NewRelationRepo(db)
		relations, err := relationRepo.FindBySourceAndType(nil, sourceTodo.ID, model.RelationDuplicateOf)
		if err != nil {
			rt.Fatalf("failed to find relations: %v", err)
		}
		if len(relations) == 0 {
			rt.Fatalf("no duplicate_of relation found for source todo %d", sourceTodo.ID)
		}

		foundRelation := false
		for _, rel := range relations {
			if rel.TargetID == targetTodo.ID {
				foundRelation = true
				break
			}
		}
		if !foundRelation {
			rt.Fatalf("duplicate_of relation from source %d to target %d not found", sourceTodo.ID, targetTodo.ID)
		}

		// Verify (b): the source todo's status is now 'duplicate'
		if updatedTodo.Status != model.StatusDuplicate {
			rt.Fatalf("source todo status expected %q, got %q", model.StatusDuplicate, updatedTodo.Status)
		}

		// Double-check by reading from DB directly
		todoRepo := repository.NewTodoRepo(db)
		dbTodo, err := todoRepo.FindByID(nil, sourceTodo.ID, userID)
		if err != nil {
			rt.Fatalf("failed to read source todo from db: %v", err)
		}
		if dbTodo.Status != model.StatusDuplicate {
			rt.Fatalf("source todo status in DB expected %q, got %q", model.StatusDuplicate, dbTodo.Status)
		}
	})
}
