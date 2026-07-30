//go:build windows

package transport

import (
	"fmt"
	"net"

	"github.com/Microsoft/go-winio"
	"github.com/google/uuid"
)

// listenSidecar binds a Windows named pipe with a random per-session name.
// Named pipes bypass the Winsock/network stack, so no firewall prompt is shown
// and no network capability is required — the desktop client (running as the
// same user) dials the same path.
func listenSidecar() (net.Listener, string, error) {
	path := fmt.Sprintf(`\\.\pipe\todo-manager-%s`, uuid.NewString())
	ln, err := winio.ListenPipe(path, &winio.PipeConfig{
		// MessageMode false => byte stream, which is what HTTP/1.1 expects.
		InputBufferSize:  1 << 20, // 1 MiB
		OutputBufferSize: 1 << 20,
	})
	if err != nil {
		return nil, "", fmt.Errorf("listen named pipe: %w", err)
	}
	return ln, path, nil
}
