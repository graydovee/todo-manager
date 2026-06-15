package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/graydovee/todo-manager/internal/model"
	"github.com/graydovee/todo-manager/internal/repository"
	"gorm.io/gorm"
)

var (
	validCategories = map[string]bool{"bug": true, "feature": true, "task": true}
	validPriorities = map[string]bool{"p0": true, "p1": true, "p2": true, "p3": true}
)

type TodoService struct {
	db                *gorm.DB
	todoRepo          *repository.TodoRepo
	tagRepo           *repository.TagRepo
	relationRepo      *repository.RelationRepo
	counterRepo       *repository.CodeCounterRepo
	statusHistoryRepo *repository.StatusHistoryRepo
}

func NewTodoService(
	db *gorm.DB,
	todoRepo *repository.TodoRepo,
	tagRepo *repository.TagRepo,
	relationRepo *repository.RelationRepo,
	counterRepo *repository.CodeCounterRepo,
	statusHistoryRepo *repository.StatusHistoryRepo,
) *TodoService {
	return &TodoService{
		db:                db,
		todoRepo:          todoRepo,
		tagRepo:           tagRepo,
		relationRepo:      relationRepo,
		counterRepo:       counterRepo,
		statusHistoryRepo: statusHistoryRepo,
	}
}

type CreateTodoInput struct {
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	Category      string     `json:"category"`
	Priority      string     `json:"priority"`
	Tags          []string   `json:"tags"`
	DueAt         *time.Time `json:"due_at"`
	DependsOnIDs  []uint     `json:"depends_on_ids"`
	DuplicateOfID *uint      `json:"duplicate_of_id"`
}

type UpdateTodoInput struct {
	Title         *string     `json:"title"`
	Description   *string     `json:"description"`
	Category      *string     `json:"category"`
	Priority      *string     `json:"priority"`
	Tags          *[]string   `json:"tags"`
	DueAt         **time.Time `json:"due_at"`
	DependsOnIDs  *[]uint     `json:"depends_on_ids"`
	DuplicateOfID *uint       `json:"duplicate_of_id"`
}

type ConflictInfo struct {
	PendingDependencies []TodoSummary `json:"pending_dependencies,omitempty"`
	CompletedDependents []TodoSummary `json:"completed_dependents,omitempty"`
}

type TodoSummary struct {
	ID       uint   `json:"id"`
	Code     string `json:"code"`
	Title    string `json:"title"`
	Category string `json:"category"`
	Status   string `json:"status"`
}

type TodoGraphNode struct {
	ID                uint       `json:"id"`
	Code              string     `json:"code"`
	Title             string     `json:"title"`
	Category          string     `json:"category"`
	Priority          string     `json:"priority"`
	Status            string     `json:"status"`
	DueAt             *time.Time `json:"due_at"`
	PrerequisiteCount int        `json:"prerequisite_count"`
	DependentCount    int        `json:"dependent_count"`
	ComponentID       string     `json:"component_id"`
	IsComponentRoot   bool       `json:"is_component_root"`
}

type TodoGraphEdge struct {
	SourceID uint `json:"source_id"`
	TargetID uint `json:"target_id"`
}

type TodoGraphComponent struct {
	ID            string        `json:"id"`
	RootIDs       []uint        `json:"root_ids"`
	RootSummaries []TodoSummary `json:"root_summaries"`
	NodeIDs       []uint        `json:"node_ids"`
	AllCompleted  bool          `json:"all_completed"`
}

type TodoGraphSnapshot struct {
	Nodes      []TodoGraphNode      `json:"nodes"`
	Edges      []TodoGraphEdge      `json:"edges"`
	Components []TodoGraphComponent `json:"components"`
}

func (s *TodoService) CreateTodo(userID uint, input CreateTodoInput) (*model.Todo, error) {
	category := strings.ToLower(input.Category)
	if !validCategories[category] {
		return nil, fmt.Errorf("invalid category: %s (must be bug, feature, or task)", input.Category)
	}

	priority := "p2"
	if input.Priority != "" {
		priority = strings.ToLower(input.Priority)
	}
	if !validPriorities[priority] {
		return nil, fmt.Errorf("invalid priority: %s", input.Priority)
	}

	todo := &model.Todo{}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		code, err := s.counterRepo.GetNextCode(tx, userID)
		if err != nil {
			return err
		}

		now := time.Now()
		todo.UserID = userID
		todo.Code = code
		todo.Title = input.Title
		todo.Description = input.Description
		todo.Category = category
		todo.Priority = priority
		todo.DueAt = input.DueAt
		todo.CreatedAt = now
		todo.UpdatedAt = now

		if err := s.todoRepo.Create(tx, todo); err != nil {
			return err
		}

		// Record initial status history entry
		historyRecord := &model.TodoStatusHistory{
			TodoID:    todo.ID,
			OldStatus: "",
			NewStatus: model.StatusOpen,
			ChangedAt: time.Now(),
		}
		if err := s.statusHistoryRepo.Create(tx, historyRecord); err != nil {
			return err
		}

		tags := normalizeTags(input.Tags)
		if len(tags) > 0 {
			if err := s.tagRepo.ReplaceTags(tx, todo.ID, tags); err != nil {
				return err
			}
		}

		return s.createRelations(tx, todo.ID, userID, input.DependsOnIDs, input.DuplicateOfID)
	})
	if err != nil {
		return nil, err
	}

	return s.todoRepo.FindByIDWithDetails(nil, todo.ID, userID)
}

func (s *TodoService) UpdateTodo(userID, todoID uint, input UpdateTodoInput) (*model.Todo, error) {
	todo, err := s.todoRepo.FindByID(nil, todoID, userID)
	if err != nil {
		return nil, fmt.Errorf("todo not found")
	}

	if input.Title != nil {
		todo.Title = *input.Title
	}
	if input.Description != nil {
		todo.Description = *input.Description
	}
	if input.Category != nil {
		cat := strings.ToLower(*input.Category)
		if !validCategories[cat] {
			return nil, fmt.Errorf("invalid category: %s (must be bug, feature, or task)", *input.Category)
		}
		todo.Category = cat
	}
	if input.Priority != nil {
		p := strings.ToLower(*input.Priority)
		if !validPriorities[p] {
			return nil, fmt.Errorf("invalid priority: %s", *input.Priority)
		}
		todo.Priority = p
	}
	if input.DueAt != nil {
		todo.DueAt = *input.DueAt
	}

	todo.UpdatedAt = time.Now()

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.todoRepo.Update(tx, todo); err != nil {
			return err
		}

		if input.Tags != nil {
			tags := normalizeTags(*input.Tags)
			if err := s.tagRepo.ReplaceTags(tx, todo.ID, tags); err != nil {
				return err
			}
		}

		if input.DependsOnIDs != nil || input.DuplicateOfID != nil {
			var depsOnIDs []uint
			var dupOfID *uint

			if input.DependsOnIDs != nil {
				depsOnIDs = *input.DependsOnIDs
			} else {
				existing, _ := s.relationRepo.FindBySource(nil, todo.ID)
				for _, r := range existing {
					if r.RelationType == model.RelationDependsOn {
						depsOnIDs = append(depsOnIDs, r.TargetID)
					}
				}
			}

			if input.DuplicateOfID != nil {
				dupOfID = input.DuplicateOfID
			} else {
				existing, _ := s.relationRepo.FindBySourceAndType(nil, todo.ID, model.RelationDuplicateOf)
				if len(existing) > 0 {
					id := existing[0].TargetID
					dupOfID = &id
				}
			}

			if err := s.relationRepo.ReplaceRelations(tx, todo.ID, nil); err != nil {
				return err
			}
			if err := s.createRelations(tx, todo.ID, userID, depsOnIDs, dupOfID); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.todoRepo.FindByIDWithDetails(nil, todo.ID, userID)
}

func (s *TodoService) DeleteTodo(userID, todoID uint) error {
	_, err := s.todoRepo.FindByID(nil, todoID, userID)
	if err != nil {
		return fmt.Errorf("todo not found")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.tagRepo.ReplaceTags(tx, todoID, nil); err != nil {
			return err
		}
		if err := s.relationRepo.DeleteBySourceOrTarget(tx, todoID); err != nil {
			return err
		}
		// Delete status history records before deleting the todo
		if err := s.statusHistoryRepo.DeleteByTodoID(tx, todoID); err != nil {
			return err
		}
		return s.todoRepo.Delete(tx, todoID, userID)
	})
}

func (s *TodoService) StartTodo(userID, todoID uint) error {
	todo, err := s.todoRepo.FindByID(nil, todoID, userID)
	if err != nil {
		return fmt.Errorf("todo not found")
	}
	if todo.Status != model.StatusOpen {
		return fmt.Errorf("only open todos can be started")
	}
	oldStatus := todo.Status
	todo.Status = model.StatusInProgress
	todo.UpdatedAt = time.Now()

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.todoRepo.Update(tx, todo); err != nil {
			return err
		}
		// Record open→in_progress transition
		return s.statusHistoryRepo.Create(tx, &model.TodoStatusHistory{
			TodoID:    todo.ID,
			OldStatus: oldStatus,
			NewStatus: model.StatusInProgress,
			ChangedAt: time.Now(),
		})
	})
}

func (s *TodoService) SetStatus(userID, todoID uint, newStatus string) error {
	if !model.ValidStatus(newStatus) {
		return fmt.Errorf("invalid status: %s", newStatus)
	}
	todo, err := s.todoRepo.FindByID(nil, todoID, userID)
	if err != nil {
		return fmt.Errorf("todo not found")
	}
	if todo.Status == newStatus {
		return nil
	}
	oldStatus := todo.Status
	todo.Status = newStatus
	todo.UpdatedAt = time.Now()

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.todoRepo.Update(tx, todo); err != nil {
			return err
		}
		// Record status transition history
		return s.statusHistoryRepo.Create(tx, &model.TodoStatusHistory{
			TodoID:    todo.ID,
			OldStatus: oldStatus,
			NewStatus: newStatus,
			ChangedAt: time.Now(),
		})
	})
}

func (s *TodoService) CompleteTodo(userID, todoID uint, cascadeDependencies bool) (*ConflictInfo, error) {
	todo, err := s.todoRepo.FindByID(nil, todoID, userID)
	if err != nil {
		return nil, fmt.Errorf("todo not found")
	}
	if todo.Status == model.StatusCompleted {
		return nil, nil
	}

	incompleteDeps, err := s.getIncompleteDependencies(todoID, userID)
	if err != nil {
		return nil, err
	}

	if len(incompleteDeps) > 0 && !cascadeDependencies {
		conflict := &ConflictInfo{PendingDependencies: toSummaries(incompleteDeps)}
		return conflict, nil
	}

	return nil, s.db.Transaction(func(tx *gorm.DB) error {
		if cascadeDependencies {
			for _, dep := range incompleteDeps {
				if err := s.cascadeComplete(tx, dep.ID, userID); err != nil {
					return err
				}
			}
		}

		oldStatus := todo.Status
		todo.Status = model.StatusCompleted
		todo.UpdatedAt = time.Now()
		if err := s.todoRepo.Update(tx, todo); err != nil {
			return err
		}

		// Record →completed transition
		if err := s.statusHistoryRepo.Create(tx, &model.TodoStatusHistory{
			TodoID:    todo.ID,
			OldStatus: oldStatus,
			NewStatus: model.StatusCompleted,
			ChangedAt: time.Now(),
		}); err != nil {
			return err
		}

		return s.autoCompleteDuplicates(tx, todoID, userID)
	})
}

func (s *TodoService) ReopenTodo(userID, todoID uint, cascadeDependents bool) (*ConflictInfo, error) {
	todo, err := s.todoRepo.FindByID(nil, todoID, userID)
	if err != nil {
		return nil, fmt.Errorf("todo not found")
	}
	if todo.Status != model.StatusCompleted {
		return nil, nil
	}

	allCompletedDependents, err := s.getAllCompletedDependentsRecursive(todoID, userID, make(map[uint]bool))
	if err != nil {
		return nil, err
	}

	if len(allCompletedDependents) > 0 && !cascadeDependents {
		conflict := &ConflictInfo{CompletedDependents: toSummaries(allCompletedDependents)}
		return conflict, nil
	}

	return nil, s.db.Transaction(func(tx *gorm.DB) error {
		if cascadeDependents {
			completedDependents, err := s.getCompletedDependents(todoID, userID)
			if err != nil {
				return err
			}
			for _, dep := range completedDependents {
				if err := s.cascadeReopen(tx, dep.ID, userID); err != nil {
					return err
				}
			}
		}

		oldStatus := todo.Status
		todo.Status = model.StatusOpen
		todo.UpdatedAt = time.Now()
		if err := s.todoRepo.Update(tx, todo); err != nil {
			return err
		}

		// Record completed→open transition
		if err := s.statusHistoryRepo.Create(tx, &model.TodoStatusHistory{
			TodoID:    todo.ID,
			OldStatus: oldStatus,
			NewStatus: model.StatusOpen,
			ChangedAt: time.Now(),
		}); err != nil {
			return err
		}

		return s.autoReopenDuplicates(tx, todoID, userID)
	})
}

func (s *TodoService) GetTodo(userID, todoID uint) (*model.Todo, error) {
	return s.todoRepo.FindByIDWithDetails(nil, todoID, userID)
}

func (s *TodoService) ListTodos(userID uint, filters repository.TodoFilters) ([]*model.Todo, int64, error) {
	return s.todoRepo.List(nil, userID, filters)
}

func (s *TodoService) GetTodoGraph(userID uint) (*TodoGraphSnapshot, error) {
	todos, err := s.todoRepo.ListAllWithTags(nil, userID)
	if err != nil {
		return nil, err
	}

	relations, err := s.relationRepo.FindByUserAndType(nil, userID, model.RelationDependsOn)
	if err != nil {
		return nil, err
	}

	todoByID := make(map[uint]*model.Todo, len(todos))
	neighbors := make(map[uint]map[uint]struct{}, len(todos))
	prerequisiteCount := make(map[uint]int, len(todos))
	dependentCount := make(map[uint]int, len(todos))
	seenNodeIDs := make([]uint, 0, len(todos))

	for _, todo := range todos {
		todoByID[todo.ID] = todo
		neighbors[todo.ID] = make(map[uint]struct{})
		prerequisiteCount[todo.ID] = 0
		dependentCount[todo.ID] = 0
		seenNodeIDs = append(seenNodeIDs, todo.ID)
	}

	edges := make([]TodoGraphEdge, 0, len(relations))
	for _, rel := range relations {
		if todoByID[rel.SourceID] == nil || todoByID[rel.TargetID] == nil {
			continue
		}

		edges = append(edges, TodoGraphEdge{
			SourceID: rel.SourceID,
			TargetID: rel.TargetID,
		})

		prerequisiteCount[rel.SourceID]++
		dependentCount[rel.TargetID]++
		neighbors[rel.SourceID][rel.TargetID] = struct{}{}
		neighbors[rel.TargetID][rel.SourceID] = struct{}{}
	}

	componentIDByTodo := make(map[uint]string, len(todos))
	componentNodeIDs := make(map[string][]uint)
	componentAllCompleted := make(map[string]bool)
	componentRoots := make(map[string][]uint)

	visited := make(map[uint]bool, len(todos))
	componentIndex := 0

	for _, startID := range seenNodeIDs {
		if visited[startID] {
			continue
		}

		componentIndex++
		componentID := fmt.Sprintf("component-%d", componentIndex)
		queue := []uint{startID}
		visited[startID] = true
		componentAllCompleted[componentID] = true

		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]

			componentIDByTodo[current] = componentID
			componentNodeIDs[componentID] = append(componentNodeIDs[componentID], current)

			if todoByID[current].Status != model.StatusCompleted {
				componentAllCompleted[componentID] = false
			}

			for next := range neighbors[current] {
				if visited[next] {
					continue
				}
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}

	components := make([]TodoGraphComponent, 0, len(componentNodeIDs))
	nodes := make([]TodoGraphNode, 0, len(todos))

	for _, todoID := range seenNodeIDs {
		componentID := componentIDByTodo[todoID]
		if dependentCount[todoID] == 0 {
			componentRoots[componentID] = append(componentRoots[componentID], todoID)
		}
	}

	componentOrder := make([]string, 0, len(componentNodeIDs))
	for i := 1; i <= componentIndex; i++ {
		componentOrder = append(componentOrder, fmt.Sprintf("component-%d", i))
	}

	for _, componentID := range componentOrder {
		nodeIDs := componentNodeIDs[componentID]
		rootIDs := componentRoots[componentID]
		sort.Slice(nodeIDs, func(i, j int) bool { return nodeIDs[i] < nodeIDs[j] })
		sort.Slice(rootIDs, func(i, j int) bool { return rootIDs[i] < rootIDs[j] })

		rootSummaries := make([]TodoSummary, 0, len(rootIDs))
		for _, rootID := range rootIDs {
			root := todoByID[rootID]
			rootSummaries = append(rootSummaries, TodoSummary{
				ID:       root.ID,
				Code:     root.Code,
				Title:    root.Title,
				Category: root.Category,
				Status:   root.Status,
			})
		}

		components = append(components, TodoGraphComponent{
			ID:            componentID,
			RootIDs:       rootIDs,
			RootSummaries: rootSummaries,
			NodeIDs:       nodeIDs,
			AllCompleted:  componentAllCompleted[componentID],
		})
	}

	for _, todoID := range seenNodeIDs {
		todo := todoByID[todoID]
		componentID := componentIDByTodo[todoID]
		nodes = append(nodes, TodoGraphNode{
			ID:                todo.ID,
			Code:              todo.Code,
			Title:             todo.Title,
			Category:          todo.Category,
			Priority:          todo.Priority,
			Status:            todo.Status,
			DueAt:             todo.DueAt,
			PrerequisiteCount: prerequisiteCount[todo.ID],
			DependentCount:    dependentCount[todo.ID],
			ComponentID:       componentID,
			IsComponentRoot:   dependentCount[todo.ID] == 0,
		})
	}

	return &TodoGraphSnapshot{
		Nodes:      nodes,
		Edges:      edges,
		Components: components,
	}, nil
}

func (s *TodoService) createRelations(tx *gorm.DB, todoID, userID uint, dependsOnIDs []uint, duplicateOfID *uint) error {
	for _, depID := range dependsOnIDs {
		_, err := s.todoRepo.FindByID(tx, depID, userID)
		if err != nil {
			return fmt.Errorf("dependency %d not found", depID)
		}
		if depID == todoID {
			return fmt.Errorf("self-dependency not allowed")
		}
		hasCycle, err := s.relationRepo.HasCycle(tx, todoID, depID)
		if err != nil {
			return err
		}
		if hasCycle {
			return fmt.Errorf("dependency would create cycle")
		}
		rel := &model.TodoRelation{
			SourceID:     todoID,
			TargetID:     depID,
			RelationType: model.RelationDependsOn,
		}
		if err := s.relationRepo.Create(tx, rel); err != nil {
			return err
		}
	}

	if duplicateOfID != nil {
		target, err := s.todoRepo.FindByID(tx, *duplicateOfID, userID)
		if err != nil {
			return fmt.Errorf("duplicate target not found")
		}
		existingDups, _ := s.relationRepo.FindBySourceAndType(tx, target.ID, model.RelationDuplicateOf)
		if len(existingDups) > 0 {
			return fmt.Errorf("duplicate target must be canonical")
		}
		rel := &model.TodoRelation{
			SourceID:     todoID,
			TargetID:     *duplicateOfID,
			RelationType: model.RelationDuplicateOf,
		}
		if err := s.relationRepo.Create(tx, rel); err != nil {
			return err
		}

		// Set the current todo's status to duplicate
		todo, err := s.todoRepo.FindByID(tx, todoID, userID)
		if err != nil {
			return fmt.Errorf("failed to fetch todo for duplicate status update")
		}
		oldStatus := todo.Status
		if oldStatus != model.StatusDuplicate {
			todo.Status = model.StatusDuplicate
			todo.UpdatedAt = time.Now()
			if err := s.todoRepo.Update(tx, todo); err != nil {
				return err
			}
			// Record status history entry for the transition to duplicate
			if err := s.statusHistoryRepo.Create(tx, &model.TodoStatusHistory{
				TodoID:    todo.ID,
				OldStatus: oldStatus,
				NewStatus: model.StatusDuplicate,
				ChangedAt: time.Now(),
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *TodoService) getIncompleteDependencies(todoID, userID uint) ([]*model.Todo, error) {
	relations, err := s.relationRepo.FindBySourceAndType(nil, todoID, model.RelationDependsOn)
	if err != nil {
		return nil, err
	}
	var incomplete []*model.Todo
	for _, rel := range relations {
		todo, err := s.todoRepo.FindByID(nil, rel.TargetID, userID)
		if err != nil {
			continue
		}
		if todo.Status != model.StatusCompleted {
			incomplete = append(incomplete, todo)
		}
	}
	return incomplete, nil
}

func (s *TodoService) getCompletedDependents(todoID, userID uint) ([]*model.Todo, error) {
	relations, err := s.relationRepo.FindByTarget(nil, todoID)
	if err != nil {
		return nil, err
	}
	var completed []*model.Todo
	for _, rel := range relations {
		if rel.RelationType != model.RelationDependsOn {
			continue
		}
		todo, err := s.todoRepo.FindByID(nil, rel.SourceID, userID)
		if err != nil {
			continue
		}
		if todo.Status == model.StatusCompleted {
			completed = append(completed, todo)
		}
	}
	return completed, nil
}

func (s *TodoService) getAllCompletedDependentsRecursive(todoID, userID uint, visited map[uint]bool) ([]*model.Todo, error) {
	if visited[todoID] {
		return nil, nil
	}
	visited[todoID] = true

	directDeps, err := s.getCompletedDependents(todoID, userID)
	if err != nil {
		return nil, err
	}

	var all []*model.Todo
	for _, dep := range directDeps {
		if visited[dep.ID] {
			continue
		}
		all = append(all, dep)
		nested, err := s.getAllCompletedDependentsRecursive(dep.ID, userID, visited)
		if err != nil {
			return nil, err
		}
		all = append(all, nested...)
	}
	return all, nil
}

func (s *TodoService) cascadeComplete(tx *gorm.DB, todoID, userID uint) error {
	todo, err := s.todoRepo.FindByID(tx, todoID, userID)
	if err != nil || todo.Status == model.StatusCompleted {
		return err
	}

	incompleteDeps, err := s.getIncompleteDependencies(todoID, userID)
	if err != nil {
		return err
	}
	for _, dep := range incompleteDeps {
		if err := s.cascadeComplete(tx, dep.ID, userID); err != nil {
			return err
		}
	}

	oldStatus := todo.Status
	todo.Status = model.StatusCompleted
	todo.UpdatedAt = time.Now()
	if err := s.todoRepo.Update(tx, todo); err != nil {
		return err
	}

	// Record status transition for cascaded completion
	if err := s.statusHistoryRepo.Create(tx, &model.TodoStatusHistory{
		TodoID:    todo.ID,
		OldStatus: oldStatus,
		NewStatus: model.StatusCompleted,
		ChangedAt: time.Now(),
	}); err != nil {
		return err
	}

	return s.autoCompleteDuplicates(tx, todoID, userID)
}

func (s *TodoService) cascadeReopen(tx *gorm.DB, todoID, userID uint) error {
	todo, err := s.todoRepo.FindByID(tx, todoID, userID)
	if err != nil || todo.Status != model.StatusCompleted {
		return err
	}

	completedDependents, err := s.getCompletedDependents(todoID, userID)
	if err != nil {
		return err
	}
	for _, dep := range completedDependents {
		if err := s.cascadeReopen(tx, dep.ID, userID); err != nil {
			return err
		}
	}

	oldStatus := todo.Status
	todo.Status = model.StatusOpen
	todo.UpdatedAt = time.Now()
	if err := s.todoRepo.Update(tx, todo); err != nil {
		return err
	}

	// Record status transition for cascaded reopen
	if err := s.statusHistoryRepo.Create(tx, &model.TodoStatusHistory{
		TodoID:    todo.ID,
		OldStatus: oldStatus,
		NewStatus: model.StatusOpen,
		ChangedAt: time.Now(),
	}); err != nil {
		return err
	}

	return s.autoReopenDuplicates(tx, todoID, userID)
}

func (s *TodoService) autoCompleteDuplicates(tx *gorm.DB, todoID, userID uint) error {
	relations, err := s.relationRepo.FindByTarget(nil, todoID)
	if err != nil {
		return err
	}
	for _, rel := range relations {
		if rel.RelationType != model.RelationDuplicateOf {
			continue
		}
		dup, err := s.todoRepo.FindByID(tx, rel.SourceID, userID)
		if err != nil || dup.Status == model.StatusCompleted {
			continue
		}
		dup.Status = model.StatusCompleted
		dup.UpdatedAt = time.Now()
		if err := s.todoRepo.Update(tx, dup); err != nil {
			return err
		}
	}
	return nil
}

func (s *TodoService) autoReopenDuplicates(tx *gorm.DB, todoID, userID uint) error {
	relations, err := s.relationRepo.FindByTarget(nil, todoID)
	if err != nil {
		return err
	}
	for _, rel := range relations {
		if rel.RelationType != model.RelationDuplicateOf {
			continue
		}
		dup, err := s.todoRepo.FindByID(tx, rel.SourceID, userID)
		if err != nil || dup.Status != model.StatusCompleted {
			continue
		}
		dup.Status = model.StatusOpen
		dup.UpdatedAt = time.Now()
		if err := s.todoRepo.Update(tx, dup); err != nil {
			return err
		}
	}
	return nil
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, tag := range tags {
		t := strings.TrimSpace(strings.ToLower(tag))
		if t == "" {
			continue
		}
		// Truncate to 100 characters
		if len([]rune(t)) > 100 {
			t = string([]rune(t)[:100])
		}
		if !seen[t] {
			seen[t] = true
			result = append(result, t)
		}
	}
	return result
}

func toSummaries(todos []*model.Todo) []TodoSummary {
	summaries := make([]TodoSummary, len(todos))
	for i, t := range todos {
		summaries[i] = TodoSummary{ID: t.ID, Code: t.Code, Title: t.Title, Category: t.Category, Status: t.Status}
	}
	return summaries
}
