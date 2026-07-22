/**
 * Authentication module for the desktop client (API Key mode).
 *
 * The desktop app authenticates with a Bearer API key — the user generates a
 * key via the web app's Access Keys page and pastes it into the desktop
 * connection screen. The backend's AuthEither middleware accepts Bearer tokens
 * on every data endpoint.
 *
 * OIDC/SSO login (via a child WebView window) is planned for a future phase.
 */

export interface AuthUser {
  /** The backend base URL the user is connected to. */
  backendUrl: string;
  /** A masked prefix of the API key, for display. */
  keyHint: string;
}

/**
 * Build a display-safe hint from an API key (e.g. "tdk_abc…xyz").
 */
export function keyHint(key: string): string {
  if (key.length <= 8) return "…";
  return `${key.slice(0, 7)}…${key.slice(-3)}`;
}
