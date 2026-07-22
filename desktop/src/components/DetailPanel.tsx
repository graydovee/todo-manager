import { useState, useEffect } from "react";
import * as todoApi from "../api/todos";
import { formatDisplayCode } from "../utils/displayCode";
import type { TodoDetail, Status, TodoSummary, Priority } from "../types";

interface DetailPanelProps {
  detail: TodoDetail | null;
  loading: boolean;
  editing: boolean;
  onEditingChange: (v: boolean) => void;
  onTodoChanged: () => void;
}

/**
 * DetailPanel — shows the full detail of the selected todo: header, status,
 * action buttons, metadata, relations, and comments.
 */
export function DetailPanel({
  detail,
  loading,
  editing,
  onEditingChange,
  onTodoChanged,
}: DetailPanelProps) {
  if (loading) return <div className="detail-panel">Loading…</div>;
  if (!detail) return <div className="detail-panel">No detail</div>;

  const t = detail;
  const code = formatDisplayCode(t.category, t.code);

  const doAction = async (action: "start" | "complete" | "reopen") => {
    try {
      if (action === "start") await todoApi.startTodo(t.id);
      else if (action === "complete")
        await todoApi.completeTodo(t.id, false);
      else await todoApi.reopenTodo(t.id, false);
      onTodoChanged();
    } catch (e) {
      console.error("Action failed:", e);
    }
  };

  return (
    <div className="detail-panel">
      <div className="detail-panel__header-text">{code}&nbsp;&nbsp;{t.title}</div>
      <div className="detail-panel__status">{statusLabel(t.status)}</div>

      <div className="detail-panel__actions">
        {t.status === "open" && (
          <button className="btn-primary" onClick={() => doAction("start")}>
            Start
          </button>
        )}
        {t.status !== "completed" && t.status !== "duplicate" && (
          <button className="btn-primary" onClick={() => doAction("complete")}>
            Complete
          </button>
        )}
        {t.status === "completed" && (
          <button className="btn-primary" onClick={() => doAction("reopen")}>
            Reopen
          </button>
        )}
      </div>

      <div className="detail-sep" />

      <Section label="Description">{t.description || "-"}</Section>

      <div className="detail-sep" />

      <div className="detail-grid">
        <Section label="Priority">{(t.priority || "").toUpperCase() || "-"}</Section>
        <Section label="Due">{t.due_at || "-"}</Section>
      </div>
      <div className="detail-grid">
        <Section label="Created">{formatTime(t.created_at)}</Section>
        <Section label="Updated">{formatTime(t.updated_at)}</Section>
      </div>

      <Section label="Tags">{(t.tags ?? []).join(", ") || "-"}</Section>

      <div className="detail-sep" />

      <Section label="Prerequisites">
        {summariesText(detail.depends_on)}
      </Section>
      <Section label="Dependents">
        {summariesText(detail.depended_by)}
      </Section>
      <Section label="Duplicate of">
        {detail.duplicate_of
          ? `${formatDisplayCode(detail.duplicate_of.category, detail.duplicate_of.code)}  ${detail.duplicate_of.title}`
          : "-"}
      </Section>
      <Section label="Duplicates">
        {summariesText(detail.duplicates)}
      </Section>

      <div className="detail-sep" />

      <CommentsSection todoId={t.id} />

      {editing && (
        <EditForm
          detail={detail}
          onSaved={() => {
            onEditingChange(false);
            onTodoChanged();
          }}
        />
      )}
    </div>
  );
}

function Section({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="detail-section">
      <div className="detail-section__label">{label}</div>
      <div className="detail-section__value">{children}</div>
    </div>
  );
}

function summariesText(
  summaries: TodoSummary[] | undefined,
): string {
  if (!summaries || summaries.length === 0) return "-";
  return summaries
    .map((s) => `${formatDisplayCode(s.category, s.code)}  ${s.title}`)
    .join("\n");
}

function formatTime(s: string): string {
  if (!s) return "-";
  try {
    const d = new Date(s);
    if (isNaN(d.getTime())) return s;
    return d
      .toLocaleString("sv-SE", {
        year: "numeric",
        month: "2-digit",
        day: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
      })
      .replace("T", " ");
  } catch {
    return s;
  }
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

// --- Comments ---

function CommentsSection({
  todoId,
}: {
  todoId: number;
}) {
  const [comments, setComments] = useState<
    { id: number; content: string; created_at: string }[]
  >([]);
  const [text, setText] = useState("");
  const [loadingC, setLoadingC] = useState(true);

  useEffect(() => {
    void (async () => {
      try {
        const c = await todoApi.listComments(todoId);
        setComments(c ?? []);
      } catch {
        // ignore
      } finally {
        setLoadingC(false);
      }
    })();
  }, [todoId]);

  const send = async () => {
    const content = text.trim();
    if (!content) return;
    try {
      await todoApi.createComment(todoId, content);
      setText("");
      const c = await todoApi.listComments(todoId);
      setComments(c ?? []);
    } catch (e) {
      console.error("Send comment failed:", e);
    }
  };

  const del = async (id: number) => {
    try {
      await todoApi.deleteComment(todoId, id);
      const c = await todoApi.listComments(todoId);
      setComments(c ?? []);
    } catch (e) {
      console.error("Delete comment failed:", e);
    }
  };

  return (
    <Section label="Comments">
      {loadingC ? (
        "Loading…"
      ) : comments.length === 0 ? (
        <span className="detail-muted">No comments</span>
      ) : (
        <div className="comment-list">
          {comments.map((c) => (
            <div key={c.id} className="comment-item">
              <div className="comment-item__body">{c.content}</div>
              <div className="comment-item__meta">
                <span className="comment-item__time">{formatTime(c.created_at)}</span>
                <button
                  className="comment-item__del"
                  onClick={() => del(c.id)}
                >
                  Del
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
      <div className="comment-input-row">
        <textarea
          className="comment-input"
          placeholder="Add a comment…"
          value={text}
          onChange={(e) => setText(e.target.value)}
          rows={2}
        />
        <button className="btn-primary" onClick={send}>
          Send
        </button>
      </div>
    </Section>
  );
}

// --- Edit form ---

function EditForm({
  detail,
  onSaved,
}: {
  detail: TodoDetail;
  onSaved: () => void;
}) {
  const [title, setTitle] = useState(detail.title);
  const [desc, setDesc] = useState(detail.description);
  const [priority, setPriority] = useState<Priority>(detail.priority);
  const [tags, setTags] = useState((detail.tags ?? []).join(", "));

  const save = async () => {
    try {
      await todoApi.updateTodo(detail.id, {
        title,
        description: desc,
        priority: (priority || "p2") as Priority,
        tags: tags
          .split(",")
          .map((t) => t.trim())
          .filter(Boolean),
      });
      onSaved();
    } catch (e) {
      console.error("Save failed:", e);
    }
  };

  return (
    <div className="detail-sep" >
      <div className="edit-form">
        <label className="edit-form__label">Title *</label>
        <input
          className="text-input"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
        />
        <label className="edit-form__label">Description</label>
        <textarea
          className="text-input"
          rows={3}
          value={desc}
          onChange={(e) => setDesc(e.target.value)}
        />
        <label className="edit-form__label">Priority (P0-P3)</label>
        <input
          className="text-input"
          value={priority}
          onChange={(e) => setPriority(e.target.value as Priority)}
        />
        <label className="edit-form__label">Tags (comma separated)</label>
        <input
          className="text-input"
          value={tags}
          onChange={(e) => setTags(e.target.value)}
        />
        <button className="btn-primary" onClick={save}>
          Save
        </button>
      </div>
    </div>
  );
}
