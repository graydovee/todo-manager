import { useCallback } from "react";
import { getCurrentWindow } from "@tauri-apps/api/window";

interface TitleBarProps {
  authenticated?: boolean;
  locked: boolean;
  pinned: boolean;
  onToggleLock: () => void;
  onTogglePin: () => void;
  onCreate?: () => void;
  onRefresh?: () => void;
  onManage?: () => void;
}

/**
 * TitleBar — custom window title bar. Lock/pin state and toggles are managed
 * by the parent (AppContent) so the tray menu and titlebar stay in sync.
 */
export function TitleBar({
  authenticated = false,
  locked,
  pinned,
  onToggleLock,
  onTogglePin,
  onCreate,
  onRefresh,
  onManage,
}: TitleBarProps) {
  const handleClose = useCallback(async () => {
    try {
      await getCurrentWindow().close();
    } catch {
      window.close();
    }
  }, []);

  return (
    <div className="title-bar" data-tauri-drag-region>
      <span className="title-bar__title" data-tauri-drag-region>
        Todos
      </span>
      <div className="title-bar__buttons">
        {authenticated && (
          <>
            <TitleBarButton title="New" onClick={() => onCreate?.()}>
              ＋
            </TitleBarButton>
            <TitleBarButton title="Refresh" onClick={() => onRefresh?.()}>
              ↻
            </TitleBarButton>
          </>
        )}
        <TitleBarButton title="Pin on top" active={pinned} onClick={onTogglePin}>
          {pinned ? "📌" : "📍"}
        </TitleBarButton>
        <TitleBarButton title="Lock" active={locked} onClick={onToggleLock}>
          {locked ? "🔒" : "🔓"}
        </TitleBarButton>
        {authenticated && (
          <TitleBarButton title="Manage" onClick={() => onManage?.()}>
            ⚙
          </TitleBarButton>
        )}
        <TitleBarButton title="Close" onClick={handleClose}>
          ✕
        </TitleBarButton>
      </div>
    </div>
  );
}

interface TitleBarButtonProps {
  children: React.ReactNode;
  title: string;
  active?: boolean;
  onClick: () => void;
}

function TitleBarButton({
  children,
  title,
  active = false,
  onClick,
}: TitleBarButtonProps) {
  return (
    <button
      className={`title-bar__btn${active ? " title-bar__btn--active" : ""}`}
      title={title}
      onClick={onClick}
    >
      {children}
    </button>
  );
}
