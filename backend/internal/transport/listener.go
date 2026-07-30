// Package transport provides a cross-platform local socket listener used when
// the backend runs as an embedded Tauri sidecar. Instead of binding a TCP port
// (which triggers Windows firewall prompts and requires network permissions),
// the sidecar listens on a Windows named pipe or a Unix domain socket and the
// desktop client connects over that local channel.
//
// The listener is a plain net.Listener, so Echo (and net/http) serve ordinary
// HTTP/1.1 over it unchanged — only the transport differs.
package transport

import "net"

// ListenSidecar binds a per-session local socket at a random path and returns
// the listener together with the address a client should dial. The concrete
// transport (Windows named pipe vs. Unix domain socket) is selected by build
// tags in listener_windows.go / listener_unix.go.
func ListenSidecar() (net.Listener, string, error) {
	return listenSidecar()
}

// SocketHost is the sentinel host used as the HTTP "Host" header value for
// requests travelling over the local socket. It carries no routing meaning —
// the Go HTTP server ignores it — but a syntactically valid host is required by
// the HTTP/1.1 spec and by some client HTTP parsers.
const SocketHost = "localhost"
