import {
  createContext,
  useContext,
  useState,
  useEffect,
  useCallback,
  type ReactNode,
} from "react";
import { initClient, checkAuth, resetClient, getBaseURL } from "../api/client";
import { getApiKey, clearApiKey } from "../api/config";
import { keyHint, type AuthUser } from "../api/auth";

/**
 * Auth context for the desktop client (API Key mode).
 *
 * Startup behaviour (matches the Fyne desktop client):
 * - If a backend URL + API key are stored, automatically verify the connection
 *   (initClient + checkAuth). On success, go straight to the todo list.
 * - If no credentials are stored, or the stored key is invalid, show the
 *   connection screen.
 *
 * The `ready` flag is false during the initial auto-connect attempt so the app
 * can show a brief loading state instead of flashing the connection screen.
 */

interface AuthState {
  user: AuthUser | null;
  ready: boolean;
  clientReady: boolean;
  setClientReady: (v: boolean) => void;
  connect: () => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [ready, setReady] = useState(false);
  const [clientReady, setClientReady] = useState(false);

  // On startup, attempt auto-connect with stored credentials.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      const ok = await initClient();
      if (!ok) {
        // No stored URL or key — show connection screen.
        if (!cancelled) setReady(true);
        return;
      }
      if (!cancelled) setClientReady(true);
      try {
        await checkAuth();
        if (cancelled) return;
        const url = getBackendUrlSync();
        const key = await getApiKey();
        setUser({ backendUrl: url, keyHint: keyHint(key) });
      } catch {
        // Stored key is invalid — fall back to connection screen.
        if (cancelled) return;
        resetClient();
        setClientReady(false);
      } finally {
        if (!cancelled) setReady(true);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const connect = useCallback(async () => {
    await checkAuth();
    const url = getBackendUrlSync();
    const key = await getApiKey();
    setUser({ backendUrl: url, keyHint: keyHint(key) });
  }, []);

  const logout = useCallback(async () => {
    await clearApiKey();
    resetClient();
    setClientReady(false);
    setUser(null);
  }, []);

  return (
    <AuthContext.Provider
      value={{ user, ready, clientReady, setClientReady, connect, logout }}
    >
      {children}
    </AuthContext.Provider>
  );
}

function getBackendUrlSync(): string {
  return getBaseURL().replace(/\/api\/v\d+\/?$/, "");
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
