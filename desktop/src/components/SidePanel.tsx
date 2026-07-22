import { useState } from "react";
import { useTranslation } from "react-i18next";
import type { TodoDetail } from "../types";
import { DetailPanel } from "./DetailPanel";
import { ManagePanel } from "./ManagePanel";
import { CreatePanel } from "./CreatePanel";
import { ChevronLeftIcon } from "./ui/icons";

interface SidePanelProps {
  mode: "detail" | "manage" | "create";
  detail: TodoDetail | null;
  detailLoading: boolean;
  onClose: () => void;
  onTodoChanged: () => void;
  onLogout: () => void;
}

/**
 * SidePanel — the chrome wrapper for all side-panel modes.
 *
 * Renders a header (title + collapse button + Edit/Save toggle for detail
 * mode) and the mode-specific body.
 */
export function SidePanel({
  mode,
  detail,
  detailLoading,
  onClose,
  onTodoChanged,
  onLogout,
}: SidePanelProps) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState(false);

  const titles: Record<SidePanelProps["mode"], string> = {
    detail: t("detail.title"),
    manage: t("manage.title"),
    create: t("create.title"),
  };

  return (
    <div className="side-panel">
      <div className="side-panel__header">
        <span className="side-panel__title">{titles[mode]}</span>
        <div className="side-panel__actions">
          {mode === "detail" && (
            <button className="btn btn--ghost btn--sm" onClick={() => setEditing((e) => !e)}>
              {editing ? t("detail.save") : t("detail.edit")}
            </button>
          )}
          <button className="icon-btn" title={t("detail.collapse")} onClick={onClose}>
            <ChevronLeftIcon />
          </button>
        </div>
      </div>
      <div className="side-panel__body">
        {mode === "detail" && (
          <DetailPanel
            detail={detail}
            loading={detailLoading}
            editing={editing}
            onEditingChange={setEditing}
            onTodoChanged={onTodoChanged}
          />
        )}
        {mode === "manage" && <ManagePanel onLogout={onLogout} />}
        {mode === "create" && <CreatePanel onTodoChanged={onTodoChanged} />}
      </div>
    </div>
  );
}
