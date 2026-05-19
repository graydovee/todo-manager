package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/graydovee/todolist/internal/app"
	"github.com/graydovee/todolist/internal/config"
	"github.com/graydovee/todolist/internal/database"
	"github.com/graydovee/todolist/internal/repository"
	"github.com/graydovee/todolist/internal/service"
	"gorm.io/gorm/logger"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	initLogger(cfg)

	gormLogger := initGormLogger(cfg)

	db, err := database.NewDB(cfg, gormLogger)
	if err != nil {
		slog.Error("failed to connect database", "error", err)
		os.Exit(1)
	}

	if err := database.RunMigrations(db, cfg.DB.Driver); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Run data migration (old-format codes → sequential numeric codes)
	migrationSvc := service.NewMigrationService(db, repository.NewTodoRepo(db), repository.NewCodeCounterRepo(db))
	if err := migrationSvc.Run(); err != nil {
		slog.Error("failed to run data migration", "error", err)
		os.Exit(1)
	}

	e := app.New(cfg, db)

	go func() {
		addr := fmt.Sprintf(":%d", cfg.Server.Port)
		slog.Info("starting server", "addr", addr)
		if err := e.Start(addr); err != nil {
			slog.Info("server stopped", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server")
	if err := e.Shutdown(nil); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}

func initLogger(cfg *config.Config) {
	level := parseLogLevel(cfg.Log.Level)
	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}

	if cfg.Log.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler))
}

func initGormLogger(cfg *config.Config) logger.Interface {
	level := logger.Info
	switch cfg.Log.Level {
	case "silent":
		level = logger.Silent
	case "error":
		level = logger.Error
	case "warn":
		level = logger.Warn
	case "info":
		level = logger.Info
	}
	return logger.Default.LogMode(level)
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
