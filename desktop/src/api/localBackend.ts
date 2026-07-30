import { invoke } from "@tauri-apps/api/core";

/**
 * Bindings to the Rust sidecar lifecycle commands (see desktop/src-tauri/src/sidecar.rs).
 *
 * In local mode, the desktop app spawns the bundled Go backend as a sidecar
 * process that listens on a local socket (no TCP port, no firewall prompt).
 * These calls start/stop that process; once started, all data access goes
 * through the `local_http` command (wrapped by the `localInvokeAdapter` axios
 * adapter), so the rest of the API layer is unaware of the transport.
 */

/** Start the embedded backend. Returns the local socket address it listens on. */
export function startLocalBackend(): Promise<string> {
  return invoke<string>("start_local_backend");
}

/** Stop the embedded backend (kill the sidecar process). */
export function stopLocalBackend(): Promise<void> {
  return invoke<void>("stop_local_backend");
}
