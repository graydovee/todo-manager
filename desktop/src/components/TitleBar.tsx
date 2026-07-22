import { useCallback } from "react";
import { useTranslation } from "react-i18next";
import { getCurrentWindow } from "@tauri-apps/api/window";
import {
  PlusIcon,
  RefreshIcon,
  PinIcon,
  LockIcon,
  UnlockIcon,
  SettingsIcon,
  CloseIcon,
} from "./ui/icons";

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
  const { t } = useTranslation();

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
        {t("common.appName")}
      </span>
      <div className="title-bar__buttons">
        {authenticated && (
          <>
            <button className="icon-btn" title={t("titlebar.new")} onClick={() => onCreate?.()}>
              <PlusIcon />
            </button>
            <button className="icon-btn" title={t("titlebar.refresh")} onClick={() => onRefresh?.()}>
              <RefreshIcon />
            </button>
          </>
        )}
        <button
          className={`icon-btn${pinned ? " icon-btn--active" : ""}`}
          title={t("titlebar.pin")}
          onClick={onTogglePin}
        >
          <PinIcon filled={pinned} />
        </button>
        <button
          className={`icon-btn${locked ? " icon-btn--active" : ""}`}
          title={t("titlebar.lock")}
          onClick={onToggleLock}
        >
          {locked ? <LockIcon /> : <UnlockIcon />}
        </button>
        {authenticated && (
          <button className="icon-btn" title={t("titlebar.manage")} onClick={() => onManage?.()}>
            <SettingsIcon />
          </button>
        )}
        <button className="icon-btn" title={t("titlebar.close")} onClick={handleClose}>
          <CloseIcon />
        </button>
      </div>
    </div>
  );
}
