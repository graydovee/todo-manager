package middleware

import (
	"github.com/graydovee/todolist/internal/model"
	"github.com/labstack/echo/v4"
)

const UserContextKey = "current_user"

func GetUser(c echo.Context) *model.User {
	user, ok := c.Get(UserContextKey).(*model.User)
	if !ok {
		return nil
	}
	return user
}
