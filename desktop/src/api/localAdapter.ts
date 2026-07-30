import { invoke } from "@tauri-apps/api/core";
import type { AxiosResponse, InternalAxiosRequestConfig } from "axios";

/**
 * Axios adapter for local mode.
 *
 * Instead of making an HTTP request over the network, this adapter forwards
 * the request to the embedded backend sidecar over a local socket (Windows
 * named pipe / Unix domain socket) via the Tauri `local_http` command. The Rust
 * side dials the pipe, writes the HTTP/1.1 request, and returns the parsed
 * response.
 *
 * This keeps the entire `todos.ts` / `auth.ts` API layer (and every call site)
 * byte-for-byte identical to remote mode — only the transport differs.
 *
 * The baseURL in local mode is a bare "/api/v1"; the adapter strips it and
 * sends only the path + query to Rust (the pipe has no host).
 */
export async function localInvokeAdapter(
  config: InternalAxiosRequestConfig,
): Promise<AxiosResponse> {
  const baseURL = config.baseURL ?? "";
  const url = config.url ?? "";
  const path = buildPath(baseURL, url, config.params);

  const method = (config.method ?? "get").toUpperCase();

  // Collect headers (axios stores them in config.headers as an AxiosHeaders).
  const headers: Record<string, string> = {};
  if (config.headers) {
    const h = config.headers as unknown as Record<string, unknown>;
    for (const key of Object.keys(h)) {
      const val = h[key];
      if (typeof val === "string" || typeof val === "number") {
        headers[key] = String(val);
      }
    }
  }

  // Build body (string for JSON, same as tauriHttpAdapter).
  let body: string | undefined;
  if (method !== "GET" && method !== "HEAD" && config.data != null) {
    body = typeof config.data === "string" ? config.data : JSON.stringify(config.data);
  }

  const response = await invoke<{
    status: number;
    headers: Record<string, string>;
    body: string;
  }>("local_http", {
    request: { method, path, body: body ?? null, headers },
  });

  // Parse the body: try JSON when the backend says it's JSON, else keep text.
  const contentType = response.headers["content-type"] ?? "";
  let data: unknown;
  if (contentType.includes("application/json")) {
    try {
      data = JSON.parse(response.body);
    } catch {
      data = response.body;
    }
  } else {
    data = response.body;
  }

  return {
    data,
    status: response.status,
    statusText: "",
    headers: response.headers,
    config,
    request: {},
  };
}

/** Build the request path (path + query string), stripping the baseURL origin. */
function buildPath(
  baseURL: string,
  url: string,
  params?: Record<string, unknown>,
): string {
  // baseURL in local mode is "/api/v1"; in case a full URL leaked through,
  // strip any scheme+host.
  const base = baseURL.replace(/^https?:\/\/[^/]+/, "");
  let full = `${base}${url}`;
  if (params) {
    const searchParams = new URLSearchParams();
    for (const [key, value] of Object.entries(params)) {
      if (value == null) continue;
      if (Array.isArray(value)) {
        for (const v of value) {
          searchParams.append(key, String(v));
        }
      } else {
        searchParams.append(key, String(value));
      }
    }
    const qs = searchParams.toString();
    if (qs) {
      full += (full.includes("?") ? "&" : "?") + qs;
    }
  }
  return full;
}
