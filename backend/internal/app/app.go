package app

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/graydovee/todo-manager/internal/auth"
	"github.com/graydovee/todo-manager/internal/config"
	"github.com/graydovee/todo-manager/internal/handler"
	"github.com/graydovee/todo-manager/internal/middleware"
	"github.com/graydovee/todo-manager/internal/repository"
	"github.com/graydovee/todo-manager/internal/service"
	"github.com/graydovee/todo-manager/internal/session"
	"github.com/graydovee/todo-manager/static"
	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
	"gorm.io/gorm"
)

func New(cfg *config.Config, db *gorm.DB) *echo.Echo {
	e := echo.New()
	e.HideBanner = true

	e.Use(echoMiddleware.RequestID())
	e.Use(echoMiddleware.Recover())
	e.Use(middleware.CORS(cfg.Server.CORSOrigins))
	e.Use(middleware.RequestLogger())

	// "none" mode (embedded desktop sidecar) has no sessions, so CSRF and the
	// auth/access-key routes are not applicable. Browsers never drive this
	// instance directly — the desktop client goes through the Tauri HTTP plugin.
	if cfg.Auth.Mode != "none" {
		e.Use(middleware.CSRF("/api/v1/auth/login", "/api/v1/auth/callback"))
	}

	e.GET("/health", func(c echo.Context) error {
		sqlDB, err := db.DB()
		if err != nil {
			return c.JSON(500, map[string]string{"status": "error"})
		}
		if err := sqlDB.Ping(); err != nil {
			return c.JSON(500, map[string]string{"status": "error"})
		}
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	userRepo := repository.NewUserRepo(db)
	accessKeyRepo := repository.NewAccessKeyRepo(db)
	todoRepo := repository.NewTodoRepo(db)
	tagRepo := repository.NewTagRepo(db)
	relationRepo := repository.NewRelationRepo(db)
	counterRepo := repository.NewCodeCounterRepo(db)
	commentRepo := repository.NewCommentRepo(db)
	statusHistoryRepo := repository.NewStatusHistoryRepo(db)

	// authMW is applied to every /todos and /summaries route below. In "none"
	// mode it injects a fixed local user; otherwise it delegates to the usual
	// session-or-access-key resolution.
	var authMW echo.MiddlewareFunc
	var sessionOnlyMW echo.MiddlewareFunc
	var authService *service.AuthService
	var accessKeyService *service.AccessKeyService
	if cfg.Auth.Mode == "none" {
		authMW = middleware.LocalAuth(userRepo, cfg.Auth.Local.DisplayName)
		sessionOnlyMW = authMW
	} else {
		sessionStore := session.NewDBStore(db, cfg.Session.Secret, cfg.Session.MaxAge, cfg.Session.CleanupInterval)

		var basicAuthProvider *auth.BasicAuthProvider
		if cfg.Auth.Mode == "basic" {
			basicAuthProvider = auth.NewBasicAuthProvider(&cfg.Auth.Basic)
		}

		var oidcAuthProvider *auth.OIDCAuthProvider
		if cfg.Auth.Mode == "oidc" {
			var err error
			oidcAuthProvider, err = auth.NewOIDCAuthProvider(context.Background(), &cfg.Auth.OIDC)
			if err != nil {
				// Fatal: a nil provider would panic on every login attempt. Crash so
				// the scheduler restarts the pod and retries provider discovery,
				// instead of serving with a permanently broken auth path.
				slog.Error("failed to initialize OIDC provider", "error", err)
				os.Exit(1)
			}
		}

		authService = service.NewAuthService(cfg, basicAuthProvider, oidcAuthProvider, userRepo, sessionStore)
		accessKeyService = service.NewAccessKeyService(db, accessKeyRepo, userRepo)
		sessionOnlyMW = middleware.SessionAuth(sessionStore, userRepo)
		authMW = middleware.AuthEither(sessionStore, userRepo, accessKeyService)
	}

	summaryRepo := repository.NewSummaryRepo(db)
	followupRepo := repository.NewFollowupRepo(db)

	llmClient := service.NewLLMClient(&cfg.LLM)

	todoService := service.NewTodoService(db, todoRepo, tagRepo, relationRepo, counterRepo, statusHistoryRepo)
	commentService := service.NewCommentService(db, commentRepo, todoRepo)
	summaryService := service.NewSummaryService(db, summaryRepo, todoRepo, commentRepo, relationRepo, statusHistoryRepo, llmClient, &cfg.LLM)
	followupService := service.NewFollowupService(db, followupRepo, summaryRepo, llmClient, &cfg.LLM)

	todoHandler := handler.NewTodoHandler(todoService, commentService, todoRepo, tagRepo, relationRepo, db)
	summaryHandler := handler.NewSummaryHandler(summaryService)
	followupHandler := handler.NewFollowupHandler(followupService, followupRepo)

	api := e.Group("/api/v1")

	// Auth and access-key management routes only exist when there is a real auth
	// backend (basic/oidc). They are meaningless in single-user "none" mode.
	if cfg.Auth.Mode != "none" {
		authHandler := handler.NewAuthHandler(authService)
		accessKeyHandler := handler.NewAccessKeyHandler(accessKeyService)

		authGroup := api.Group("/auth")
		authGroup.GET("/mode", authHandler.GetMode)
		authGroup.GET("/csrf", authHandler.CSRFToken)
		authGroup.POST("/login", authHandler.Login)
		authGroup.GET("/login", authHandler.LoginOIDC)
		authGroup.GET("/callback", authHandler.CallbackOIDC)
		authGroup.POST("/logout", authHandler.Logout, sessionOnlyMW)
		authGroup.GET("/me", authHandler.GetMe, sessionOnlyMW)

		accessKeys := api.Group("/access-keys", sessionOnlyMW)
		accessKeys.GET("", accessKeyHandler.List)
		accessKeys.GET("/permissions", accessKeyHandler.Permissions)
		accessKeys.POST("", accessKeyHandler.Create)
		accessKeys.POST("/:id/rotate", accessKeyHandler.Rotate)
		accessKeys.DELETE("/:id", accessKeyHandler.Delete)
	}

	todos := api.Group("/todos")
	todos.GET("", todoHandler.List, authMW, middleware.RequirePermission("todos:list"))
	todos.GET("/graph", todoHandler.Graph, authMW, middleware.RequirePermission("todos:graph"))
	todos.GET("/tags", todoHandler.Tags, authMW, middleware.RequirePermission("todos:tags"))
	todos.GET("/by-date-range", todoHandler.ListByDateRange, authMW, middleware.RequirePermission("todos:by_date_range"))
	todos.GET("/:id", todoHandler.Get, authMW, middleware.RequirePermission("todos:get"))
	todos.POST("", todoHandler.Create, authMW, middleware.RequirePermission("todos:create"))
	todos.PATCH("/:id", todoHandler.Update, authMW, middleware.RequirePermission("todos:update"))
	todos.DELETE("/:id", todoHandler.Delete, authMW, middleware.RequirePermission("todos:delete"))
	todos.POST("/:id/start", todoHandler.Start, authMW, middleware.RequirePermission("todos:start"))
	todos.PATCH("/:id/status", todoHandler.SetStatus, authMW, middleware.RequirePermission("todos:set_status"))
	todos.PATCH("/:id/pin", todoHandler.Pin, authMW, middleware.RequirePermission("todos:pin"))
	todos.PATCH("/:id/highlight", todoHandler.Highlight, authMW, middleware.RequirePermission("todos:highlight"))
	todos.POST("/:id/complete", todoHandler.Complete, authMW, middleware.RequirePermission("todos:complete"))
	todos.POST("/:id/reopen", todoHandler.Reopen, authMW, middleware.RequirePermission("todos:reopen"))
	todos.GET("/:id/comments", todoHandler.ListComments, authMW, middleware.RequirePermission("todos:comments:list"))
	todos.POST("/:id/comments", todoHandler.CreateComment, authMW, middleware.RequirePermission("todos:comments:create"))
	todos.DELETE("/:id/comments/:cid", todoHandler.DeleteComment, authMW, middleware.RequirePermission("todos:comments:delete"))

	summaries := api.Group("/summaries")
	summaries.POST("", summaryHandler.Create, authMW, middleware.RequirePermission("summaries:create"))
	summaries.GET("", summaryHandler.List, authMW, middleware.RequirePermission("summaries:list"))
	summaries.GET("/:id/stream", summaryHandler.Stream, authMW, middleware.RequirePermission("summaries:stream"))
	summaries.GET("/:id", summaryHandler.Get, authMW, middleware.RequirePermission("summaries:get"))
	summaries.DELETE("/:id", summaryHandler.Delete, authMW, middleware.RequirePermission("summaries:delete"))
	summaries.POST("/:id/followup", followupHandler.Followup, authMW, middleware.RequirePermission("summaries:followup:create"))
	summaries.GET("/:id/followups", followupHandler.ListFollowups, authMW, middleware.RequirePermission("summaries:followups:list"))

	registerSPA(e)

	return e
}

func registerSPA(e *echo.Echo) {
	sub, err := fs.Sub(static.FS, "frontend_dist")
	if err != nil {
		if _, statErr := os.Stat("frontend_dist"); statErr == nil {
			fileServer := http.FileServer(http.Dir("frontend_dist"))
			e.GET("/*", echo.WrapHandler(http.StripPrefix("/", fileServer)))
		}
		return
	}

	fileServer := http.FileServer(http.FS(sub))
	e.GET("/assets/*", echo.WrapHandler(fileServer))

	e.GET("/*", func(c echo.Context) error {
		reqPath := c.Request().URL.Path
		if strings.HasPrefix(reqPath, "/api/") {
			return c.JSON(404, map[string]string{"error": "not found"})
		}

		cleanPath := strings.TrimPrefix(reqPath, "/")
		if cleanPath != "" {
			f, err := sub.Open(cleanPath)
			if err == nil {
				f.Close()
				http.FileServer(http.FS(sub)).ServeHTTP(c.Response(), c.Request())
				return nil
			}
		}

		data, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			return c.String(500, "index.html not found")
		}
		return c.Blob(200, "text/html; charset=utf-8", data)
	})
}

func GetAddr(cfg *config.Config) string {
	return fmt.Sprintf(":%d", cfg.Server.Port)
}
