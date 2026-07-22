import { useState } from "react";
import { useTranslation } from "react-i18next";
import dayjs from "dayjs";
import * as todoApi from "../api/todos";
import type { CreateTodoInput, Category, Priority, TodoSummary } from "../types";
import { Field } from "./ui/Field";
import { Select } from "./ui/Select";
import { TagInput } from "./ui/TagInput";
import { TodoSearchSelect } from "./ui/TodoSearchSelect";
import { categoryOptions, priorityOptions } from "../utils/enumOptions";

interface CreatePanelProps {
  onTodoChanged: () => void;
}

/**
 * CreatePanel — form to create a new todo. Fields mirror the web frontend:
 * title, description, category/priority dropdowns, tag tokens, due date,
 * and a remote-search prerequisite picker.
 */
export function CreatePanel({ onTodoChanged }: CreatePanelProps) {
  const { t } = useTranslation();
  const [title, setTitle] = useState("");
  const [desc, setDesc] = useState("");
  const [category, setCategory] = useState<Category>("task");
  const [priority, setPriority] = useState<Priority>("p2");
  const [tags, setTags] = useState<string[]>([]);
  const [dueAt, setDueAt] = useState("");
  const [deps, setDeps] = useState<TodoSummary[]>([]);
  const [error, setError] = useState("");
  const [msg, setMsg] = useState<{ kind: "ok" | "error"; text: string } | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const create = async () => {
    setMsg(null);
    if (!title.trim()) {
      setError(t("form.titleRequired"));
      return;
    }
    setError("");
    const input: CreateTodoInput = {
      title: title.trim(),
      category,
      priority,
    };
    if (desc.trim()) input.description = desc.trim();
    if (tags.length) input.tags = tags;
    if (dueAt) input.due_at = dayjs(dueAt).toISOString();
    if (deps.length) input.depends_on_ids = deps.map((d) => d.id);

    setSubmitting(true);
    try {
      await todoApi.createTodo(input);
      setMsg({ kind: "ok", text: t("form.created") });
      setTitle("");
      setDesc("");
      setCategory("task");
      setPriority("p2");
      setTags([]);
      setDueAt("");
      setDeps([]);
      onTodoChanged();
    } catch (e) {
      setMsg({
        kind: "error",
        text: t("form.createFailed", { msg: e instanceof Error ? e.message : String(e) }),
      });
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <form
      className="create-panel"
      onSubmit={(e) => {
        e.preventDefault();
        void create();
      }}
    >
      <Field label={t("form.title")} required error={error}>
        <input
          className="text-input"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder={t("form.titlePlaceholder")}
          autoFocus
        />
      </Field>
      <Field label={t("form.description")}>
        <textarea
          className="text-input"
          rows={3}
          value={desc}
          onChange={(e) => setDesc(e.target.value)}
          placeholder={t("form.descriptionPlaceholder")}
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
      <Field label={t("form.dependsOn")}>
        <TodoSearchSelect
          value={deps}
          onChange={setDeps}
          placeholder={t("form.dependsOnPlaceholder")}
        />
      </Field>
      <button type="submit" className="btn btn--primary" disabled={submitting}>
        {t("form.create")}
      </button>
      {msg && <div className={`form-msg form-msg--${msg.kind}`}>{msg.text}</div>}
    </form>
  );
}
