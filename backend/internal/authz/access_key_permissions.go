package authz

type APIPermission struct {
	ID          string `json:"id"`
	GroupID     string `json:"group_id"`
	Method      string `json:"method"`
	PathPattern string `json:"path_pattern"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

type PermissionGroup struct {
	ID             string   `json:"id"`
	Label          string   `json:"label"`
	Description    string   `json:"description"`
	PermissionIDs  []string `json:"permission_ids"`
}

type PermissionCatalog struct {
	APIs    []APIPermission        `json:"apis"`
	Groups  []PermissionGroup      `json:"groups"`
	Presets map[string][]string    `json:"presets"`
}

var AllAPIPermissions = []APIPermission{
	{ID: "todos:list", GroupID: "todos_read", Method: "GET", PathPattern: "/api/v1/todos", Label: "List todos", Description: "Read the todo list."},
	{ID: "todos:graph", GroupID: "todos_read", Method: "GET", PathPattern: "/api/v1/todos/graph", Label: "Read todo graph", Description: "Read the todo dependency graph."},
	{ID: "todos:tags", GroupID: "todos_read", Method: "GET", PathPattern: "/api/v1/todos/tags", Label: "List tags", Description: "Read distinct todo tags."},
	{ID: "todos:by_date_range", GroupID: "todos_read", Method: "GET", PathPattern: "/api/v1/todos/by-date-range", Label: "List todos by date range", Description: "Read todos filtered by updated date range."},
	{ID: "todos:get", GroupID: "todos_read", Method: "GET", PathPattern: "/api/v1/todos/:id", Label: "Get todo", Description: "Read a single todo."},
	{ID: "todos:comments:list", GroupID: "todos_read", Method: "GET", PathPattern: "/api/v1/todos/:id/comments", Label: "List comments", Description: "Read comments for a todo."},
	{ID: "todos:create", GroupID: "todos_write", Method: "POST", PathPattern: "/api/v1/todos", Label: "Create todo", Description: "Create a todo."},
	{ID: "todos:update", GroupID: "todos_write", Method: "PATCH", PathPattern: "/api/v1/todos/:id", Label: "Update todo", Description: "Update a todo."},
	{ID: "todos:delete", GroupID: "todos_write", Method: "DELETE", PathPattern: "/api/v1/todos/:id", Label: "Delete todo", Description: "Delete a todo."},
	{ID: "todos:start", GroupID: "todos_write", Method: "POST", PathPattern: "/api/v1/todos/:id/start", Label: "Start todo", Description: "Start a todo."},
	{ID: "todos:set_status", GroupID: "todos_write", Method: "PATCH", PathPattern: "/api/v1/todos/:id/status", Label: "Set todo status", Description: "Update todo status."},
	{ID: "todos:pin", GroupID: "todos_write", Method: "PATCH", PathPattern: "/api/v1/todos/:id/pin", Label: "Pin todo", Description: "Pin or unpin a todo."},
	{ID: "todos:highlight", GroupID: "todos_write", Method: "PATCH", PathPattern: "/api/v1/todos/:id/highlight", Label: "Highlight todo", Description: "Highlight or unhighlight a todo."},
	{ID: "todos:complete", GroupID: "todos_write", Method: "POST", PathPattern: "/api/v1/todos/:id/complete", Label: "Complete todo", Description: "Complete a todo."},
	{ID: "todos:reopen", GroupID: "todos_write", Method: "POST", PathPattern: "/api/v1/todos/:id/reopen", Label: "Reopen todo", Description: "Reopen a todo."},
	{ID: "todos:comments:create", GroupID: "todos_write", Method: "POST", PathPattern: "/api/v1/todos/:id/comments", Label: "Create comment", Description: "Create a comment on a todo."},
	{ID: "todos:comments:delete", GroupID: "todos_write", Method: "DELETE", PathPattern: "/api/v1/todos/:id/comments/:cid", Label: "Delete comment", Description: "Delete a comment from a todo."},
	{ID: "summaries:create", GroupID: "summary", Method: "POST", PathPattern: "/api/v1/summaries", Label: "Create summary", Description: "Create an AI summary."},
	{ID: "summaries:list", GroupID: "summary", Method: "GET", PathPattern: "/api/v1/summaries", Label: "List summaries", Description: "Read summary history."},
	{ID: "summaries:stream", GroupID: "summary", Method: "GET", PathPattern: "/api/v1/summaries/:id/stream", Label: "Stream summary", Description: "Stream summary generation output."},
	{ID: "summaries:get", GroupID: "summary", Method: "GET", PathPattern: "/api/v1/summaries/:id", Label: "Get summary", Description: "Read a summary."},
	{ID: "summaries:delete", GroupID: "summary", Method: "DELETE", PathPattern: "/api/v1/summaries/:id", Label: "Delete summary", Description: "Delete a summary."},
	{ID: "summaries:followup:create", GroupID: "summary", Method: "POST", PathPattern: "/api/v1/summaries/:id/followup", Label: "Create follow-up", Description: "Create a summary follow-up."},
	{ID: "summaries:followups:list", GroupID: "summary", Method: "GET", PathPattern: "/api/v1/summaries/:id/followups", Label: "List follow-ups", Description: "Read summary follow-ups."},
}

var PermissionGroups = []PermissionGroup{
	{
		ID:            "todos_read",
		Label:         "Read APIs",
		Description:   "Read-only todo APIs.",
		PermissionIDs: []string{"todos:list", "todos:graph", "todos:tags", "todos:by_date_range", "todos:get", "todos:comments:list"},
	},
	{
		ID:            "todos_write",
		Label:         "Write APIs",
		Description:   "Todo mutation APIs.",
		PermissionIDs: []string{"todos:create", "todos:update", "todos:delete", "todos:start", "todos:set_status", "todos:pin", "todos:highlight", "todos:complete", "todos:reopen", "todos:comments:create", "todos:comments:delete"},
	},
	{
		ID:            "summary",
		Label:         "Summary APIs",
		Description:   "AI summary and follow-up APIs.",
		PermissionIDs: []string{"summaries:create", "summaries:list", "summaries:stream", "summaries:get", "summaries:delete", "summaries:followup:create", "summaries:followups:list"},
	},
}

var PresetRead = []string{"todos:list", "todos:graph", "todos:tags", "todos:by_date_range", "todos:get", "todos:comments:list"}
var PresetWrite = []string{"todos:create", "todos:update", "todos:delete", "todos:start", "todos:set_status", "todos:pin", "todos:highlight", "todos:complete", "todos:reopen", "todos:comments:create", "todos:comments:delete"}
var PresetReadWrite = []string{"todos:list", "todos:graph", "todos:tags", "todos:by_date_range", "todos:get", "todos:comments:list", "todos:create", "todos:update", "todos:delete", "todos:start", "todos:set_status", "todos:pin", "todos:highlight", "todos:complete", "todos:reopen", "todos:comments:create", "todos:comments:delete"}
var PresetSummary = []string{"summaries:create", "summaries:list", "summaries:stream", "summaries:get", "summaries:delete", "summaries:followup:create", "summaries:followups:list"}

func PermissionCatalogResponse() PermissionCatalog {
	return PermissionCatalog{
		APIs:   AllAPIPermissions,
		Groups: PermissionGroups,
		Presets: map[string][]string{
			"read":       PresetRead,
			"write":      PresetWrite,
			"read_write": PresetReadWrite,
			"summary":    PresetSummary,
		},
	}
}

func PermissionExists(id string) bool {
	for _, perm := range AllAPIPermissions {
		if perm.ID == id {
			return true
		}
	}
	return false
}
