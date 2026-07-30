// Embedded backend sidecar management.
//
// When the user picks "local mode", the desktop app spawns the bundled Go
// backend binary as a sidecar. Instead of binding a TCP port (which would
// trigger the Windows firewall prompt and require a network capability), the
// sidecar listens on a local socket — a Windows named pipe or a Unix domain
// socket — and prints the socket path to stdout. The Tauri shell reads that
// path and forwards HTTP requests to the sidecar over the local socket via the
// `local_http` command, so the frontend's axios client keeps working unchanged
// (only the transport swaps from TCP to a local pipe).

use std::io::{Read, Write};
use std::sync::Mutex;

use interprocess::local_socket::prelude::*; // Stream (aka LocalSocketStream), connect + name traits
use serde::{Deserialize, Serialize};
use tauri::State;
use tauri_plugin_shell::process::CommandChild;
use tauri_plugin_shell::ShellExt;

/// The stdout sentinel the Go sidecar prints once its local socket is bound,
/// immediately followed by the socket address (named pipe path on Windows,
/// unix socket path on Unix).
const READY_PREFIX: &str = "SIDECAR_READY ";

/// Holds the running sidecar's socket address and process handle so that
/// `local_http` can dial it and `stop_local_backend` can terminate it.
pub struct SidecarState {
    addr: Mutex<Option<String>>,
    child: Mutex<Option<CommandChild>>,
}

impl SidecarState {
    pub fn new() -> Self {
        Self {
            addr: Mutex::new(None),
            child: Mutex::new(None),
        }
    }

    /// Kill the sidecar child process (if any). Called on app exit to avoid
    /// leaving an orphaned backend running.
    pub fn kill_child(&self) {
        if let Ok(mut guard) = self.child.lock() {
            if let Some(child) = guard.take() {
                let _ = child.kill();
            }
        }
    }
}

#[derive(Debug, Serialize)]
pub struct HttpError {
    pub error: String,
}

#[derive(Debug, Deserialize)]
pub struct LocalHttpRequest {
    pub method: String,
    /// Path + optional query string, e.g. "/api/v1/todos?page_size=5".
    pub path: String,
    #[serde(default)]
    pub body: Option<String>,
    /// Header name -> value map. Content-Length/Host/Connection are set here.
    #[serde(default)]
    pub headers: std::collections::HashMap<String, String>,
}

#[derive(Debug, Serialize)]
pub struct LocalHttpResponse {
    pub status: u16,
    /// Header name (lowercased) -> value.
    pub headers: std::collections::HashMap<String, String>,
    pub body: String,
}

/// Spawn the embedded backend sidecar. It boots the Go server in
/// auth.mode=none against a SQLite file in the app data dir, listening on a
/// local socket. Returns the socket address (so the caller can stash it), but
/// the address is also stored in SidecarState for use by `local_http`.
#[tauri::command]
pub async fn start_local_backend(
    app: tauri::AppHandle,
    state: State<'_, SidecarState>,
) -> Result<String, String> {
    // Refuse to spawn a second sidecar if one is already running.
    {
        let addr = state.addr.lock().map_err(|e| e.to_string())?;
        if addr.is_some() {
            return Ok(addr.clone().unwrap());
        }
    }

    // The SQLite database lives in a "data" subdirectory next to the main
    // executable (the install directory). This keeps the local data alongside
    // the program so it is easy to find/back up, and avoids the read-only
    // Program Files problem as long as the installer targets a writable
    // location (currentUser install mode → user's AppData\Local).
    let exe_dir = std::env::current_exe()
        .map_err(|e| format!("resolve current exe: {e}"))?
        .parent()
        .ok_or("cannot resolve exe parent dir")?
        .to_path_buf();
    let data_dir = exe_dir.join("data");
    std::fs::create_dir_all(&data_dir)
        .map_err(|e| format!("create data dir: {e}"))?;
    let db_path = data_dir.join("todo-local.db");

    let sidecar = app
        .shell()
        .sidecar("todo-manager-sidecar")
        .map_err(|e| format!("lookup sidecar: {e}"))?;

    // Start with a clean environment so the sidecar isn't affected by the
    // launcher's env, then set the few variables it needs.
    let (mut rx, child) = sidecar
        .env_clear()
        .env("TODO_MANAGER_SKIP_CONFIG", "1")
        .env("TODO_MANAGER_AUTH_MODE", "none")
        .env("TODO_MANAGER_SIDECAR", "1")
        .env("TODO_MANAGER_DB_DSN", db_path.to_string_lossy().to_string())
        .spawn()
        .map_err(|e| format!("spawn sidecar: {e}"))?;

    // Wait for the "SIDECAR_READY <addr>" line on stdout (the sidecar also
    // emits migration/log noise, so scan until we find the sentinel).
    use tauri_plugin_shell::process::CommandEvent;
    let mut resolved_addr: Option<String> = None;
    while let Some(event) = rx.recv().await {
        match event {
            CommandEvent::Stdout(bytes) | CommandEvent::Stderr(bytes) => {
                let text = String::from_utf8_lossy(&bytes);
                for line in text.lines() {
                    let line = line.trim();
                    if let Some(rest) = line.strip_prefix(READY_PREFIX) {
                        resolved_addr = Some(rest.trim().to_string());
                        break;
                    }
                }
                if resolved_addr.is_some() {
                    break;
                }
            }
            CommandEvent::Error(err) => {
                return Err(format!("sidecar error: {}", err));
            }
            CommandEvent::Terminated(payload) => {
                return Err(format!("sidecar exited prematurely: {:?}", payload));
            }
            _ => {}
        }
    }

    let addr = resolved_addr.ok_or_else(|| "sidecar did not report a ready address".to_string())?;

    *state.addr.lock().map_err(|e| e.to_string())? = Some(addr.clone());
    *state.child.lock().map_err(|e| e.to_string())? = Some(child);

    Ok(addr)
}

/// Stop the embedded backend sidecar (if running). Called on app exit and when
/// the user switches away from local mode.
#[tauri::command]
pub fn stop_local_backend(state: State<'_, SidecarState>) -> Result<(), String> {
    if let Some(child) = state.child.lock().map_err(|e| e.to_string())?.take() {
        let _ = child.kill();
    }
    *state.addr.lock().map_err(|e| e.to_string())? = None;
    Ok(())
}

/// Forward an HTTP/1.1 request to the sidecar over the local socket and return
/// the parsed response. This lets the frontend keep its axios-based API layer
/// unchanged — only the transport is swapped from TCP to a local pipe.
#[tauri::command]
pub fn local_http(
    state: State<'_, SidecarState>,
    request: LocalHttpRequest,
) -> Result<LocalHttpResponse, HttpError> {
    let addr = {
        let guard = state.addr.lock().map_err(|e| HttpError {
            error: e.to_string(),
        })?;
        guard
            .clone()
            .ok_or_else(|| HttpError { error: "local backend is not running".into() })?
    };

    // Build the HTTP/1.1 request bytes.
    let body = request.body.unwrap_or_default();
    let mut req = format!(
        "{} {} HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\nContent-Length: {}\r\n",
        request.method.to_uppercase(),
        request.path,
        body.len()
    );
    for (k, v) in &request.headers {
        // Skip hop-by-hop / content-length headers we already set.
        let lk = k.to_lowercase();
        if lk == "content-length" || lk == "host" || lk == "connection" {
            continue;
        }
        req.push_str(k);
        req.push_str(": ");
        req.push_str(v);
        req.push_str("\r\n");
    }
    req.push_str("\r\n");
    req.push_str(&body);

    let mut conn = {
        // Build a local-socket Name from the address: named pipes (Windows) use
        // the namespaced form, unix socket paths use the filesystem form.
        let name = make_socket_name(&addr)?;
        LocalSocketStream::connect(name).map_err(|e| HttpError {
            error: format!("connect to sidecar: {e}"),
        })?
    };
    conn.write_all(req.as_bytes()).map_err(|e| HttpError {
        error: format!("write request: {e}"),
    })?;
    conn.flush().map_err(|e| HttpError {
        error: format!("flush request: {e}"),
    })?;

    // Read the full response. Connection: close means the server closes after
    // sending, so we can read until EOF for the body.
    let mut raw = Vec::new();
    conn.read_to_end(&mut raw).map_err(|e| HttpError {
        error: format!("read response: {e}"),
    })?;

    parse_http_response(&raw)
}

/// Parse a raw HTTP/1.1 response buffer into status line + headers + body.
fn parse_http_response(raw: &[u8]) -> Result<LocalHttpResponse, HttpError> {
    // Split headers from body at the first blank line.
    let sep = b"\r\n\r\n";
    let header_end = windows_find(raw, sep).ok_or_else(|| HttpError {
        error: "malformed HTTP response: no header/body separator".into(),
    })?;
    let header_bytes = &raw[..header_end];
    let body_bytes = &raw[header_end + sep.len()..];

    let header_str = String::from_utf8_lossy(header_bytes);
    let mut lines = header_str.lines();

    let status_line = lines.next().ok_or_else(|| HttpError {
        error: "malformed HTTP response: missing status line".into(),
    })?;
    // "HTTP/1.1 200 OK"
    let mut parts = status_line.splitn(3, ' ');
    let _version = parts.next();
    let status: u16 = parts
        .next()
        .ok_or_else(|| HttpError { error: "missing status code".into() })?
        .parse()
        .map_err(|e| HttpError { error: format!("bad status code: {e}") })?;

    let mut headers = std::collections::HashMap::new();
    for line in lines {
        if let Some((k, v)) = line.split_once(':') {
            headers.insert(k.trim().to_lowercase(), v.trim().to_string());
        }
    }

    Ok(LocalHttpResponse {
        status,
        headers,
        body: String::from_utf8_lossy(body_bytes).to_string(),
    })
}

/// Find a subsequence within a byte slice, returning the starting index.
fn windows_find(haystack: &[u8], needle: &[u8]) -> Option<usize> {
    haystack
        .windows(needle.len())
        .position(|w| w == needle)
}

/// Convert a raw socket address string into an interprocess `Name`.
///
/// Windows named pipes (`\\.\pipe\…`) live in the namespaced namespace, while
/// Unix domain sockets are filesystem paths — the conversion traits differ per
/// platform.
fn make_socket_name(addr: &str) -> Result<interprocess::local_socket::Name<'_>, HttpError> {
    #[cfg(windows)]
    {
        use interprocess::local_socket::{GenericNamespaced, ToNsName as _};
        addr.to_ns_name::<GenericNamespaced>()
            .map_err(|e| HttpError {
                error: format!("invalid socket name: {e}"),
            })
    }
    #[cfg(not(windows))]
    {
        use interprocess::local_socket::{GenericFilePath, ToFsName as _};
        addr.to_fs_name::<GenericFilePath>()
            .map_err(|e| HttpError {
                error: format!("invalid socket name: {e}"),
            })
    }
}
