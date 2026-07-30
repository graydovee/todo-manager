import {
  createContext,
  useContext,
  useState,
  useEffect,
  useCallback,
  type ReactNode,
} from "react";
import { initClient, checkAuth, resetClient, getBaseURL, initLocalClient } from "../api/client";
import { getApiKey, getMode } from "../api/config";
import { startLocalBackend } from "../api/localBackend";
import { keyHint, type AuthUser } from "../api/auth";

/**
 * Auth context for the desktop client.
 *
 * Two connection modes:
 * - "remote": connect to a user-configured backend server with an API key.
 * - "local":  run an embedded backend (sidecar) against a local SQLite database
 *             with no login required. Data is independent from any remote server.
 *
 * Startup behaviour:
 * - On launch, read the stored mode. If "local", start the sidecar and verify;
 *   if "remote" with stored URL+key, auto-connect; otherwise show the connection
 *   screen.
 */

interface AuthState {
  user: AuthUser | null;
  ready: boolean;
  clientReady: boolean;
  setClientReady: (v: boolean) => void;
  connect: () => Promise<void>;
  connectLocal: () => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [ready, setReady] = useState(false);
  const [clientReady, setClientReady] = useState(false);

  // On startup, auto-connect according to the stored mode.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      const mode = await getMode();
      if (mode === "local") {
        try {
          await startLocalBackend();
          await initLocalClient();
          if (cancelled) return;
          setClientReady(true);
          await checkAuth();
          if (cancelled) return;
          setUser({ backendUrl: "local", keyHint: "", isLocal: true });
        } catch {
          // Sidecar failed to start — fall back to the connection screen.
          if (cancelled) return;
          resetClient();
          setClientReady(false);
        } finally {
          if (!cancelled) setReady(true);
        }
        return;
      }

      // Remote mode: auto-connect with stored credentials.
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

  const connectLocal = useCallback(async () => {
    await startLocalBackend();
    await initLocalClient();
    setClientReady(true);
    await checkAuth();
    setUser({ backendUrl: "local", keyHint: "", isLocal: true });
  }, []);

  const logout = useCallback(async () => {
    // In local mode, keep the sidecar running (avoid restart churn); just drop
    // the user so the connection screen reappears. The sidecar is cleaned up on
    // app exit. Clearing the mode lets the user pick remote on the next screen.
    resetClient();
    setClientReady(false);
    setUser(null);
  }, []);

  return (
    <AuthContext.Provider
      value={{ user, ready, clientReady, setClientReady, connect, connectLocal, logout }}
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
