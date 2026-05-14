package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/graydovee/todolist/internal/model"
	"github.com/graydovee/todolist/internal/repository"
	"gorm.io/gorm"
)

var (
	validCategories = map[string]bool{"bug": true, "feature": true, "task": true}
	validPriorities = map[string]bool{"p0": true, "p1": true, "p2": true, "p3": true}
)

type TodoService struct {
	db           *gorm.DB
	todoRepo     *repository.TodoRepo
	tagRepo      *repository.TagRepo
	relationRepo *repository.RelationRepo
	counterRepo  *repository.CodeCounterRepo
}

func NewTodoService(
	db *gorm.DB,
	todoRepo *repository.TodoRepo,
	tagRepo *repository.TagRepo,
	relationRepo *repository.RelationRepo,
	counterRepo *repository.CodeCounterRepo,
) *TodoService {
	return &TodoService{
		db:           db,
		todoRepo:     todoRepo,
		tagRepo:      tagRepo,
		relationRepo: relationRepo,
		counterRepo:  counterRepo,
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
	ID    uint   `json:"id"`
	Code  string `json:"code"`
	Title string `json:"title"`
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
		code, err := s.counterRepo.GetNextCode(tx, userID, category)
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
	todo.Status = model.StatusInProgress
	todo.UpdatedAt = time.Now()
	return s.todoRepo.Update(nil, todo)
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
	todo.Status = newStatus
	todo.UpdatedAt = time.Now()
	return s.todoRepo.Update(nil, todo)
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

		todo.Status = model.StatusCompleted
		todo.UpdatedAt = time.Now()
		if err := s.todoRepo.Update(tx, todo); err != nil {
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
	if todo.Status == model.StatusCompleted {
		return nil, nil
	}

	completedDependents, err := s.getCompletedDependents(todoID, userID)
	if err != nil {
		return nil, err
	}

	if len(completedDependents) > 0 && !cascadeDependents {
		conflict := &ConflictInfo{CompletedDependents: toSummaries(completedDependents)}
		return conflict, nil
	}

	return nil, s.db.Transaction(func(tx *gorm.DB) error {
		if cascadeDependents {
			for _, dep := range completedDependents {
				if err := s.cascadeReopen(tx, dep.ID, userID); err != nil {
					return err
				}
			}
		}

		todo.Status = model.StatusOpen
		todo.UpdatedAt = time.Now()
		if err := s.todoRepo.Update(tx, todo); err != nil {
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

	todo.Status = model.StatusCompleted
	todo.UpdatedAt = time.Now()
	if err := s.todoRepo.Update(tx, todo); err != nil {
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

	todo.Status = model.StatusOpen
	todo.UpdatedAt = time.Now()
	if err := s.todoRepo.Update(tx, todo); err != nil {
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
		if t != "" && !seen[t] {
			seen[t] = true
			result = append(result, t)
		}
	}
	return result
}

func toSummaries(todos []*model.Todo) []TodoSummary {
	summaries := make([]TodoSummary, len(todos))
	for i, t := range todos {
		summaries[i] = TodoSummary{ID: t.ID, Code: t.Code, Title: t.Title}
	}
	return summaries
}
