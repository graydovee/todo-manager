import { useState, useRef, useEffect, useCallback } from "react";
import { useTranslation } from "react-i18next";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { AuthProvider, useAuth } from "./stores/authContext";
import { TitleBar } from "./components/TitleBar";
import { ConnectionScreen } from "./components/ConnectionScreen";
import { DesktopMain } from "./components/DesktopMain";
import { applyHotkey, currentHotkey, onUnlock } from "./utils/hotkey";
import { DEFAULT_LOCK_OPACITY } from "./utils/settings";

export default function App() {
  return (
    <AuthProvider>
      <AppContent />
    </AuthProvider>
  );
}

interface DesktopMainHandle {
  openCreate: () => void;
  openManage: () => void;
  refreshList: () => void;
}

function AppContent() {
  const { t } = useTranslation();
  const { user, ready, clientReady } = useAuth();
  const authenticated = clientReady && ready && !!user;
  const mainHandle = useRef<DesktopMainHandle | null>(null);

  // Window mode state — managed here so the titlebar and tray stay in sync.
  const [locked, setLocked] = useState(false);
  const [pinned, setPinned] = useState(false);

  // Toggle lock: click-through + semi-transparent + force topmost.
  const toggleLock = useCallback(async () => {
    const next = !locked;
    setLocked(next);
    try {
      await invoke("set_lock", { locked: next });
      // Lock implies always-on-top.
      if (next) setPinned(true);
      await invoke("sync_tray_state", { locked: next, topmost: next || pinned });
    } catch (e) {
      console.error("set_lock failed:", e);
    }
  }, [locked, pinned]);

  // Toggle pin (always-on-top). Unlocking topmost also unlocks.
  const togglePin = useCallback(async () => {
    const next = !pinned;
    setPinned(next);
    try {
      await invoke("set_always_on_top", { top: next });
      if (!next && locked) {
        setLocked(false);
        // Also clear the OS-level click-through, or the window stays
        // unresponsive even though the UI looks unlocked.
        await invoke("set_lock", { locked: false });
      }
      await invoke("sync_tray_state", { locked: locked && next, topmost: next });
    } catch (e) {
      console.error("set_always_on_top failed:", e);
    }
  }, [pinned, locked]);

  // Listen for tray menu events — toggle the same handlers.
  useEffect(() => {
    const unlisteners: (() => void)[] = [];
    listen("tray-toggle-lock", () => void toggleLock()).then((u) => unlisteners.push(u));
    listen("tray-toggle-pin", () => void togglePin()).then((u) => unlisteners.push(u));
    return () => unlisteners.forEach((u) => u());
  }, [toggleLock, togglePin]);

  // Refs so the OS-global hotkey callback always sees fresh state without
  // re-registering the shortcut on every lock/unlock.
  const lockedRef = useRef(locked);
  lockedRef.current = locked;
  const toggleLockRef = useRef(toggleLock);
  toggleLockRef.current = toggleLock;

  // Register the global unlock hotkey (default Alt+Shift+F11). Unlock-only:
  // pressing it while already unlocked does nothing.
  useEffect(() => {
    onUnlock(() => {
      if (lockedRef.current) void toggleLockRef.current();
    });
    void applyHotkey(currentHotkey()).catch((e) =>
      console.error("unlock hotkey registration failed:", e)
    );
  }, []);

  // Apply semi-transparent background when locked — toggle a class on #root
  // so all child elements (title-bar, panels, rows) become semi-transparent
  // via CSS and the user can see through the window to the desktop.
  useEffect(() => {
    const root = document.getElementById("root");
    if (root) {
      root.classList.toggle("app-locked", locked);
    }
  }, [locked]);

  // Lock opacity: read the persisted value on startup and follow changes
  // from the manage panel (custom event keeps the two in sync).
  useEffect(() => {
    const apply = () => {
      const v = parseFloat(localStorage.getItem("lock_opacity") ?? String(DEFAULT_LOCK_OPACITY));
      const clamped = isNaN(v) ? DEFAULT_LOCK_OPACITY : Math.min(1, Math.max(0.2, v));
      document.documentElement.style.setProperty("--lock-opacity", String(clamped));
    };
    apply();
    window.addEventListener("lock-opacity-changed", apply);
    return () => window.removeEventListener("lock-opacity-changed", apply);
  }, []);

  return (
    <>
      <TitleBar
        authenticated={authenticated}
        locked={locked}
        pinned={pinned}
        onToggleLock={() => void toggleLock()}
        onTogglePin={() => void togglePin()}
        onCreate={() => mainHandle.current?.openCreate()}
        onRefresh={() => mainHandle.current?.refreshList()}
        onManage={() => mainHandle.current?.openManage()}
      />
      <div className="app-content">
        {!ready ? (
          <div className="loading-screen">{t("common.loading")}</div>
        ) : authenticated ? (
          <DesktopMain ref={mainHandle} />
        ) : (
          <ConnectionScreen />
        )}
      </div>
    </>
  );
}
