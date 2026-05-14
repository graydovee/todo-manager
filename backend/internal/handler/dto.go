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
}

type TodoGraphNodeResponse struct {
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

type TodoGraphEdgeResponse struct {
	SourceID uint `json:"source_id"`
	TargetID uint `json:"target_id"`
}

type TodoGraphComponentResponse struct {
	ID            string           `json:"id"`
	RootIDs       []uint           `json:"root_ids"`
	RootSummaries []TodoSummaryDTO `json:"root_summaries"`
	NodeIDs       []uint           `json:"node_ids"`
	AllCompleted  bool             `json:"all_completed"`
}

type TodoGraphResponse struct {
	Nodes      []TodoGraphNodeResponse      `json:"nodes"`
	Edges      []TodoGraphEdgeResponse      `json:"edges"`
	Components []TodoGraphComponentResponse `json:"components"`
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
		Status:      todo.Status,
		DueAt:       todo.DueAt,
		Tags:        tags,
		CreatedAt:   todo.CreatedAt,
		UpdatedAt:   todo.UpdatedAt,
	}
}
