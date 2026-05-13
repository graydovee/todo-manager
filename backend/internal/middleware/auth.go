package middleware

import (
	"net/http"

	"github.com/graydovee/todolist/internal/model"
	"github.com/graydovee/todolist/internal/repository"
	"github.com/graydovee/todolist/internal/session"
	"github.com/labstack/echo/v4"
)

const UserContextKey = "current_user"

func Auth(store *session.DBStore, userRepo *repository.UserRepo) echo.MiddlewareFunc {
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

			c.Set(UserContextKey, user)
			return next(c)
		}
	}
}

func GetUser(c echo.Context) *model.User {
	user, ok := c.Get(UserContextKey).(*model.User)
	if !ok {
		return nil
	}
	return user
}
