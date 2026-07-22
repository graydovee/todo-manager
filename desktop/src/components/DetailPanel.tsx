import { useState, useEffect } from "react";
import { useTranslation } from "react-i18next";
import dayjs from "dayjs";
import * as todoApi from "../api/todos";
import { formatDisplayCode } from "../utils/displayCode";
import { statusLabel, categoryOptions, priorityOptions } from "../utils/enumOptions";
import { Field } from "./ui/Field";
import { Select } from "./ui/Select";
import { TagInput } from "./ui/TagInput";
import { TrashIcon } from "./ui/icons";
import type { TodoDetail, TodoSummary, Category, Priority } from "../types";

interface DetailPanelProps {
  detail: TodoDetail | null;
  loading: boolean;
  editing: boolean;
  onEditingChange: (v: boolean) => void;
  onTodoChanged: () => void;
}

/**
 * DetailPanel — shows the full detail of the selected todo: header, badges,
 * action buttons, metadata, relations, and comments.
 */
export function DetailPanel({
  detail,
  loading,
  editing,
  onEditingChange,
  onTodoChanged,
}: DetailPanelProps) {
  const { t } = useTranslation();

  if (loading) return <div className="detail-panel">{t("common.loading")}</div>;
  if (!detail) return <div className="detail-panel">{t("detail.noDetail")}</div>;

  const todo = detail;
  const code = formatDisplayCode(todo.category, todo.code);

  const doAction = async (action: "start" | "complete" | "reopen") => {
    try {
      if (action === "start") await todoApi.startTodo(todo.id);
      else if (action === "complete") await todoApi.completeTodo(todo.id, false);
      else await todoApi.reopenTodo(todo.id, false);
      onTodoChanged();
    } catch (e) {
      console.error("Action failed:", e);
    }
  };

  return (
    <div className="detail-panel">
      <div className="detail-panel__header-text">
        {todo.title}
      </div>
      <div className="detail-panel__meta-row">
        <span className="todo-row__code">{code}</span>
        <span className={`badge badge--cat-${todo.category}`}>{t(`category.${todo.category}`)}</span>
        <span className={`badge badge--st-${todo.status}`}>{statusLabel(t, todo.status)}</span>
        <span className={`badge badge--${todo.priority}`}>{todo.priority.toUpperCase()}</span>
      </div>

      <div className="detail-panel__actions">
        {todo.status === "open" && (
          <button className="btn btn--primary btn--sm" onClick={() => doAction("start")}>
            {t("detail.start")}
          </button>
        )}
        {todo.status !== "completed" && todo.status !== "duplicate" && (
          <button className="btn btn--primary btn--sm" onClick={() => doAction("complete")}>
            {t("detail.complete")}
          </button>
        )}
        {todo.status === "completed" && (
          <button className="btn btn--primary btn--sm" onClick={() => doAction("reopen")}>
            {t("detail.reopen")}
          </button>
        )}
      </div>

      <div className="detail-sep" />

      <Section label={t("detail.description")}>
        {todo.description || <span className="detail-muted">-</span>}
      </Section>

      <div className="detail-sep" />

      <div className="detail-grid">
        <Section label={t("detail.due")}>{todo.due_at ? formatTime(todo.due_at) : "-"}</Section>
        <Section label={t("detail.tags")}>{(todo.tags ?? []).join(", ") || "-"}</Section>
      </div>
      <div className="detail-grid">
        <Section label={t("detail.created")}>{formatTime(todo.created_at)}</Section>
        <Section label={t("detail.updated")}>{formatTime(todo.updated_at)}</Section>
      </div>

      <div className="detail-sep" />

      <Section label={t("detail.prerequisites")}>{summariesText(detail.depends_on)}</Section>
      <Section label={t("detail.dependents")}>{summariesText(detail.depended_by)}</Section>
      <Section label={t("detail.duplicateOf")}>
        {detail.duplicate_of
          ? `${formatDisplayCode(detail.duplicate_of.category, detail.duplicate_of.code)}  ${detail.duplicate_of.title}`
          : "-"}
      </Section>
      <Section label={t("detail.duplicates")}>{summariesText(detail.duplicates)}</Section>

      <div className="detail-sep" />

      <CommentsSection todoId={todo.id} />

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

function summariesText(summaries: TodoSummary[] | undefined): React.ReactNode {
  if (!summaries || summaries.length === 0) return "-";
  return summaries
    .map((s) => `${formatDisplayCode(s.category, s.code)}  ${s.title}`)
    .join("\n");
}

function formatTime(s: string): string {
  if (!s) return "-";
  const d = dayjs(s);
  return d.isValid() ? d.format("YYYY-MM-DD HH:mm") : s;
}

// --- Comments ---

function CommentsSection({ todoId }: { todoId: number }) {
  const { t } = useTranslation();
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
    <Section label={t("detail.comments")}>
      {loadingC ? (
        t("common.loading")
      ) : comments.length === 0 ? (
        <span className="detail-muted">{t("detail.noComments")}</span>
      ) : (
        <div className="comment-list">
          {comments.map((c) => (
            <div key={c.id} className="comment-item">
              <div className="comment-item__body">{c.content}</div>
              <div className="comment-item__meta">
                <span className="comment-item__time">{formatTime(c.created_at)}</span>
                <button className="icon-btn" title={t("common.delete")} onClick={() => del(c.id)}>
                  <TrashIcon size={14} />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
      <div className="comment-input-row">
        <textarea
          className="text-input"
          placeholder={t("detail.commentPlaceholder")}
          value={text}
          onChange={(e) => setText(e.target.value)}
          rows={2}
        />
        <button className="btn btn--primary btn--sm" onClick={send}>
          {t("detail.send")}
        </button>
      </div>
    </Section>
  );
}

// --- Edit form ---

function EditForm({ detail, onSaved }: { detail: TodoDetail; onSaved: () => void }) {
  const { t } = useTranslation();
  const [title, setTitle] = useState(detail.title);
  const [desc, setDesc] = useState(detail.description);
  const [category, setCategory] = useState<Category>(detail.category);
  const [priority, setPriority] = useState<Priority>(detail.priority);
  const [tags, setTags] = useState<string[]>(detail.tags ?? []);
  const [dueAt, setDueAt] = useState(
    detail.due_at ? dayjs(detail.due_at).format("YYYY-MM-DDTHH:mm") : "",
  );
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  const save = async () => {
    if (!title.trim()) {
      setError(t("form.titleRequired"));
      return;
    }
    setError("");
    setSaving(true);
    try {
      await todoApi.updateTodo(detail.id, {
        title: title.trim(),
        description: desc,
        category,
        priority,
        tags,
        due_at: dueAt ? dayjs(dueAt).toISOString() : null,
      });
      onSaved();
    } catch (e) {
      console.error("Save failed:", e);
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <div className="detail-sep" />
      <form
        className="edit-form"
        onSubmit={(e) => {
          e.preventDefault();
          void save();
        }}
      >
        <Field label={t("form.title")} required error={error}>
          <input className="text-input" value={title} onChange={(e) => setTitle(e.target.value)} />
        </Field>
        <Field label={t("form.description")}>
          <textarea
            className="text-input"
            rows={3}
            value={desc}
            onChange={(e) => setDesc(e.target.value)}
          />
        </Field>
        <Field label={t("form.category")}>
          <Select
            options={categoryOptions(t)}
            value={category}
            onChange={(v) => setCategory(v as Category)}
          />
        </Field>
        <Field label={t("form.priority")}>
          <Select
            options={priorityOptions(t)}
            value={priority}
            onChange={(v) => setPriority(v as Priority)}
          />
        </Field>
        <Field label={t("form.tags")}>
          <TagInput value={tags} onChange={setTags} placeholder={t("form.tagsPlaceholder")} />
        </Field>
        <Field label={t("form.dueDate")}>
          <input
            type="datetime-local"
            className="text-input"
            value={dueAt}
            onChange={(e) => setDueAt(e.target.value)}
          />
        </Field>
        <button type="submit" className="btn btn--primary" disabled={saving}>
          {t("detail.save")}
        </button>
      </form>
    </>
  );
}
