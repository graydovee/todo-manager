import { LazyStore } from "@tauri-apps/plugin-store";

/**
 * Desktop client configuration, persisted via the Tauri Store plugin.
 *
 * The backend URL is user-configurable so the desktop client can connect to any
 * remote deployment (or, in the future, a locally-embedded backend). The value
 * is normalised to a bare origin (no trailing slash, no /api/v1 suffix) — the
 * API client appends `/api/v1` itself.
 */

const STORE_FILE = "desktop-config.json";
const KEY_BACKEND_URL = "backendUrl";
const KEY_API_KEY = "apiKey";

let storePromise: Promise<LazyStore> | null = null;

function store(): Promise<LazyStore> {
  if (!storePromise) {
    storePromise = Promise.resolve(new LazyStore(STORE_FILE));
  }
  return storePromise;
}

/**
 * Get the configured backend base URL (e.g. "https://todo.qaer.io").
 * Returns the empty string if not yet configured.
 */
export async function getBackendUrl(): Promise<string> {
  const s = await store();
  return normalizeOrigin((await s.get<string>(KEY_BACKEND_URL)) ?? "");
}

/**
 * Persist the backend base URL. The value is normalised before storage.
 */
export async function setBackendUrl(url: string): Promise<void> {
  const s = await store();
  await s.set(KEY_BACKEND_URL, normalizeOrigin(url));
  await s.save();
}

/**
 * Get the stored API key (Bearer token). Returns the empty string if not set.
 */
export async function getApiKey(): Promise<string> {
  const s = await store();
  return (await s.get<string>(KEY_API_KEY)) ?? "";
}

/**
 * Persist the API key.
 */
export async function setApiKey(key: string): Promise<void> {
  const s = await store();
  await s.set(KEY_API_KEY, key);
  await s.save();
}

/** Clear the stored API key (logout). */
export async function clearApiKey(): Promise<void> {
  const s = await store();
  await s.delete(KEY_API_KEY);
  await s.save();
}

/**
 * Normalise a user-entered URL to a bare origin:
 *   - ensure it has a scheme (default https)
 *   - strip any trailing slash
 *   - strip an optional /api/v1 suffix
 * The API client will append "/api/v1" itself.
 */
export function normalizeOrigin(input: string): string {
  let v = input.trim();
  if (!v) return "";
  if (!/^https?:\/\//i.test(v)) {
    v = "https://" + v;
  }
  // Strip /api/v1 suffix if the user pasted a full API URL.
  v = v.replace(/\/api\/v\d+\/?$/i, "");
  // Strip trailing slash.
  v = v.replace(/\/+$/, "");
  return v;
}
