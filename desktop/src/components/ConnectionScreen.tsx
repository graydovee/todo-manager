import { useState } from "react";
import { useAuth } from "../stores/authContext";
import { getBackendUrl, setBackendUrl, setApiKey } from "../api/config";
import { initClient, ping, checkAuth, resetClient } from "../api/client";

/**
 * ConnectionScreen — the desktop client's entry screen.
 *
 * Single-step: the user enters the backend URL + API key and clicks Connect.
 * The connection is verified via a two-step check (Ping /health → CheckAuth
 * /todos?page_size=1), matching the Fyne desktop client's flow. On success,
 * the auth context's `user` is set and App switches to the todo list.
 */
export function ConnectionScreen() {
  const { connect, setClientReady } = useAuth();
  const [url, setUrl] = useState("");
  const [apiKey, setApiKeyState] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  // On mount, pre-fill the URL if already configured.
  useState(() => {
    void (async () => {
      const existing = await getBackendUrl();
      if (existing) setUrl(existing);
    })();
  });

  const handleConnect = async () => {
    setError("");
    setLoading(true);
    try {
      // Step 1: persist the URL + key.
      await setBackendUrl(url);
      await setApiKey(apiKey);

      // Step 2: ping the backend (health check, no auth).
      await ping(url);

      // Step 3: initialise the client (sets baseURL + Bearer header) and verify.
      const ok = await initClient();
      if (!ok) {
        setError("Failed to initialise — check URL and API key");
        return;
      }
      await checkAuth();
      setClientReady(true);
      await connect();
    } catch (e) {
      resetClient();
      setClientReady(false);
      const msg = e instanceof Error ? e.message : String(e);
      if (msg.includes("401") || msg.toLowerCase().includes("unauthorized")) {
        setError("Invalid API key — please check and try again");
      } else if (msg.includes("Network") || msg.includes("ECONNREFUSED")) {
        setError(`Cannot reach backend at ${url}`);
      } else {
        setError(`Connection failed: ${msg}`);
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="connection-screen">
      <div className="connection-card">
        <h2>Connect to Backend</h2>
        <p className="connection-hint">
          Enter your todo manager backend URL and API key
        </p>
        <input
          className="connection-input"
          type="text"
          placeholder="https://todo.example.com"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          autoFocus
        />
        <input
          className="connection-input"
          type="password"
          placeholder="API Key (tdk_…)"
          value={apiKey}
          onChange={(e) => setApiKeyState(e.target.value)}
          onKeyDown={(e) =>
            e.key === "Enter" && !loading && handleConnect()
          }
        />
        {error && <p className="connection-error">{error}</p>}
        <button
          className="connection-btn"
          onClick={handleConnect}
          disabled={loading || !url.trim() || !apiKey.trim()}
        >
          {loading ? "Connecting…" : "Connect"}
        </button>
        <p className="connection-hint" style={{ marginTop: 4 }}>
          Generate an API key from the web app's Access Keys page.
        </p>
      </div>
    </div>
  );
}
