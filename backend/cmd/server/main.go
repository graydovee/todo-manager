package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/graydovee/todo-manager/internal/app"
	"github.com/graydovee/todo-manager/internal/config"
	"github.com/graydovee/todo-manager/internal/database"
	"github.com/graydovee/todo-manager/internal/repository"
	"github.com/graydovee/todo-manager/internal/service"
	"github.com/graydovee/todo-manager/internal/transport"
	"gorm.io/gorm/logger"
)

func main() {
	// -config: path to a YAML config file. Defaults to "config.yaml" for normal
	// deployments. The embedded desktop sidecar passes an empty path together
	// with TODO_MANAGER_SKIP_CONFIG=1 so it can boot from env vars alone, with
	// no YAML file shipped alongside the binary.
	configPath := flag.String("config", "config.yaml", "path to config file (empty + TODO_MANAGER_SKIP_CONFIG=1 = env-only boot)")
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

	sidecarMode := truthyEnv("TODO_MANAGER_SIDECAR")
	go func() {
		if sidecarMode {
			// Embedded mode: listen on a local socket (Windows named pipe or
			// Unix domain socket) instead of a TCP port. This avoids Windows
			// firewall prompts and network permissions entirely. The path is
			// printed to stdout as a single line so the spawning Tauri process
			// can discover and dial it.
			ln, addr, err := transport.ListenSidecar()
			if err != nil {
				slog.Error("failed to listen local socket", "error", err)
				os.Exit(1)
			}
			e.Listener = ln
			fmt.Println(sidecarReadyPrefix + addr)
			os.Stdout.Sync()
			slog.Info("starting sidecar server", "addr", addr)
			if err := e.Start(""); err != nil {
				slog.Info("sidecar server stopped", "error", err)
			}
			return
		}

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
	if err := e.Shutdown(context.Background()); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}

// sidecarReadyPrefix is written to stdout, immediately followed by the local
// socket address, once the sidecar listener is bound. The Tauri parent reads
// stdout lines and waits for this sentinel before dialing.
const sidecarReadyPrefix = "SIDECAR_READY "

func truthyEnv(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
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
