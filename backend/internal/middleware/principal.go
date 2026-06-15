package middleware

import (
	"net/http"
	"strings"

	"github.com/graydovee/todolist/internal/model"
	"github.com/graydovee/todolist/internal/repository"
	"github.com/graydovee/todolist/internal/service"
	"github.com/graydovee/todolist/internal/session"
	"github.com/labstack/echo/v4"
)

const PrincipalContextKey = "auth_principal"

type PrincipalType string

const (
	PrincipalSession   PrincipalType = "session"
	PrincipalAccessKey PrincipalType = "access_key"
)

type AuthPrincipal struct {
	Type           PrincipalType
	User           *model.User
	AccessKeyID    uint
	AuthorizedAPIs map[string]struct{}
}

func SessionAuth(store *session.DBStore, userRepo *repository.UserRepo) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, err := store.GetUserID(c.Request())
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			}
			user, err := userRepo.FindByID(userID)
			if err != nil {
				store.DestroySession(c.Request(), c.Response())
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			}
			setSessionPrincipal(c, user)
			return next(c)
		}
	}
}

func AccessKeyAuth(accessKeyService *service.AccessKeyService, userRepo *repository.UserRepo) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token := bearerToken(c.Request().Header.Get("Authorization"))
			if token == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			}

			authResult, err := accessKeyService.Authenticate(token)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			}
			user, err := userRepo.FindByID(authResult.UserID)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			}
			setAccessKeyPrincipal(c, user, authResult.AccessKeyID, authResult.AuthorizedAPIs)
			return next(c)
		}
	}
}

func AuthEither(store *session.DBStore, userRepo *repository.UserRepo, accessKeyService *service.AccessKeyService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if bearerToken(c.Request().Header.Get("Authorization")) != "" {
				return AccessKeyAuth(accessKeyService, userRepo)(next)(c)
			}
			return SessionAuth(store, userRepo)(next)(c)
		}
	}
}

func RequirePermission(permissionID string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			principal := GetPrincipal(c)
			if principal == nil || principal.User == nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			}
			if principal.Type == PrincipalSession {
				return next(c)
			}
			if _, ok := principal.AuthorizedAPIs[permissionID]; !ok {
				return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
			}
			return next(c)
		}
	}
}

func GetPrincipal(c echo.Context) *AuthPrincipal {
	principal, ok := c.Get(PrincipalContextKey).(*AuthPrincipal)
	if !ok {
		return nil
	}
	return principal
}

func setSessionPrincipal(c echo.Context, user *model.User) {
	c.Set(PrincipalContextKey, &AuthPrincipal{
		Type: PrincipalSession,
		User: user,
	})
	c.Set(UserContextKey, user)
}

func setAccessKeyPrincipal(c echo.Context, user *model.User, accessKeyID uint, authorizedAPIs map[string]struct{}) {
	c.Set(PrincipalContextKey, &AuthPrincipal{
		Type:           PrincipalAccessKey,
		User:           user,
		AccessKeyID:    accessKeyID,
		AuthorizedAPIs: authorizedAPIs,
	})
	c.Set(UserContextKey, user)
}

func bearerToken(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
