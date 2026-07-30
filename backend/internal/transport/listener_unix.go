//go:build !windows

package transport

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// listenSidecar binds a Unix domain socket with a random per-session name under
// the user's runtime/temp directory. UDS bypass the network stack, so no
// firewall prompt or network capability is involved.
func listenSidecar() (net.Listener, string, error) {
	dir := socketDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, "", fmt.Errorf("create socket dir: %w", err)
	}
	path := filepath.Join(dir, fmt.Sprintf("todo-manager-%s.sock", uuid.NewString()))

	// net.Listen("unix") fails if the path already exists, so remove a possible
	// stale socket first (ignore "not exist").
	_ = os.Remove(path)

	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, "", fmt.Errorf("listen unix socket: %w", err)
	}
	// Restrict to the current user.
	_ = os.Chmod(path, 0o600)
	return ln, path, nil
}

// socketDir picks a per-user runtime directory, preferring $XDG_RUNTIME_DIR
// (typically /run/user/<uid> on Linux) and falling back to the OS temp dir.
func socketDir() string {
	if v := os.Getenv("XDG_RUNTIME_DIR"); v != "" {
		return v
	}
	return os.TempDir()
}
