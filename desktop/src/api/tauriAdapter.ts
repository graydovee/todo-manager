import { fetch as tauriFetch } from "@tauri-apps/plugin-http";
import type { InternalAxiosRequestConfig, AxiosResponse } from "axios";

/**
 * Custom axios adapter that routes all requests through Tauri's Rust HTTP plugin
 * (reqwest under the hood), bypassing the WebView's CORS restrictions entirely.
 *
 * Without this, axios would use the WebView's XHR/fetch — which enforces CORS.
 * The Tauri origin (`tauri://localhost` / `https://tauri.localhost`) is not in
 * the backend's CORS allowlist, so cross-origin requests fail. By going through
 * Rust, we sidestep the browser's same-origin policy.
 *
 * The adapter translates an axios request into a Tauri fetch call and wraps the
 * Response back into the axios response shape.
 */
export async function tauriHttpAdapter(
  config: InternalAxiosRequestConfig,
): Promise<AxiosResponse> {
  // Build the full URL.
  const baseURL = config.baseURL ?? "";
  const url = config.url ?? "";
  const fullUrl = buildUrl(baseURL, url, config.params);

  // Determine method.
  const method = (config.method ?? "get").toUpperCase();

  // Build headers (axios stores them in config.headers as an AxiosHeaders).
  const headers: Record<string, string> = {};
  if (config.headers) {
    // AxiosHeaders supports iteration and direct access.
    const h = config.headers as unknown as Record<string, unknown>;
    for (const key of Object.keys(h)) {
      const val = h[key];
      if (typeof val === "string" || typeof val === "number") {
        headers[key] = String(val);
      }
    }
  }

  // Build body.
  let body: BodyInit | undefined;
  if (method !== "GET" && method !== "HEAD" && config.data != null) {
    if (typeof config.data === "string") {
      body = config.data;
    } else {
      body = JSON.stringify(config.data);
    }
  }

  // Make the request through Rust (no signal — desktop requests are short-lived).
  const response = await tauriFetch(fullUrl, {
    method,
    headers,
    body,
  });

  // Read the response body.
  const responseHeaders: Record<string, string> = {};
  response.headers.forEach((value, key) => {
    responseHeaders[key] = value;
  });

  const contentType = response.headers.get("content-type") ?? "";
  let data: unknown;
  if (contentType.includes("application/json")) {
    data = await response.json();
  } else {
    data = await response.text();
  }

  return {
    data,
    status: response.status,
    statusText: response.statusText,
    headers: responseHeaders,
    config,
    request: {},
  };
}

/** Build a URL with query params, mirroring axios's behaviour. */
function buildUrl(
  baseURL: string,
  url: string,
  params?: Record<string, unknown>,
): string {
  let full = `${baseURL}${url}`;
  if (params) {
    const searchParams = new URLSearchParams();
    for (const [key, value] of Object.entries(params)) {
      if (value == null) continue;
      if (Array.isArray(value)) {
        // axios serializes arrays as repeated params (key=a&key=b). For our
        // backend (comma-separated), the caller should pre-join into a string.
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
