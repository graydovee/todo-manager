package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
)

func CSRF(skipPrefixes ...string) echo.MiddlewareFunc {
	skipper := func(c echo.Context) bool {
		if authHeader := c.Request().Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			return true
		}
		path := c.Request().URL.Path
		for _, prefix := range skipPrefixes {
			if strings.HasPrefix(path, prefix) {
				return true
			}
		}
		return false
	}

	return echoMiddleware.CSRFWithConfig(echoMiddleware.CSRFConfig{
		TokenLookup:    "header:X-CSRF-Token",
		CookieName:     "_csrf",
		CookiePath:     "/",
		CookieHTTPOnly: false,
		CookieSameSite: http.SameSiteLaxMode,
		Skipper:        skipper,
	})
}
