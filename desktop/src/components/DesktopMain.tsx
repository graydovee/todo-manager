import { useState, useCallback, useEffect, forwardRef, useImperativeHandle } from "react";
import { getCurrentWindow, LogicalSize } from "@tauri-apps/api/window";
import { TodoList } from "./TodoList";
import { SidePanel } from "./SidePanel";
import type { Todo, TodoDetail } from "../types";
import * as todoApi from "../api/todos";

export interface DesktopMainHandle {
  openCreate: () => void;
  openManage: () => void;
  refreshList: () => void;
}

/**
 * DesktopMain — the core desktop layout: a fixed 320px todo list on the left
 * and an optional side panel (detail / manage / create) on the right.
 *
 * Opening a side panel WIDENS the window from 320px to 720px (the list column
 * never resizes). Closing it shrinks back. The top-left corner stays pinned
 * so the window grows to the right — matching the Fyne desktop client.
 */

type SideMode = "none" | "detail" | "manage" | "create";

const LIST_WIDTH = 320;
const SIDE_WIDTH = 400;
const FULL_WIDTH = LIST_WIDTH + SIDE_WIDTH;

export const DesktopMain = forwardRef<DesktopMainHandle>(function DesktopMain(_, ref) {
  const [sideMode, setSideMode] = useState<SideMode>("none");
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);

  // Detail data for the side panel.
  const [detail, setDetail] = useState<TodoDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const sideVisible = sideMode !== "none";

  // Widen / shrink the window when the side panel opens / closes.
  useEffect(() => {
    void (async () => {
      const win = getCurrentWindow();
      // Keep the top-left corner fixed: capture position before resize.
      const pos = await win.outerPosition();
      const size = await win.outerSize();
      const scaleFactor = await win.scaleFactor();
      const width = sideVisible ? FULL_WIDTH : LIST_WIDTH;
      const height = size.height / scaleFactor;
      await win.setSize(new LogicalSize(width, height));
      // Restore position (setSize may shift the window on some platforms).
      await win.setPosition(pos);
    })();
  }, [sideVisible]);

  const refreshList = useCallback(() => {
    setRefreshKey((k) => k + 1);
  }, []);

  const openDetail = useCallback(
    async (todo: Todo) => {
      setSelectedId(todo.id);
      setSideMode("detail");
      setDetailLoading(true);
      setDetail(null);
      try {
        const d = await todoApi.getTodo(todo.id);
        setDetail(d);
      } catch {
        // Error is non-fatal; the panel shows a loading/error state.
      } finally {
        setDetailLoading(false);
      }
    },
    [],
  );

  const openManage = useCallback(() => {
    setSideMode("manage");
  }, []);

  const openCreate = useCallback(() => {
    setSideMode("create");
  }, []);

  const closeSidePanel = useCallback(() => {
    setSideMode("none");
    setSelectedId(null);
    setDetail(null);
    refreshList();
  }, [refreshList]);

  // Called after a todo action (start/complete/reopen/edit/create) to refresh
  // both the list and the open detail.
  const onTodoChanged = useCallback(() => {
    refreshList();
    if (sideMode === "detail" && selectedId) {
      void (async () => {
        try {
          const d = await todoApi.getTodo(selectedId);
          setDetail(d);
        } catch {
          // ignore
        }
      })();
    }
  }, [refreshList, sideMode, selectedId]);

  // Expose imperative actions for the TitleBar buttons.
  useImperativeHandle(ref, () => ({
    openCreate,
    openManage,
    refreshList,
  }), [openCreate, openManage, refreshList]);

  return (
    <div className="desktop-main">
      <div className="desktop-list-col">
        <TodoList
          refreshKey={refreshKey}
          selectedId={selectedId}
          onOpenDetail={openDetail}
          onOpenManage={openManage}
          onOpenCreate={openCreate}
          onTodoChanged={onTodoChanged}
        />
      </div>
      {sideVisible && (
        <div className="desktop-side-col">
          <SidePanel
            mode={sideMode}
            detail={detail}
            detailLoading={detailLoading}
            onClose={closeSidePanel}
            onTodoChanged={onTodoChanged}
            onLogout={() => {
              closeSidePanel();
            }}
          />
        </div>
      )}
    </div>
  );
});
