package handler

import (
	"net/http"

	"github.com/graydovee/todolist/internal/middleware"
	"github.com/graydovee/todolist/internal/service"
	"github.com/labstack/echo/v4"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

type AuthModeResponse struct {
	Mode string `json:"mode"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type UserResponse struct {
	ID          uint   `json:"id"`
	DisplayName string `json:"display_name"`
}

func (h *AuthHandler) GetMode(c echo.Context) error {
	return c.JSON(http.StatusOK, AuthModeResponse{Mode: h.authService.GetAuthMode()})
}

func (h *AuthHandler) GetMe(c echo.Context) error {
	user := middleware.GetUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
	}
	return c.JSON(http.StatusOK, UserResponse{
		ID:          user.ID,
		DisplayName: user.DisplayName,
	})
}

func (h *AuthHandler) Login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
	}
	if req.Username == "" || req.Password == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "username and password required"})
	}

	user, err := h.authService.LoginBasic(c.Response(), c.Request(), req.Username, req.Password)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "invalid credentials"})
	}

	return c.JSON(http.StatusOK, UserResponse{
		ID:          user.ID,
		DisplayName: user.DisplayName,
	})
}

func (h *AuthHandler) LoginOIDC(c echo.Context) error {
	authURL, err := h.authService.InitOIDCLogin(c.Response(), c.Request())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to init OIDC login"})
	}
	return c.Redirect(http.StatusFound, authURL)
}

func (h *AuthHandler) CallbackOIDC(c echo.Context) error {
	code := c.QueryParam("code")
	state := c.QueryParam("state")
	if code == "" || state == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "missing code or state"})
	}

	_, err := h.authService.HandleOIDCCallback(c.Request().Context(), c.Response(), c.Request(), code, state)
	if err != nil {
		c.Logger().Errorf("OIDC callback failed: %v", err)
		return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "OIDC callback failed"})
	}

	return c.Redirect(http.StatusFound, "/")
}

func (h *AuthHandler) Logout(c echo.Context) error {
	h.authService.Logout(c.Response(), c.Request())
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *AuthHandler) CSRFToken(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"csrf_token": c.Get("csrf").(string)})
}
