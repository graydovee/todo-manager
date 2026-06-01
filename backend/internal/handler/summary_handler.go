package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/graydovee/todolist/internal/middleware"
	"github.com/graydovee/todolist/internal/model"
	"github.com/graydovee/todolist/internal/service"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type SummaryHandler struct {
	summaryService *service.SummaryService
}

func NewSummaryHandler(summaryService *service.SummaryService) *SummaryHandler {
	return &SummaryHandler{
		summaryService: summaryService,
	}
}

type CreateSummaryRequest struct {
	StartDate    string `json:"start_date"`
	EndDate      string `json:"end_date"`
	TodoIDs      []uint `json:"todo_ids,omitempty"`
	Language     string `json:"language,omitempty"`      // "Chinese", "English", or ""
	CustomPrompt string `json:"custom_prompt,omitempty"` // optional, max 500 characters
}

func (h *SummaryHandler) Create(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}

	var req CreateSummaryRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
	}

	if req.StartDate == "" || req.EndDate == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "start_date and end_date are required in ISO 8601 format"})
	}

	// Validate language field
	if req.Language != "" && req.Language != "Chinese" && req.Language != "English" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid language value, must be one of: Chinese, English"})
	}

	// Validate and trim custom_prompt
	customPrompt := strings.TrimSpace(req.CustomPrompt)
	if len(req.CustomPrompt) > 500 {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "custom prompt exceeds maximum length of 500 characters"})
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "start_date and end_date are required in ISO 8601 format"})
	}

	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "start_date and end_date are required in ISO 8601 format"})
	}

	summary, err := h.summaryService.CreateSummaryWithTodos(user.ID, startDate, endDate, req.TodoIDs, req.Language, customPrompt)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusCreated, summary)
}

func (h *SummaryHandler) List(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}

	summaries, err := h.summaryService.ListSummaries(user.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
	}

	if summaries == nil {
		summaries = []*model.Summary{}
	}

	return c.JSON(http.StatusOK, summaries)
}

func (h *SummaryHandler) Get(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
	}

	summary, err := h.summaryService.GetSummary(user.ID, uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.JSON(http.StatusNotFound, ErrorResponse{Error: "summary not found"})
		}
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
	}

	return c.JSON(http.StatusOK, summary)
}

func (h *SummaryHandler) Delete(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
	}

	// Verify the summary exists and belongs to the user
	_, err = h.summaryService.GetSummary(user.ID, uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.JSON(http.StatusNotFound, ErrorResponse{Error: "summary not found"})
		}
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
	}

	if err := h.summaryService.DeleteSummary(user.ID, uint(id)); err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// writeSSEData writes content as a properly formatted SSE data event.
// Multi-line content is split into multiple "data:" fields within a single event.
// Per SSE spec, the client joins multiple data fields with "\n".
func writeSSEData(w io.Writer, content string) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprintf(w, "\n") // empty line terminates the event
}

// Stream handles GET /api/v1/summaries/:id/stream
// Sets SSE headers and forwards chunks from SummaryService to the client.
func (h *SummaryHandler) Stream(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
	}

	// First check if summary exists at all (for 404 vs 403 distinction)
	summary, err := h.summaryService.GetSummaryByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.JSON(http.StatusNotFound, ErrorResponse{Error: "summary not found"})
		}
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
	}

	// Check ownership
	if summary.UserID != user.ID {
		return c.JSON(http.StatusForbidden, ErrorResponse{Error: "forbidden"})
	}

	// Set SSE headers
	resp := c.Response()
	resp.Header().Set("Content-Type", "text/event-stream")
	resp.Header().Set("Cache-Control", "no-cache")
	resp.Header().Set("Connection", "keep-alive")
	resp.WriteHeader(http.StatusOK)

	flusher, ok := resp.Writer.(http.Flusher)
	if !ok {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "streaming not supported"})
	}

	switch summary.Status {
	case model.SummaryStatusCompleted:
		// Send stored result_content as a single data event + done event
		writeSSEData(resp, summary.ResultContent)
		flusher.Flush()
		fmt.Fprintf(resp, "event: done\ndata: \n\n")
		flusher.Flush()
		return nil

	case model.SummaryStatusError:
		// Send error event with stored error message
		fmt.Fprintf(resp, "event: error\ndata: %s\n\n", summary.ResultContent)
		flusher.Flush()
		return nil

	case model.SummaryStatusAnalyzing:
		// Call StreamAnalysis and forward chunks as SSE data events
		ctx := c.Request().Context()
		ch, err := h.summaryService.StreamAnalysis(ctx, uint(id), user.ID)
		if err != nil {
			// Send error event since we already wrote SSE headers
			fmt.Fprintf(resp, "event: error\ndata: %s\n\n", err.Error())
			flusher.Flush()
			return nil
		}

		for chunk := range ch {
			if chunk.Done {
				fmt.Fprintf(resp, "event: done\ndata: \n\n")
				flusher.Flush()
				return nil
			}
			if chunk.Err != nil {
				fmt.Fprintf(resp, "event: error\ndata: %s\n\n", chunk.Err.Error())
				flusher.Flush()
				return nil
			}
			// Forward content chunk as SSE data event
			writeSSEData(resp, chunk.Content)
			flusher.Flush()
		}

		// Channel closed without Done signal — send done event
		fmt.Fprintf(resp, "event: done\ndata: \n\n")
		flusher.Flush()
		return nil

	default:
		// Unknown status — send error
		fmt.Fprintf(resp, "event: error\ndata: unknown summary status\n\n")
		flusher.Flush()
		return nil
	}
}
