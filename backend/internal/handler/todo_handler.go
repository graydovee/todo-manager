package handler

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/graydovee/todolist/internal/middleware"
	"github.com/graydovee/todolist/internal/model"
	"github.com/graydovee/todolist/internal/repository"
	"github.com/graydovee/todolist/internal/service"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type TodoHandler struct {
	todoService    *service.TodoService
	commentService *service.CommentService
	todoRepo       *repository.TodoRepo
	tagRepo        *repository.TagRepo
	relationRepo   *repository.RelationRepo
	db             *gorm.DB
}

func NewTodoHandler(todoService *service.TodoService, commentService *service.CommentService, todoRepo *repository.TodoRepo, tagRepo *repository.TagRepo, relationRepo *repository.RelationRepo, db *gorm.DB) *TodoHandler {
	return &TodoHandler{
		todoService:    todoService,
		commentService: commentService,
		todoRepo:       todoRepo,
		tagRepo:        tagRepo,
		relationRepo:   relationRepo,
		db:             db,
	}
}

func (h *TodoHandler) List(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}

	filters := repository.TodoFilters{
		Query:     c.QueryParam("q"),
		Code:      c.QueryParam("code"),
		Category:  c.QueryParam("category"),
		Priority:  c.QueryParam("priority"),
		SortBy:    c.QueryParam("sort_by"),
		SortOrder: c.QueryParam("sort_order"),
	}

	if tags := c.QueryParam("tag"); tags != "" {
		filters.Tags = strings.Split(tags, ",")
	}

	if status := c.QueryParam("status"); status != "" {
		filters.Status = status
	}

	filters.Page = queryParamInt(c, "page", 1)
	filters.PageSize = queryParamInt(c, "page_size", 20)

	todos, total, err := h.todoService.ListTodos(user.ID, filters)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
	}

	items := make([]TodoResponse, len(todos))
	for i, todo := range todos {
		items[i] = todoToResponse(todo)
	}

	return c.JSON(http.StatusOK, PaginatedResponse{
		Items:    items,
		Total:    total,
		Page:     filters.Page,
		PageSize: filters.PageSize,
	})
}

func (h *TodoHandler) Tags(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}

	tags, err := h.tagRepo.FindDistinctByUser(nil, user.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
	}

	if tags == nil {
		tags = []string{}
	}

	sort.Slice(tags, func(i, j int) bool {
		return strings.ToLower(tags[i]) < strings.ToLower(tags[j])
	})

	return c.JSON(http.StatusOK, tags)
}

func (h *TodoHandler) Get(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
	}

	todo, err := h.todoService.GetTodo(user.ID, uint(id))
	if err != nil {
		return c.JSON(http.StatusNotFound, ErrorResponse{Error: "todo not found"})
	}

	detail := h.buildDetailResponse(todo, user.ID)
	return c.JSON(http.StatusOK, detail)
}

func (h *TodoHandler) Graph(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}

	graph, err := h.todoService.GetTodoGraph(user.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
	}

	nodes := make([]TodoGraphNodeResponse, len(graph.Nodes))
	for i, node := range graph.Nodes {
		nodes[i] = TodoGraphNodeResponse{
			ID:                node.ID,
			Code:              node.Code,
			Title:             node.Title,
			Category:          node.Category,
			Priority:          node.Priority,
			Status:            node.Status,
			DueAt:             node.DueAt,
			PrerequisiteCount: node.PrerequisiteCount,
			DependentCount:    node.DependentCount,
			ComponentID:       node.ComponentID,
			IsComponentRoot:   node.IsComponentRoot,
		}
	}

	edges := make([]TodoGraphEdgeResponse, len(graph.Edges))
	for i, edge := range graph.Edges {
		edges[i] = TodoGraphEdgeResponse{
			SourceID: edge.SourceID,
			TargetID: edge.TargetID,
		}
	}

	components := make([]TodoGraphComponentResponse, len(graph.Components))
	for i, component := range graph.Components {
		components[i] = TodoGraphComponentResponse{
			ID:            component.ID,
			RootIDs:       component.RootIDs,
			RootSummaries: toDTOs(component.RootSummaries),
			NodeIDs:       component.NodeIDs,
			AllCompleted:  component.AllCompleted,
		}
	}

	return c.JSON(http.StatusOK, TodoGraphResponse{
		Nodes:      nodes,
		Edges:      edges,
		Components: components,
	})
}

func (h *TodoHandler) Create(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}

	var input service.CreateTodoInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
	}

	if input.Title == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "title is required"})
	}
	if input.Category == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "category is required"})
	}

	todo, err := h.todoService.CreateTodo(user.ID, input)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusCreated, todoToResponse(todo))
}

func (h *TodoHandler) Update(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
	}

	var input service.UpdateTodoInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
	}

	todo, err := h.todoService.UpdateTodo(user.ID, uint(id), input)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, todoToResponse(todo))
}

func (h *TodoHandler) Delete(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
	}

	if err := h.todoService.DeleteTodo(user.ID, uint(id)); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *TodoHandler) Start(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
	}

	if err := h.todoService.StartTodo(user.ID, uint(id)); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *TodoHandler) SetStatus(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
	}

	var req SetStatusRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
	}

	if err := h.todoService.SetStatus(user.ID, uint(id), req.Status); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *TodoHandler) Complete(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
	}

	var req CompleteRequest
	c.Bind(&req)

	conflict, err := h.todoService.CompleteTodo(user.ID, uint(id), req.CascadeDependencies)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	}
	if conflict != nil {
		return c.JSON(http.StatusConflict, ConflictResponse{
			Error:   "pending dependencies",
			Pending: toDTOs(conflict.PendingDependencies),
		})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *TodoHandler) Reopen(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
	}

	var req ReopenRequest
	c.Bind(&req)

	conflict, err := h.todoService.ReopenTodo(user.ID, uint(id), req.CascadeDependents)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	}
	if conflict != nil {
		return c.JSON(http.StatusConflict, ConflictResponse{
			Error:     "completed dependents",
			Completed: toDTOs(conflict.CompletedDependents),
		})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *TodoHandler) ListComments(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
	}

	comments, err := h.commentService.ListByTodoID(user.ID, uint(id))
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, comments)
}

func (h *TodoHandler) CreateComment(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
	}

	var req CreateCommentRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
	}

	comment, err := h.commentService.Create(user.ID, uint(id), req.Content)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusCreated, comment)
}

func (h *TodoHandler) DeleteComment(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}

	cid, err := strconv.ParseUint(c.Param("cid"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid comment id"})
	}

	if err := h.commentService.Delete(user.ID, uint(cid)); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *TodoHandler) Pin(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
	}

	var req PinRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
	}

	todo, err := h.todoRepo.FindByIDWithDetails(nil, uint(id), user.ID)
	if err != nil {
		return c.JSON(http.StatusNotFound, ErrorResponse{Error: "todo not found"})
	}

	todo.Pinned = req.Pinned
	if err := h.todoRepo.Update(nil, todo); err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, todoToResponse(todo))
}

func (h *TodoHandler) Highlight(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
	}

	var req HighlightRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
	}

	todo, err := h.todoRepo.FindByIDWithDetails(nil, uint(id), user.ID)
	if err != nil {
		return c.JSON(http.StatusNotFound, ErrorResponse{Error: "todo not found"})
	}

	todo.Highlighted = req.Highlighted
	if err := h.todoRepo.Update(nil, todo); err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, todoToResponse(todo))
}

func (h *TodoHandler) buildDetailResponse(todo *model.Todo, userID uint) TodoDetailResponse {
	resp := TodoDetailResponse{TodoResponse: todoToResponse(todo)}

	var dependsOn, dependedBy, duplicates []TodoSummaryDTO
	var duplicateOf *TodoSummaryDTO

	for _, rel := range todo.Relations {
		var relatedTodo model.Todo
		switch rel.RelationType {
		case model.RelationDependsOn:
			if h.db.Select("id, code, title, category, status").Where("id = ? AND user_id = ?", rel.TargetID, userID).First(&relatedTodo).Error == nil {
				dependsOn = append(dependsOn, TodoSummaryDTO{ID: relatedTodo.ID, Code: relatedTodo.Code, Title: relatedTodo.Title, Category: relatedTodo.Category, Status: relatedTodo.Status})
			}
		case model.RelationDuplicateOf:
			if h.db.Select("id, code, title, category, status").Where("id = ? AND user_id = ?", rel.TargetID, userID).First(&relatedTodo).Error == nil {
				duplicateOf = &TodoSummaryDTO{ID: relatedTodo.ID, Code: relatedTodo.Code, Title: relatedTodo.Title, Category: relatedTodo.Category, Status: relatedTodo.Status}
			}
		}
	}

	var reverseRels []model.TodoRelation
	h.db.Where("target_id = ?", todo.ID).Find(&reverseRels)
	for _, rel := range reverseRels {
		var relatedTodo model.Todo
		if h.db.Select("id, code, title, category, status").Where("id = ? AND user_id = ?", rel.SourceID, userID).First(&relatedTodo).Error != nil {
			continue
		}
		switch rel.RelationType {
		case model.RelationDependsOn:
			dependedBy = append(dependedBy, TodoSummaryDTO{ID: relatedTodo.ID, Code: relatedTodo.Code, Title: relatedTodo.Title, Category: relatedTodo.Category, Status: relatedTodo.Status})
		case model.RelationDuplicateOf:
			duplicates = append(duplicates, TodoSummaryDTO{ID: relatedTodo.ID, Code: relatedTodo.Code, Title: relatedTodo.Title, Category: relatedTodo.Category, Status: relatedTodo.Status})
		}
	}

	resp.DependsOn = ensureNonNil(dependsOn)
	resp.DependedBy = ensureNonNil(dependedBy)
	resp.Duplicates = ensureNonNil(duplicates)
	resp.DuplicateOf = duplicateOf

	return resp
}

func toDTOs(summaries []service.TodoSummary) []TodoSummaryDTO {
	result := make([]TodoSummaryDTO, len(summaries))
	for i, s := range summaries {
		result[i] = TodoSummaryDTO{ID: s.ID, Code: s.Code, Title: s.Title, Category: s.Category, Status: s.Status}
	}
	return result
}

func ensureNonNil(s []TodoSummaryDTO) []TodoSummaryDTO {
	if s == nil {
		return []TodoSummaryDTO{}
	}
	return s
}

func queryParamInt(c echo.Context, key string, defaultVal int) int {
	v := c.QueryParam(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}
