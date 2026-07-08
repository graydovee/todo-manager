package ui

import (
	"os"

	"github.com/graydovee/todo-manager/desktop/internal/config"
)

// homeDir caches the user home directory for config persistence.
var homeDir string

// SetHomeDir records the home directory used by homePersist.
func SetHomeDir(dir string) { homeDir = dir }

// homePersist writes the config to disk, best-effort. Failures are swallowed;
// callers that need to report them should use Persist explicitly.
func homePersist(cfg *config.Config) {
	dir := homeDir
	if dir == "" {
		var err error
		dir, err = os.UserHomeDir()
		if err != nil {
			return
		}
	}
	_ = config.Write(dir, cfg)
}
