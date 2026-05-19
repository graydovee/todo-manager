package service

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/graydovee/todolist/internal/model"
	"github.com/graydovee/todolist/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	// Create tables with explicit SQL — must execute one at a time for SQLite
	sqlDB, _ := db.DB()
	tables := []string{
		`CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY AUTOINCREMENT, auth_provider TEXT NOT NULL, auth_subject TEXT NOT NULL, display_name TEXT NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, UNIQUE(auth_provider, auth_subject))`,
		`CREATE TABLE IF NOT EXISTS todos (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, code TEXT NOT NULL, title TEXT NOT NULL, description TEXT DEFAULT '', category TEXT NOT NULL CHECK(category IN ('bug','feature','task')), priority TEXT NOT NULL DEFAULT 'p2' CHECK(priority IN ('p0','p1','p2','p3')), status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open','in_progress','completed')), due_at DATETIME, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, UNIQUE(user_id, code))`,
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

func setupService(t *testing.T) (*TodoService, *gorm.DB) {
	t.Helper()
	db := setupTestDB(t)
	svc := NewTodoService(
		db,
		repository.NewTodoRepo(db),
		repository.NewTagRepo(db),
		repository.NewRelationRepo(db),
		repository.NewCodeCounterRepo(db),
	)
	return svc, db
}

func TestCreateTodo_Bug(t *testing.T) {
	svc, _ := setupService(t)

	todo, err := svc.CreateTodo(1, CreateTodoInput{
		Title:    "Fix crash",
		Category: "bug",
		Priority: "p0",
		Tags:     []string{"URGENT", " Backend "},
	})

	if err != nil {
		t.Fatalf("create todo: %v", err)
	}
	if todo.Code != "1" {
		t.Errorf("expected 1, got %s", todo.Code)
	}
	if todo.Category != "bug" {
		t.Errorf("expected bug, got %s", todo.Category)
	}
	if len(todo.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(todo.Tags))
	}
	tagMap := map[string]bool{}
	for _, tt := range todo.Tags {
		tagMap[tt.Tag] = true
	}
	if !tagMap["urgent"] || !tagMap["backend"] {
		t.Errorf("expected tags urgent and backend, got %v", todo.Tags)
	}
}

func TestCreateTodo_FeatureAndTask(t *testing.T) {
	svc, _ := setupService(t)

	f, err := svc.CreateTodo(1, CreateTodoInput{Title: "New feature", Category: "feature"})
	if err != nil {
		t.Fatalf("create feature: %v", err)
	}
	if f.Code != "1" {
		t.Errorf("expected 1, got %s", f.Code)
	}

	task, err := svc.CreateTodo(1, CreateTodoInput{Title: "Task item", Category: "task"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if task.Code != "2" {
		t.Errorf("expected 2, got %s", task.Code)
	}
}

func TestCreateTodo_CodeIncrements(t *testing.T) {
	svc, _ := setupService(t)

	for i := 0; i < 5; i++ {
		_, err := svc.CreateTodo(1, CreateTodoInput{Title: "Bug", Category: "bug"})
		if err != nil {
			t.Fatalf("create todo %d: %v", i, err)
		}
	}

	last, _ := svc.CreateTodo(1, CreateTodoInput{Title: "Last", Category: "bug"})
	if last.Code != "6" {
		t.Errorf("expected 6, got %s", last.Code)
	}
}

func TestCreateTodo_InvalidCategory(t *testing.T) {
	svc, _ := setupService(t)

	_, err := svc.CreateTodo(1, CreateTodoInput{Title: "Bad", Category: "invalid"})
	if err == nil {
		t.Error("expected error for invalid category")
	}
}

func TestCreateTodo_InvalidPriority(t *testing.T) {
	svc, _ := setupService(t)

	_, err := svc.CreateTodo(1, CreateTodoInput{Title: "Bad", Category: "bug", Priority: "urgent"})
	if err == nil {
		t.Error("expected error for invalid priority")
	}
}

func TestCreateTodo_DependsOn(t *testing.T) {
	svc, _ := setupService(t)

	a, _ := svc.CreateTodo(1, CreateTodoInput{Title: "A", Category: "bug"})
	b, err := svc.CreateTodo(1, CreateTodoInput{
		Title:        "B",
		Category:     "bug",
		DependsOnIDs: []uint{a.ID},
	})
	if err != nil {
		t.Fatalf("create B depending on A: %v", err)
	}
	if b.ID == 0 {
		t.Error("expected B to have an ID")
	}
}

func TestDeleteTodo(t *testing.T) {
	svc, _ := setupService(t)

	todo, _ := svc.CreateTodo(1, CreateTodoInput{Title: "Delete me", Category: "bug", Tags: []string{"tag1"}})

	err := svc.DeleteTodo(1, todo.ID)
	if err != nil {
		t.Fatalf("delete todo: %v", err)
	}

	_, err = svc.GetTodo(1, todo.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestCompleteTodo(t *testing.T) {
	svc, _ := setupService(t)

	todo, _ := svc.CreateTodo(1, CreateTodoInput{Title: "Complete me", Category: "bug"})

	conflict, err := svc.CompleteTodo(1, todo.ID, false)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if conflict != nil {
		t.Error("expected no conflict")
	}

	updated, _ := svc.GetTodo(1, todo.ID)
	if updated.Status != model.StatusCompleted {
		t.Error("expected completed")
	}
}

func TestCompleteTodo_WithDependencies(t *testing.T) {
	svc, _ := setupService(t)

	dep, _ := svc.CreateTodo(1, CreateTodoInput{Title: "Dependency", Category: "bug"})
	todo, _ := svc.CreateTodo(1, CreateTodoInput{
		Title:        "With dep",
		Category:     "bug",
		DependsOnIDs: []uint{dep.ID},
	})

	// Without cascade should return conflict
	conflict, err := svc.CompleteTodo(1, todo.ID, false)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if conflict == nil {
		t.Fatal("expected conflict for incomplete dependencies")
	}
	if len(conflict.PendingDependencies) != 1 {
		t.Fatalf("expected 1 pending dep, got %d", len(conflict.PendingDependencies))
	}
	if conflict.PendingDependencies[0].Code != "1" {
		t.Errorf("expected 1, got %s", conflict.PendingDependencies[0].Code)
	}

	// With cascade should succeed
	conflict, err = svc.CompleteTodo(1, todo.ID, true)
	if err != nil {
		t.Fatalf("cascade complete: %v", err)
	}
	if conflict != nil {
		t.Error("expected no conflict with cascade")
	}

	updatedTodo, _ := svc.GetTodo(1, todo.ID)
	updatedDep, _ := svc.GetTodo(1, dep.ID)
	if updatedTodo.Status != model.StatusCompleted || updatedDep.Status != model.StatusCompleted {
		t.Error("expected both completed")
	}
}

func TestReopenTodo(t *testing.T) {
	svc, _ := setupService(t)

	todo, _ := svc.CreateTodo(1, CreateTodoInput{Title: "Reopen me", Category: "bug"})
	svc.CompleteTodo(1, todo.ID, false)

	conflict, err := svc.ReopenTodo(1, todo.ID, false)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if conflict != nil {
		t.Error("expected no conflict")
	}

	updated, _ := svc.GetTodo(1, todo.ID)
	if updated.Status == model.StatusCompleted {
		t.Error("expected not completed")
	}
}

func TestTagNormalization(t *testing.T) {
	tags := normalizeTags([]string{"  HELLO  ", "World", "hello", "", "  WORLD  "})
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d: %v", len(tags), tags)
	}
	if tags[0] != "hello" || tags[1] != "world" {
		t.Errorf("expected [hello, world], got %v", tags)
	}
}

func TestGetTodoGraph_EmptyGraph(t *testing.T) {
	svc, _ := setupService(t)

	graph, err := svc.GetTodoGraph(1)
	if err != nil {
		t.Fatalf("get graph: %v", err)
	}

	if len(graph.Nodes) != 0 || len(graph.Edges) != 0 || len(graph.Components) != 0 {
		t.Fatalf("expected empty graph, got %+v", graph)
	}
}

func TestGetTodoGraph_SingleNodeComponent(t *testing.T) {
	svc, _ := setupService(t)

	todo, err := svc.CreateTodo(1, CreateTodoInput{Title: "Solo", Category: "task"})
	if err != nil {
		t.Fatalf("create todo: %v", err)
	}

	graph, err := svc.GetTodoGraph(1)
	if err != nil {
		t.Fatalf("get graph: %v", err)
	}

	if len(graph.Nodes) != 1 || len(graph.Components) != 1 || len(graph.Edges) != 0 {
		t.Fatalf("unexpected graph sizes: %+v", graph)
	}

	node := graph.Nodes[0]
	if node.ID != todo.ID || !node.IsComponentRoot || node.PrerequisiteCount != 0 || node.DependentCount != 0 {
		t.Fatalf("unexpected node: %+v", node)
	}

	component := graph.Components[0]
	if len(component.RootIDs) != 1 || component.RootIDs[0] != todo.ID || len(component.NodeIDs) != 1 || component.NodeIDs[0] != todo.ID {
		t.Fatalf("unexpected component: %+v", component)
	}
	if component.AllCompleted {
		t.Fatalf("expected component to be incomplete")
	}
}

func TestGetTodoGraph_ComponentRootsAndCompletion(t *testing.T) {
	svc, _ := setupService(t)

	parent, _ := svc.CreateTodo(1, CreateTodoInput{Title: "Parent", Category: "feature"})
	childA, _ := svc.CreateTodo(1, CreateTodoInput{Title: "Child A", Category: "feature"})
	childB, _ := svc.CreateTodo(1, CreateTodoInput{Title: "Child B", Category: "feature"})

	_, err := svc.UpdateTodo(1, parent.ID, UpdateTodoInput{DependsOnIDs: &[]uint{childA.ID, childB.ID}})
	if err != nil {
		t.Fatalf("update dependencies: %v", err)
	}

	_, err = svc.CompleteTodo(1, parent.ID, true)
	if err != nil {
		t.Fatalf("complete graph: %v", err)
	}

	graph, err := svc.GetTodoGraph(1)
	if err != nil {
		t.Fatalf("get graph: %v", err)
	}

	if len(graph.Components) != 1 {
		t.Fatalf("expected 1 component, got %d", len(graph.Components))
	}

	component := graph.Components[0]
	if !component.AllCompleted {
		t.Fatalf("expected completed component")
	}
	if len(component.RootIDs) != 1 || component.RootIDs[0] != parent.ID {
		t.Fatalf("unexpected roots: %+v", component.RootIDs)
	}

	nodeByID := map[uint]TodoGraphNode{}
	for _, node := range graph.Nodes {
		nodeByID[node.ID] = node
	}

	if !nodeByID[parent.ID].IsComponentRoot {
		t.Fatalf("expected parent to be root")
	}
	if nodeByID[parent.ID].PrerequisiteCount != 2 {
		t.Fatalf("expected parent prerequisite count 2, got %+v", nodeByID[parent.ID])
	}
	if nodeByID[childA.ID].DependentCount != 1 || nodeByID[childB.ID].DependentCount != 1 {
		t.Fatalf("expected children dependent counts to be 1")
	}
}

func TestGetTodoGraph_SharedDependencySingleNode(t *testing.T) {
	svc, _ := setupService(t)

	rootA, _ := svc.CreateTodo(1, CreateTodoInput{Title: "Root A", Category: "task"})
	rootB, _ := svc.CreateTodo(1, CreateTodoInput{Title: "Root B", Category: "task"})
	shared, _ := svc.CreateTodo(1, CreateTodoInput{Title: "Shared", Category: "task"})

	_, err := svc.UpdateTodo(1, rootA.ID, UpdateTodoInput{DependsOnIDs: &[]uint{shared.ID}})
	if err != nil {
		t.Fatalf("update rootA dependencies: %v", err)
	}
	_, err = svc.UpdateTodo(1, rootB.ID, UpdateTodoInput{DependsOnIDs: &[]uint{shared.ID}})
	if err != nil {
		t.Fatalf("update rootB dependencies: %v", err)
	}

	graph, err := svc.GetTodoGraph(1)
	if err != nil {
		t.Fatalf("get graph: %v", err)
	}

	if len(graph.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(graph.Nodes))
	}
	if len(graph.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(graph.Edges))
	}
	if len(graph.Components) != 1 {
		t.Fatalf("expected 1 component, got %d", len(graph.Components))
	}

	component := graph.Components[0]
	if len(component.RootIDs) != 2 || component.RootIDs[0] != rootA.ID || component.RootIDs[1] != rootB.ID {
		t.Fatalf("unexpected shared roots: %+v", component.RootIDs)
	}

	nodeIDs := map[uint]bool{}
	for _, node := range graph.Nodes {
		nodeIDs[node.ID] = true
	}
	if !nodeIDs[shared.ID] {
		t.Fatalf("expected shared node present once")
	}
}

func TestUserIsolation(t *testing.T) {
	svc, _ := setupService(t)

	todo, _ := svc.CreateTodo(1, CreateTodoInput{Title: "User 1 todo", Category: "bug"})

	_, err := svc.GetTodo(2, todo.ID)
	if err == nil {
		t.Error("expected error accessing other user's todo")
	}
}

func TestCompleteTodo_AutoCompletesDuplicates(t *testing.T) {
	svc, _ := setupService(t)

	canonical, _ := svc.CreateTodo(1, CreateTodoInput{Title: "Canonical", Category: "bug"})
	dup, _ := svc.CreateTodo(1, CreateTodoInput{
		Title:         "Duplicate",
		Category:      "bug",
		DuplicateOfID: &canonical.ID,
	})

	_, err := svc.CompleteTodo(1, canonical.ID, false)
	if err != nil {
		t.Fatalf("complete canonical: %v", err)
	}

	updatedDup, _ := svc.GetTodo(1, dup.ID)
	if updatedDup.Status != model.StatusCompleted {
		t.Error("expected duplicate to be auto-completed")
	}
}

func TestReopenTodo_AutoReopensDuplicates(t *testing.T) {
	svc, _ := setupService(t)

	canonical, _ := svc.CreateTodo(1, CreateTodoInput{Title: "Canonical", Category: "bug"})
	dup, _ := svc.CreateTodo(1, CreateTodoInput{
		Title:         "Duplicate",
		Category:      "bug",
		DuplicateOfID: &canonical.ID,
	})

	svc.CompleteTodo(1, canonical.ID, false)
	svc.ReopenTodo(1, canonical.ID, false)

	updatedDup, _ := svc.GetTodo(1, dup.ID)
	if updatedDup.Status == model.StatusCompleted {
		t.Error("expected duplicate to be auto-reopened")
	}
}

func TestCompleteTodo_AlreadyComplete(t *testing.T) {
	svc, _ := setupService(t)

	todo, _ := svc.CreateTodo(1, CreateTodoInput{Title: "Done", Category: "bug"})
	svc.CompleteTodo(1, todo.ID, false)

	// Completing again should be a no-op
	conflict, err := svc.CompleteTodo(1, todo.ID, false)
	if err != nil {
		t.Fatalf("re-complete: %v", err)
	}
	if conflict != nil {
		t.Error("expected no conflict for already completed")
	}
}

func TestReopenTodo_NotCompleted(t *testing.T) {
	svc, _ := setupService(t)

	todo, _ := svc.CreateTodo(1, CreateTodoInput{Title: "Open", Category: "bug"})

	// Reopening an open todo should be a no-op
	conflict, err := svc.ReopenTodo(1, todo.ID, false)
	if err != nil {
		t.Fatalf("reopen open: %v", err)
	}
	if conflict != nil {
		t.Error("expected no conflict for open todo")
	}
}

func TestReopenTodo_WithCompletedDependentsConflict(t *testing.T) {
	svc, _ := setupService(t)

	dependency, _ := svc.CreateTodo(1, CreateTodoInput{Title: "Dependency", Category: "bug"})
	dependent, _ := svc.CreateTodo(1, CreateTodoInput{
		Title:        "Dependent",
		Category:     "feature",
		DependsOnIDs: []uint{dependency.ID},
	})

	if _, err := svc.CompleteTodo(1, dependent.ID, true); err != nil {
		t.Fatalf("complete dependent graph: %v", err)
	}

	conflict, err := svc.ReopenTodo(1, dependency.ID, false)
	if err != nil {
		t.Fatalf("reopen dependency: %v", err)
	}
	if conflict == nil {
		t.Fatal("expected conflict when reopening with completed dependents")
	}
	if len(conflict.CompletedDependents) != 1 || conflict.CompletedDependents[0].ID != dependent.ID {
		t.Fatalf("unexpected completed dependents: %+v", conflict.CompletedDependents)
	}

	updatedDependency, _ := svc.GetTodo(1, dependency.ID)
	if updatedDependency.Status != model.StatusCompleted {
		t.Fatalf("expected dependency to remain completed before cascade reopen, got %s", updatedDependency.Status)
	}
}

func TestCreateTodo_DefaultPriority(t *testing.T) {
	svc, _ := setupService(t)

	todo, _ := svc.CreateTodo(1, CreateTodoInput{Title: "Default", Category: "bug"})
	if todo.Priority != "p2" {
		t.Errorf("expected p2, got %s", todo.Priority)
	}
}
