package client

type ErrorResponse struct {
	Error string `json:"error"`
}

type ConflictResponse struct {
	Error                string        `json:"error"`
	PendingDependencies  []TodoSummary `json:"pending_dependencies,omitempty"`
	CompletedDependents  []TodoSummary `json:"completed_dependents,omitempty"`
}

type TodoSummary struct {
	ID       uint   `json:"id"`
	Code     string `json:"code"`
	Title    string `json:"title"`
	Category string `json:"category"`
	Status   string `json:"status"`
}

type Todo struct {
	ID          uint    `json:"id"`
	Code        string  `json:"code"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Priority    string  `json:"priority"`
	Status      string  `json:"status"`
	DueAt       *string `json:"due_at"`
	Tags        []string `json:"tags"`
	Pinned      bool    `json:"pinned"`
	Highlighted bool    `json:"highlighted"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type TodoDetail struct {
	Todo
	DependsOn   []TodoSummary `json:"depends_on"`
	DependedBy  []TodoSummary `json:"depended_by"`
	DuplicateOf *TodoSummary  `json:"duplicate_of"`
	Duplicates  []TodoSummary `json:"duplicates"`
}

type PaginatedTodosResponse struct {
	Items    []Todo `json:"items"`
	Total    int64  `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

type TodoGraphNode struct {
	ID                uint    `json:"id"`
	Code              string  `json:"code"`
	Title             string  `json:"title"`
	Category          string  `json:"category"`
	Priority          string  `json:"priority"`
	Status            string  `json:"status"`
	DueAt             *string `json:"due_at"`
	PrerequisiteCount int     `json:"prerequisite_count"`
	DependentCount    int     `json:"dependent_count"`
	ComponentID       string  `json:"component_id"`
	IsComponentRoot   bool    `json:"is_component_root"`
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

type TodoGraphResponse struct {
	Nodes      []TodoGraphNode      `json:"nodes"`
	Edges      []TodoGraphEdge      `json:"edges"`
	Components []TodoGraphComponent `json:"components"`
}

type TodoByDateRangeItem struct {
	ID       uint   `json:"id"`
	Code     string `json:"code"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Category string `json:"category"`
	Priority string `json:"priority"`
}

type Comment struct {
	ID        uint   `json:"id"`
	TodoID    uint   `json:"todo_id"`
	UserID    uint   `json:"user_id"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}
