import { useState, useEffect, useCallback } from "react";
import { useTranslation } from "react-i18next";
import * as todoApi from "../api/todos";
import { formatDisplayCode } from "../utils/displayCode";
import { statusLabel, PRIORITY_COLORS } from "../utils/enumOptions";
import { PlayIcon, CheckIcon, PinIcon } from "./ui/icons";
import type { Todo } from "../types";

interface TodoListProps {
  refreshKey: number;
  selectedId: number | null;
  onOpenDetail: (todo: Todo) => void;
  onOpenManage: () => void;
  onOpenCreate: () => void;
  onTodoChanged: () => void;
}

/**
 * TodoList — the fixed-width (320px) todo list column.
 *
 * Each row: priority dot + title on top, code + category/status badges below.
 * A contextual action button (start / complete) appears on hover.
 */
export function TodoList({
  refreshKey,
  selectedId,
  onOpenDetail,
  onTodoChanged,
}: TodoListProps) {
  const { t } = useTranslation();
  const [items, setItems] = useState<Todo[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const fetchTodos = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const filters: Record<string, unknown> = {
        page: 1,
        page_size: 100,
        sort_by: "created_at",
        sort_order: "desc",
      };
      // Read filter selections from localStorage (set by ManagePanel).
      const status = loadFilter("filter_status");
      if (status) filters.status = status;
      const category = loadFilter("filter_category");
      if (category) filters.category = category;
      const priority = loadFilter("filter_priority");
      if (priority) filters.priority = priority;
      const q = localStorage.getItem("filter_query")?.trim();
      if (q) filters.q = q;
      const code = localStorage.getItem("filter_code")?.trim();
      if (code) filters.code = code;

      const res = await todoApi.listTodos(filters);
      setItems(res.items ?? []);
      setTotal(res.total ?? 0);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void fetchTodos();
  }, [fetchTodos, refreshKey]);

  // Listen for filter changes from ManagePanel.
  useEffect(() => {
    const handler = () => void fetchTodos();
    window.addEventListener("todo-filter-changed", handler);
    return () => window.removeEventListener("todo-filter-changed", handler);
  }, [fetchTodos]);

  const doAction = useCallback(
    async (todo: Todo, action: "start" | "complete") => {
      try {
        if (action === "start") {
          await todoApi.startTodo(todo.id);
        } else {
          await todoApi.completeTodo(todo.id, false);
        }
        onTodoChanged();
      } catch (e) {
        console.error("Action failed:", e);
      }
    },
    [onTodoChanged],
  );

  const footer = loading
    ? t("common.loading")
    : error
      ? t("list.error", { msg: error })
      : t("list.items", { count: total });

  return (
    <div className="todo-list">
      <div className="todo-list__body">
        {items.map((todo) => (
          <TodoRow
            key={todo.id}
            todo={todo}
            selected={todo.id === selectedId}
            onClick={() => onOpenDetail(todo)}
            onAction={(action) => doAction(todo, action)}
          />
        ))}
        {!loading && items.length === 0 && !error && (
          <div className="todo-list__empty">{t("list.empty")}</div>
        )}
      </div>
      <div className="todo-list__footer">{footer}</div>
    </div>
  );
}

interface TodoRowProps {
  todo: Todo;
  selected: boolean;
  onClick: () => void;
  onAction: (action: "start" | "complete") => void;
}

function TodoRow({ todo, selected, onClick, onAction }: TodoRowProps) {
  const { t } = useTranslation();
  const code = formatDisplayCode(todo.category, todo.code);
  const title = todo.title || t("list.untitled");
  const done = todo.status === "completed" || todo.status === "duplicate";

  return (
    <div
      className={`todo-row${selected ? " todo-row--selected" : ""}`}
      onClick={onClick}
    >
      <div className="todo-row__top">
        <span className="dot" style={{ background: PRIORITY_COLORS[todo.priority] }} />
        <span className={`todo-row__title${done ? " todo-row__title--done" : ""}`}>
          {title}
        </span>
        {todo.pinned && <PinIcon size={12} filled className="todo-row__pin" />}
        {todo.status === "open" && (
          <button
            className="row-action"
            title={t("list.start")}
            onClick={(e) => {
              e.stopPropagation();
              onAction("start");
            }}
          >
            <PlayIcon size={13} />
          </button>
        )}
        {todo.status === "in_progress" && (
          <button
            className="row-action"
            title={t("list.complete")}
            onClick={(e) => {
              e.stopPropagation();
              onAction("complete");
            }}
          >
            <CheckIcon size={13} />
          </button>
        )}
      </div>
      <div className="todo-row__bottom">
        <span className="todo-row__code">{code}</span>
        <span className={`badge badge--cat-${todo.category}`}>
          {t(`category.${todo.category}`)}
        </span>
        <span className={`badge badge--st-${todo.status}`}>{statusLabel(t, todo.status)}</span>
        {todo.highlighted && <span className="badge badge--warm">{t("list.highlighted")}</span>}
      </div>
    </div>
  );
}

/** Read a filter set from localStorage and return it as a comma-joined string
 * (the backend expects comma-separated values for multi-select filters). */
function loadFilter(key: string): string | undefined {
  try {
    const raw = localStorage.getItem(key);
    if (!raw) return undefined;
    const arr = JSON.parse(raw) as string[];
    const filtered = arr.filter((v) => v);
    return filtered.length ? filtered.join(",") : undefined;
  } catch {
    return undefined;
  }
}
