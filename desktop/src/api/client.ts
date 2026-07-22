import axios, { type AxiosInstance, type InternalAxiosRequestConfig } from "axios";
import { fetch as tauriFetch } from "@tauri-apps/plugin-http";
import { getBackendUrl, getApiKey } from "./config";
import { tauriHttpAdapter } from "./tauriAdapter";

/**
 * HTTP client for the desktop app.
 *
 * Authentication uses a Bearer API key (Authorization: Bearer <key>), matching
 * the old Fyne desktop client. The backend's AuthEither middleware accepts
 * Bearer tokens on every /todos/* and /summaries/* endpoint with no backend
 * changes.
 *
 * All requests are routed through Tauri's Rust HTTP plugin (via the custom
 * adapter) to bypass the WebView's CORS restrictions — the Tauri origin is not
 * in the backend's CORS allowlist, so plain axios/XHR would be blocked.
 */

/** The current API key, captured by initClient and attached to every request. */
let currentApiKey = "";

// The single axios instance used by all API modules.
// The adapter routes requests through Tauri's Rust HTTP to bypass CORS.
export const client: AxiosInstance = axios.create({
  timeout: 30_000,
  headers: { Accept: "application/json", "Content-Type": "application/json" },
  adapter: tauriHttpAdapter,
});

// Attach the Bearer token to every request.
client.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  if (currentApiKey) {
    config.headers.Authorization = `Bearer ${currentApiKey}`;
  }
  return config;
});

/**
 * Initialise the client with the configured backend URL + API key. Must be
 * called once at startup (before any API call). Returns false if no URL or key
 * is configured.
 */
export async function initClient(): Promise<boolean> {
  const backendUrl = await getBackendUrl();
  const apiKey = await getApiKey();
  if (!backendUrl || !apiKey) {
    return false;
  }
  client.defaults.baseURL = `${backendUrl}/api/v1`;
  currentApiKey = apiKey;
  return true;
}

/** Reset the client (e.g. after the user changes settings or logs out). */
export function resetClient(): void {
  client.defaults.baseURL = undefined;
  currentApiKey = "";
}

/** The base URL currently in use (for display). Empty if not initialised. */
export function getBaseURL(): string {
  return client.defaults.baseURL ?? "";
}

/**
 * Check that the backend is reachable (GET /health on the host root, no auth).
 * Uses tauriFetch directly (the axios client doesn't have a baseURL yet at this
 * point). Mirrors the Fyne client's Ping().
 */
export async function ping(backendUrl: string): Promise<void> {
  const res = await tauriFetch(`${normalizeBase(backendUrl)}/health`);
  if (!res.ok) {
    throw new Error(`Backend health check failed: ${res.status}`);
  }
}

/**
 * Verify that the API key is valid by making a lightweight authenticated
 * request. Mirrors the Fyne client's CheckAuth().
 */
export async function checkAuth(): Promise<void> {
  await client.get("/todos", { params: { page_size: 1 } });
}

function normalizeBase(url: string): string {
  return url.replace(/\/+$/, "");
}
