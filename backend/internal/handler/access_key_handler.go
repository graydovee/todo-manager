package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/graydovee/todo-manager/internal/middleware"
	"github.com/graydovee/todo-manager/internal/model"
	"github.com/graydovee/todo-manager/internal/service"
	"github.com/labstack/echo/v4"
)

type AccessKeyHandler struct {
	accessKeyService *service.AccessKeyService
}

func NewAccessKeyHandler(accessKeyService *service.AccessKeyService) *AccessKeyHandler {
	return &AccessKeyHandler{accessKeyService: accessKeyService}
}

type CreateAccessKeyRequest struct {
	Name           string  `json:"name"`
	AuthorizedAPIs []string `json:"authorized_apis"`
	ExpiresAt      string  `json:"expires_at,omitempty"`
}

type AccessKeyResponse struct {
	ID             uint       `json:"id"`
	Name           string     `json:"name"`
	KeyPrefix      string     `json:"key_prefix"`
	AuthorizedAPIs []string   `json:"authorized_apis"`
	ExpiresAt      *time.Time `json:"expires_at"`
	LastUsedAt     *time.Time `json:"last_used_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

type AccessKeyCreateResponse struct {
	AccessKeyResponse
	PlainKey string `json:"plain_key"`
}

func (h *AccessKeyHandler) List(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}
	keys, err := h.accessKeyService.List(user.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
	}
	resp := make([]AccessKeyResponse, 0, len(keys))
	for _, key := range keys {
		authorized, err := decodeAuthorizedAPIs(key)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		}
		resp = append(resp, toAccessKeyResponse(key, authorized))
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *AccessKeyHandler) Permissions(c echo.Context) error {
	return c.JSON(http.StatusOK, h.accessKeyService.PermissionCatalog())
}

func (h *AccessKeyHandler) Create(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}
	var req CreateAccessKeyRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
	}

	expiresAt, err := parseOptionalTime(req.ExpiresAt)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid expires_at"})
	}

	key, plainKey, err := h.accessKeyService.Create(user.ID, service.CreateAccessKeyInput{
		Name:           req.Name,
		AuthorizedAPIs: req.AuthorizedAPIs,
		ExpiresAt:      expiresAt,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	}

	authorized, err := decodeAuthorizedAPIs(key)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
	}

	return c.JSON(http.StatusCreated, AccessKeyCreateResponse{
		AccessKeyResponse: toAccessKeyResponse(key, authorized),
		PlainKey:          plainKey,
	})
}

func (h *AccessKeyHandler) Rotate(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
	}

	result, err := h.accessKeyService.Rotate(user.ID, uint(id))
	if err != nil {
		return c.JSON(http.StatusNotFound, ErrorResponse{Error: "access key not found"})
	}

	authorized, err := decodeAuthorizedAPIs(result.Key)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
	}

	return c.JSON(http.StatusOK, AccessKeyCreateResponse{
		AccessKeyResponse: toAccessKeyResponse(result.Key, authorized),
		PlainKey:          result.PlainKey,
	})
}

func (h *AccessKeyHandler) Delete(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
	}
	if err := h.accessKeyService.Delete(user.ID, uint(id)); err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func toAccessKeyResponse(key *model.AccessKey, authorized []string) AccessKeyResponse {
	return AccessKeyResponse{
		ID:             key.ID,
		Name:           key.Name,
		KeyPrefix:      key.KeyPrefix,
		AuthorizedAPIs: authorized,
		ExpiresAt:      key.ExpiresAt,
		LastUsedAt:     key.LastUsedAt,
		UpdatedAt:      key.UpdatedAt,
		CreatedAt:      key.CreatedAt,
	}
}
