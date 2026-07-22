import { useState } from "react";
import * as todoApi from "../api/todos";
import type { CreateTodoInput, Category, Priority } from "../types";

interface CreatePanelProps {
  onTodoChanged: () => void;
}

/**
 * CreatePanel — form to create a new todo.
 *
 * On success: refreshes the list (so the new item appears) and the parent
 * DesktopMain will switch to detail mode for the new todo.
 */
export function CreatePanel({ onTodoChanged }: CreatePanelProps) {
  const [title, setTitle] = useState("");
  const [desc, setDesc] = useState("");
  const [category, setCategory] = useState<Category>("task");
  const [priority, setPriority] = useState<Priority>("p2");
  const [tags, setTags] = useState("");
  const [msg, setMsg] = useState("");

  const create = async () => {
    if (!title.trim()) {
      setMsg("Title is required");
      return;
    }
    const cat = category.trim().toLowerCase();
    if (cat && !["bug", "feature", "task"].includes(cat)) {
      setMsg("Category must be bug/feature/task");
      return;
    }
    const input: CreateTodoInput = {
      title: title.trim(),
      category: (cat || "task") as Category,
      priority: (priority.trim().toLowerCase() || "p2") as Priority,
    };
    if (desc.trim()) input.description = desc.trim();
    const parsedTags = tags.split(",").map((t) => t.trim()).filter(Boolean);
    if (parsedTags.length) input.tags = parsedTags;

    try {
      await todoApi.createTodo(input);
      setMsg("Created");
      setTitle(""); setDesc(""); setCategory("task"); setPriority("p2"); setTags("");
      onTodoChanged();
    } catch (e) {
      setMsg(`Failed: ${e instanceof Error ? e.message : String(e)}`);
    }
  };

  return (
    <div className="create-panel">
      <Field label="Title *">
        <input className="text-input" value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Title" autoFocus />
      </Field>
      <Field label="Description">
        <textarea className="text-input" rows={3} value={desc} onChange={(e) => setDesc(e.target.value)} placeholder="Description" />
      </Field>
      <Field label="Category (bug/feature/task)">
        <input className="text-input" value={category} onChange={(e) => setCategory(e.target.value as Category)} />
      </Field>
      <Field label="Priority (P0-P3)">
        <input className="text-input" value={priority} onChange={(e) => setPriority(e.target.value as Priority)} />
      </Field>
      <Field label="Tags (comma separated)">
        <input className="text-input" value={tags} onChange={(e) => setTags(e.target.value)} />
      </Field>
      <button className="btn-primary" onClick={create}>Create</button>
      {msg && <div className="create-msg">{msg}</div>}
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="manage-field">
      <label className="manage-field__label">{label}</label>
      {children}
    </div>
  );
}
