package handler

import (
	"time"

	"github.com/graydovee/todolist/internal/model"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type ConflictResponse struct {
	Error     string          `json:"error"`
	Pending   []TodoSummaryDTO `json:"pending_dependencies,omitempty"`
	Completed []TodoSummaryDTO `json:"completed_dependents,omitempty"`
}

type TodoSummaryDTO struct {
	ID    uint   `json:"id"`
	Code  string `json:"code"`
	Title string `json:"title"`
}

type TodoResponse struct {
	ID          uint       `json:"id"`
	Code        string     `json:"code"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Category    string     `json:"category"`
	Priority    string     `json:"priority"`
	Status      string     `json:"status"`
	DueAt       *time.Time `json:"due_at"`
	ParentID    *uint      `json:"parent_id"`
	Tags        []string   `json:"tags"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type TodoDetailResponse struct {
	TodoResponse
	DependsOn   []TodoSummaryDTO `json:"depends_on"`
	DependedBy  []TodoSummaryDTO `json:"depended_by"`
	DuplicateOf *TodoSummaryDTO  `json:"duplicate_of"`
	Duplicates  []TodoSummaryDTO `json:"duplicates"`
	Parent      *TodoSummaryDTO  `json:"parent"`
	Children    []TodoSummaryDTO `json:"children"`
}

type PaginatedResponse struct {
	Items    []TodoResponse `json:"items"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

type SetStatusRequest struct {
	Status string `json:"status"`
}

type CreateCommentRequest struct {
	Content string `json:"content"`
}

type CompleteRequest struct {
	CascadeDependencies bool `json:"cascade_dependencies"`
}

type ReopenRequest struct {
	CascadeDependents bool `json:"cascade_dependents"`
}

func todoToResponse(todo *model.Todo) TodoResponse {
	var tags []string
	for _, t := range todo.Tags {
		tags = append(tags, t.Tag)
	}
	if tags == nil {
		tags = []string{}
	}
	return TodoResponse{
		ID:          todo.ID,
		Code:        todo.Code,
		Title:       todo.Title,
		Description: todo.Description,
		Category:    todo.Category,
		Priority:    todo.Priority,
		Status:    todo.Status,
		DueAt:       todo.DueAt,
		ParentID:    todo.ParentID,
		Tags:        tags,
		CreatedAt:   todo.CreatedAt,
		UpdatedAt:   todo.UpdatedAt,
	}
}
