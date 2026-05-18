package middleware

import (
	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
)

func CORS(allowOrigins []string) echo.MiddlewareFunc {
	if len(allowOrigins) == 0 {
		allowOrigins = []string{"*"}
	}
	return echoMiddleware.CORSWithConfig(echoMiddleware.CORSConfig{
		AllowOrigins:     allowOrigins,
		AllowMethods:     []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
	})
}
