package ui

import (
	"github.com/graydovee/todo-manager/desktop/internal/config"
)

// writeConfig persists the config to disk. It is a thin wrapper so the UI can
// flush state without importing config explicitly everywhere.
func writeConfig(homeDir string, cfg *config.Config) error {
	return config.Write(homeDir, cfg)
}
