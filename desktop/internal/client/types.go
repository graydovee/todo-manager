// Package client is a small HTTP client for the todo-manager backend. It is a
// trimmed copy of todo-cli's client, authenticating with a Bearer access key
// (tdk_...).
package client

// ErrorResponse is the backend's generic error body: {"error": "..."}.
type ErrorResponse struct {
	Error string `json:"error"`
}

// ConflictResponse is returned with HTTP 409 from complete/reopen when there are
// unresolved dependency relations and cascade was not requested.
type ConflictResponse struct {
	Error               string        `json:"error"`
	PendingDependencies []TodoSummary `json:"pending_dependencies,omitempty"`
	CompletedDependents []TodoSummary `json:"completed_dependents,omitempty"`
}

// TodoSummary is the lightweight relation target used in conflict bodies and
// detail relation lists.
type TodoSummary struct {
	ID       uint   `json:"id"`
	Code     string `json:"code"`
	Title    string `json:"title"`
	Category string `json:"category"`
	Status   string `json:"status"`
}

// Todo is a list item.
type Todo struct {
	ID          uint      `json:"id"`
	Code        string    `json:"code"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Priority    string    `json:"priority"`
	Status      string    `json:"status"`
	DueAt       *string   `json:"due_at"`
	Tags        []string  `json:"tags"`
	Pinned      bool      `json:"pinned"`
	Highlighted bool      `json:"highlighted"`
	CreatedAt   string    `json:"created_at"`
	UpdatedAt   string    `json:"updated_at"`
}

// TodoDetail is a single todo with its relation graph.
type TodoDetail struct {
	Todo
	DependsOn   []TodoSummary `json:"depends_on"`
	DependedBy  []TodoSummary `json:"depended_by"`
	DuplicateOf *TodoSummary  `json:"duplicate_of"`
	Duplicates  []TodoSummary `json:"duplicates"`
}

// PaginatedTodosResponse wraps a page of list results.
type PaginatedTodosResponse struct {
	Items    []Todo `json:"items"`
	Total    int64  `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

// Comment is a note attached to a todo.
type Comment struct {
	ID        uint   `json:"id"`
	TodoID    uint   `json:"todo_id"`
	UserID    uint   `json:"user_id"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}
