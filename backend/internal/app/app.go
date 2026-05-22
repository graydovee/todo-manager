package app

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/graydovee/todolist/internal/auth"
	"github.com/graydovee/todolist/internal/config"
	"github.com/graydovee/todolist/internal/handler"
	"github.com/graydovee/todolist/internal/middleware"
	"github.com/graydovee/todolist/internal/repository"
	"github.com/graydovee/todolist/internal/service"
	"github.com/graydovee/todolist/internal/session"
	"github.com/graydovee/todolist/static"
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

	e.Use(middleware.CSRF("/api/v1/auth/login", "/api/v1/auth/callback"))

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
	todoRepo := repository.NewTodoRepo(db)
	tagRepo := repository.NewTagRepo(db)
	relationRepo := repository.NewRelationRepo(db)
	counterRepo := repository.NewCodeCounterRepo(db)
	commentRepo := repository.NewCommentRepo(db)

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
			slog.Error("failed to initialize OIDC provider", "error", err)
		}
	}

	summaryRepo := repository.NewSummaryRepo(db)

	llmClient := service.NewLLMClient(&cfg.LLM)

	authService := service.NewAuthService(cfg, basicAuthProvider, oidcAuthProvider, userRepo, sessionStore)
	todoService := service.NewTodoService(db, todoRepo, tagRepo, relationRepo, counterRepo)
	commentService := service.NewCommentService(db, commentRepo, todoRepo)
	summaryService := service.NewSummaryService(db, summaryRepo, todoRepo, llmClient, &cfg.LLM)

	authHandler := handler.NewAuthHandler(authService)
	todoHandler := handler.NewTodoHandler(todoService, commentService, todoRepo, tagRepo, relationRepo, db)
	summaryHandler := handler.NewSummaryHandler(summaryService)

	api := e.Group("/api/v1")

	authGroup := api.Group("/auth")
	authGroup.GET("/mode", authHandler.GetMode)
	authGroup.GET("/csrf", authHandler.CSRFToken)
	authGroup.POST("/login", authHandler.Login)
	authGroup.GET("/login", authHandler.LoginOIDC)
	authGroup.GET("/callback", authHandler.CallbackOIDC)
	authGroup.POST("/logout", authHandler.Logout, middleware.Auth(sessionStore, userRepo))
	authGroup.GET("/me", authHandler.GetMe, middleware.Auth(sessionStore, userRepo))

	authMW := middleware.Auth(sessionStore, userRepo)
	todos := api.Group("/todos", authMW)
	todos.GET("", todoHandler.List)
	todos.GET("/graph", todoHandler.Graph)
	todos.GET("/tags", todoHandler.Tags)
	todos.GET("/:id", todoHandler.Get)
	todos.POST("", todoHandler.Create)
	todos.PATCH("/:id", todoHandler.Update)
	todos.DELETE("/:id", todoHandler.Delete)
	todos.POST("/:id/start", todoHandler.Start)
	todos.PATCH("/:id/status", todoHandler.SetStatus)
	todos.PATCH("/:id/pin", todoHandler.Pin)
	todos.PATCH("/:id/highlight", todoHandler.Highlight)
	todos.POST("/:id/complete", todoHandler.Complete)
	todos.POST("/:id/reopen", todoHandler.Reopen)
	todos.GET("/:id/comments", todoHandler.ListComments)
	todos.POST("/:id/comments", todoHandler.CreateComment)
	todos.DELETE("/:id/comments/:cid", todoHandler.DeleteComment)

	summaries := api.Group("/summaries", authMW)
	summaries.POST("", summaryHandler.Create)
	summaries.GET("", summaryHandler.List)
	summaries.GET("/:id", summaryHandler.Get)
	summaries.DELETE("/:id", summaryHandler.Delete)

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
