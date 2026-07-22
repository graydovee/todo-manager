import { useState } from "react";
import { useAuth } from "../stores/authContext";

interface ManagePanelProps {
  onLogout: () => void;
}

const STATUS_OPTIONS = [
  { value: "open", label: "Not Started" },
  { value: "in_progress", label: "In Progress" },
  { value: "completed", label: "Completed" },
  { value: "duplicate", label: "Duplicate" },
];
const CATEGORY_OPTIONS = [
  { value: "bug", label: "bug" },
  { value: "feature", label: "feature" },
  { value: "task", label: "task" },
];
const PRIORITY_OPTIONS = [
  { value: "p0", label: "p0" },
  { value: "p1", label: "p1" },
  { value: "p2", label: "p2" },
  { value: "p3", label: "p3" },
];

/**
 * ManagePanel — filters, search, language, and logout.
 *
 * The selected filters are stored in localStorage so the TodoList can read
 * them. The Apply button triggers a list refresh via a custom event.
 */
export function ManagePanel({ onLogout }: ManagePanelProps) {
  const { logout } = useAuth();
  const [statusSel, setStatusSel] = useState<Set<string>>(() => loadSet("filter_status", ["open", "in_progress"]));
  const [catSel, setCatSel] = useState<Set<string>>(() => loadSet("filter_category"));
  const [priSel, setPriSel] = useState<Set<string>>(() => loadSet("filter_priority"));
  const [query, setQuery] = useState(() => localStorage.getItem("filter_query") ?? "");
  const [codeSearch, setCodeSearch] = useState(() => localStorage.getItem("filter_code") ?? "");
  const [lang, setLang] = useState(() => localStorage.getItem("lang") ?? "en");

  const toggle = (set: Set<string>, setFn: (s: Set<string>) => void, val: string) => {
    const next = new Set(set);
    if (next.has(val)) next.delete(val);
    else next.add(val);
    setFn(next);
  };

  const apply = () => {
    saveSet("filter_status", statusSel);
    saveSet("filter_category", catSel);
    saveSet("filter_priority", priSel);
    localStorage.setItem("filter_query", query);
    localStorage.setItem("filter_code", codeSearch);
    window.dispatchEvent(new CustomEvent("todo-filter-changed"));
  };

  const changeLang = (l: string) => {
    setLang(l);
    localStorage.setItem("lang", l);
    window.location.reload();
  };

  const doLogout = async () => {
    onLogout();
    await logout();
  };

  return (
    <div className="manage-panel">
      <Field label="Language">
        <select className="text-input" value={lang} onChange={(e) => changeLang(e.target.value)}>
          <option value="en">English</option>
          <option value="zh">简体中文</option>
        </select>
      </Field>

      <div className="detail-sep" />

      <Field label="Status">
        <ChipGroup options={STATUS_OPTIONS} selected={statusSel} onToggle={(v) => toggle(statusSel, setStatusSel, v)} />
      </Field>
      <Field label="Category">
        <ChipGroup options={CATEGORY_OPTIONS} selected={catSel} onToggle={(v) => toggle(catSel, setCatSel, v)} />
      </Field>
      <Field label="Priority">
        <ChipGroup options={PRIORITY_OPTIONS} selected={priSel} onToggle={(v) => toggle(priSel, setPriSel, v)} />
      </Field>

      <Field label="Title search">
        <input className="text-input" value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Search title…" />
      </Field>
      <Field label="Code (exact)">
        <input className="text-input" value={codeSearch} onChange={(e) => setCodeSearch(e.target.value)} placeholder="T-1" />
      </Field>

      <button className="btn-primary" onClick={apply}>Apply</button>

      <div className="detail-sep" />

      <button className="btn-primary" onClick={doLogout}>Logout</button>
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

function ChipGroup({
  options,
  selected,
  onToggle,
}: {
  options: { value: string; label: string }[];
  selected: Set<string>;
  onToggle: (v: string) => void;
}) {
  return (
    <div className="chip-group">
      {options.map((opt) => (
        <button
          key={opt.value}
          className={`chip${selected.has(opt.value) ? " chip--on" : ""}`}
          onClick={() => onToggle(opt.value)}
        >
          {opt.label}
        </button>
      ))}
    </div>
  );
}

function loadSet(key: string, defaults?: string[]): Set<string> {
  try {
    const raw = localStorage.getItem(key);
    if (raw) return new Set(JSON.parse(raw));
  } catch { /* ignore */ }
  return new Set(defaults ?? []);
}

function saveSet(key: string, set: Set<string>) {
  localStorage.setItem(key, JSON.stringify([...set]));
}
