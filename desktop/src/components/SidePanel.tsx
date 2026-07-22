import { useState } from "react";
import type { TodoDetail } from "../types";
import { DetailPanel } from "./DetailPanel";
import { ManagePanel } from "./ManagePanel";
import { CreatePanel } from "./CreatePanel";

interface SidePanelProps {
  mode: "detail" | "manage" | "create";
  detail: TodoDetail | null;
  detailLoading: boolean;
  onClose: () => void;
  onTodoChanged: () => void;
  onLogout: () => void;
}

const TITLES: Record<SidePanelProps["mode"], string> = {
  detail: "Detail",
  manage: "Manage",
  create: "New Todo",
};

/**
 * SidePanel — the chrome wrapper for all side-panel modes.
 *
 * Renders a header (title + collapse chevron-left + Edit/Save button for
 * detail mode) and the mode-specific body. The collapse button calls onClose
 * which shrinks the window back to list-only width.
 */
export function SidePanel({
  mode,
  detail,
  detailLoading,
  onClose,
  onTodoChanged,
  onLogout,
}: SidePanelProps) {
  const [editing, setEditing] = useState(false);

  return (
    <div className="side-panel">
      <div className="side-panel__header">
        <span className="side-panel__title">{TITLES[mode]}</span>
        <div className="side-panel__actions">
          {mode === "detail" && (
            <button
              className="side-panel__btn"
              onClick={() => setEditing((e) => !e)}
            >
              {editing ? "Save" : "Edit"}
            </button>
          )}
          <button
            className="side-panel__btn side-panel__btn--icon"
            title="Collapse"
            onClick={onClose}
          >
            <svg
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <polyline points="15 18 9 12 15 6" />
            </svg>
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
