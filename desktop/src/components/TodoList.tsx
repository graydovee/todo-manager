import { useState, useEffect, useCallback } from "react";
import * as todoApi from "../api/todos";
import { formatDisplayCode } from "../utils/displayCode";
import type { Todo, Status } from "../types";

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
 * Fetches todos via the shared API (no TanStack Query needed for this simple
 * list). Each row shows "CODE  Title" (bold) on the left and priority / status
 * / action button on the right. Clicking a row opens the detail side panel.
 */
export function TodoList({
  refreshKey,
  selectedId,
  onOpenDetail,
  onTodoChanged,
}: TodoListProps) {
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
        // TODO: show conflict dialog on 409
        console.error("Action failed:", e);
      }
    },
    [onTodoChanged],
  );

  const footer = loading
    ? "Loading…"
    : error
      ? `Error: ${error}`
      : `${total} items`;

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
          <div className="todo-list__empty">No todos</div>
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
  const code = formatDisplayCode(todo.category, todo.code);
  const title = todo.title || "(untitled)";
  const priority = (todo.priority || "").toUpperCase();
  const status = statusLabel(todo.status);

  let actionBtn: React.ReactNode = null;
  if (todo.status === "open") {
    actionBtn = (
      <button
        className="row-action"
        title="Start"
        onClick={(e) => {
          e.stopPropagation();
          onAction("start");
        }}
      >
        ▶
      </button>
    );
  } else if (todo.status === "in_progress") {
    actionBtn = (
      <button
        className="row-action"
        title="Complete"
        onClick={(e) => {
          e.stopPropagation();
          onAction("complete");
        }}
      >
        ✓
      </button>
    );
  }

  return (
    <div
      className={`todo-row${selected ? " todo-row--selected" : ""}`}
      onClick={onClick}
    >
      <div className="todo-row__main">
        <span className="todo-row__title">
          {code}&nbsp;&nbsp;{title}
        </span>
      </div>
      <div className="todo-row__meta">
        {priority && <span className="todo-row__priority">{priority}</span>}
        {status && (
          <span
            className={`todo-row__status${todo.status === "in_progress" ? " todo-row__status--bold" : ""}`}
          >
            {status}
          </span>
        )}
        {actionBtn}
      </div>
    </div>
  );
}

function statusLabel(s: Status): string {
  switch (s) {
    case "open":
      return "Not Started";
    case "in_progress":
      return "In Progress";
    case "completed":
      return "Completed";
    case "duplicate":
      return "Duplicate";
    default:
      return s;
  }
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
