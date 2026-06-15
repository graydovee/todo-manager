package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/graydovee/todo-manager/internal/middleware"
	"github.com/graydovee/todo-manager/internal/model"
	"github.com/graydovee/todo-manager/internal/repository"
	"github.com/graydovee/todo-manager/internal/service"
	"github.com/labstack/echo/v4"
)

// FollowupHandler handles HTTP requests for the follow-up feature.
type FollowupHandler struct {
	followupService *service.FollowupService
	followupRepo    *repository.FollowupRepo
}

// NewFollowupHandler creates a new FollowupHandler with the given dependencies.
func NewFollowupHandler(followupService *service.FollowupService, followupRepo *repository.FollowupRepo) *FollowupHandler {
	return &FollowupHandler{
		followupService: followupService,
		followupRepo:    followupRepo,
	}
}

// FollowupRequest represents the request body for the followup endpoint.
type FollowupRequest struct {
	Question        string                   `json:"question"`
	ContextMessages []service.ContextMessage `json:"context_messages"`
}

// followupDoneEvent represents the JSON payload for the SSE done event.
type followupDoneEvent struct {
	FollowupMessageID uint `json:"followup_message_id"`
	VersionID         uint `json:"version_id"`
	VersionNumber     int  `json:"version_number"`
}

// Followup handles POST /api/v1/summaries/:id/followup
// Streams AI response via SSE.
func (h *FollowupHandler) Followup(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
	}

	var req FollowupRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
	}

	ctx := c.Request().Context()
	ch, followupMsg, err := h.followupService.AskFollowup(ctx, uint(id), user.ID, req.Question, req.ContextMessages)
	if err != nil {
		// Map service errors to appropriate HTTP status codes
		errMsg := err.Error()
		switch errMsg {
		case "summary not found":
			return c.JSON(http.StatusNotFound, ErrorResponse{Error: errMsg})
		case "follow-up is only available for completed summaries",
			"a non-empty question is required",
			"question exceeds maximum length of 1000 characters",
			"context_messages exceeds maximum of 20 items",
			"context_messages role must be 'user' or 'assistant'",
			"context_messages content exceeds maximum length of 2000 characters":
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: errMsg})
		default:
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		}
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

	for chunk := range ch {
		if chunk.Done {
			// Query the latest version for this message to include in done event
			doneData := h.buildDoneEventData(followupMsg.ID)
			fmt.Fprintf(resp, "event: done\ndata: %s\n\n", doneData)
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
	doneData := h.buildDoneEventData(followupMsg.ID)
	fmt.Fprintf(resp, "event: done\ndata: %s\n\n", doneData)
	flusher.Flush()
	return nil
}

// buildDoneEventData constructs the JSON payload for the done SSE event.
func (h *FollowupHandler) buildDoneEventData(messageID uint) string {
	evt := followupDoneEvent{
		FollowupMessageID: messageID,
		VersionID:         0,
		VersionNumber:     1,
	}

	// Query the latest version for this message
	version, err := h.followupRepo.FindLatestVersionByMessageID(nil, messageID)
	if err == nil && version != nil {
		evt.VersionID = version.ID
		evt.VersionNumber = version.VersionNumber
	}

	data, _ := json.Marshal(evt)
	return string(data)
}

// ListFollowups handles GET /api/v1/summaries/:id/followups
// Returns all followup messages with versions for a summary.
func (h *FollowupHandler) ListFollowups(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
	}

	messages, err := h.followupService.ListFollowups(uint(id), user.ID)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "summary not found" {
			return c.JSON(http.StatusNotFound, ErrorResponse{Error: errMsg})
		}
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
	}

	if messages == nil {
		messages = []*model.FollowupMessage{}
	}

	return c.JSON(http.StatusOK, messages)
}
