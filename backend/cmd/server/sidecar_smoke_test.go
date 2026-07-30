package main

import (
	"bufio"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
)

// TestSidecarNamedPipe builds the server binary, runs it in sidecar mode, and
// verifies that an HTTP request flows over the Windows named pipe with no TCP
// port involved.
func TestSidecarNamedPipe(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping sidecar smoke test in short mode")
	}

	// Build the server binary into a temp dir.
	binPath := filepath.Join(t.TempDir(), "sidecar-test.exe")
	build := exec.Command("go", "build", "-o", binPath, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build server: %v\n%s", err, out)
	}

	cmd := exec.Command(binPath, "-config", "")
	cmd.Env = append(os.Environ(),
		"TODO_MANAGER_SKIP_CONFIG=1",
		"TODO_MANAGER_AUTH_MODE=none",
		"TODO_MANAGER_SIDECAR=1",
		"TODO_MANAGER_DB_DSN="+filepath.Join(t.TempDir(), "sidecar-test.db"),
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()

	r := bufio.NewReader(stdout)
	var pipePath string
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("reading stdout: %v", err)
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "SIDECAR_READY ") {
			pipePath = strings.TrimPrefix(line, "SIDECAR_READY ")
			break
		}
	}
	if pipePath == "" {
		t.Fatal("did not receive SIDECAR_READY")
	}
	t.Logf("pipe path: %s", pipePath)

	// Give the HTTP server a brief moment to start serving on the pipe.
	time.Sleep(500 * time.Millisecond)

	conn, err := winio.DialPipe(pipePath, nil)
	if err != nil {
		t.Fatalf("dial pipe: %v", err)
	}
	defer conn.Close()

	req, _ := http.NewRequest("GET", "/health", nil)
	req.Host = "localhost"
	if err := req.Write(conn); err != nil {
		t.Fatalf("write request: %v", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !strings.Contains(string(body), `"status":"ok"`) {
		t.Fatalf("unexpected health response: %d %s", resp.StatusCode, body)
	}
}
