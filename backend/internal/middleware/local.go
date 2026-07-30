package middleware

import (
	"net/http"
	"sync"

	"github.com/graydovee/todo-manager/internal/model"
	"github.com/graydovee/todo-manager/internal/repository"
	"github.com/labstack/echo/v4"
)

// LocalAuth is the auth middleware for "none" mode (auth.mode=none), used by the
// embedded desktop sidecar. All requests are attributed to a single fixed local
// user, lazily created on first use via UserRepo.UpsertByAuthProvider. The
// resulting principal is of type PrincipalSession so it automatically bypasses
// the per-permission scope checks in RequirePermission — equivalent to full
// access, which is appropriate for a single-user local instance.
//
// The resolved user is cached after the first request so we don't hit the DB on
// every subsequent call.
func LocalAuth(userRepo *repository.UserRepo, displayName string) echo.MiddlewareFunc {
	var (
		once     sync.Once
		cached   *model.User
		cacheErr error
	)

	resolve := func() (*model.User, error) {
		once.Do(func() {
			cached, cacheErr = userRepo.UpsertByAuthProvider(&model.User{
				AuthProvider: "local",
				AuthSubject:  "local",
				DisplayName:  displayName,
			})
		})
		return cached, cacheErr
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, err := resolve()
			if err != nil || user == nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "local user unavailable"})
			}
			setSessionPrincipal(c, user)
			return next(c)
		}
	}
}
