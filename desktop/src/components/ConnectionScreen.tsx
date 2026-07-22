import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useAuth } from "../stores/authContext";
import { getBackendUrl, setBackendUrl, setApiKey } from "../api/config";
import { initClient, ping, checkAuth, resetClient } from "../api/client";

/**
 * ConnectionScreen — the desktop client's entry screen.
 *
 * Single-step: the user enters the backend URL + API key and clicks Connect.
 * The connection is verified via a two-step check (Ping /health → CheckAuth
 * /todos?page_size=1). On success, the auth context's `user` is set and App
 * switches to the todo list.
 */
export function ConnectionScreen() {
  const { t } = useTranslation();
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
        setError(t("connection.errInit"));
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
        setError(t("connection.errKey"));
      } else if (msg.includes("Network") || msg.includes("ECONNREFUSED")) {
        setError(t("connection.errUnreachable", { url }));
      } else {
        setError(t("connection.errFailed", { msg }));
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="connection-screen">
      <div className="connection-card">
        <h2>{t("connection.title")}</h2>
        <p className="connection-hint">{t("connection.subtitle")}</p>
        <input
          className="text-input"
          type="text"
          placeholder={t("connection.urlPlaceholder")}
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          autoFocus
        />
        <input
          className="text-input"
          type="password"
          placeholder={t("connection.keyPlaceholder")}
          value={apiKey}
          onChange={(e) => setApiKeyState(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && !loading && handleConnect()}
        />
        {error && <p className="connection-error">{error}</p>}
        <button
          className="btn btn--primary"
          onClick={handleConnect}
          disabled={loading || !url.trim() || !apiKey.trim()}
        >
          {loading ? t("connection.connecting") : t("connection.connect")}
        </button>
        <p className="connection-footnote">{t("connection.footnote")}</p>
      </div>
    </div>
  );
}
