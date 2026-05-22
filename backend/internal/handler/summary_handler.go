package handler

import (
	"errors"
	"net/http"
	"strconv"
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
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
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

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "start_date and end_date are required in ISO 8601 format"})
	}

	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "start_date and end_date are required in ISO 8601 format"})
	}

	summary, err := h.summaryService.CreateSummary(user.ID, startDate, endDate)
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
