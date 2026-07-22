import { useState, useRef, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useAuth } from "../stores/authContext";
import { Field } from "./ui/Field";
import { ChipGroup } from "./ui/ChipGroup";
import { statusLabel, CATEGORIES, PRIORITIES, STATUSES } from "../utils/enumOptions";

interface ManagePanelProps {
  onLogout: () => void;
}

/**
 * ManagePanel — filters, search, language, and logout.
 *
 * The selected filters are stored in localStorage so the TodoList can read
 * them. The Apply button triggers a list refresh via a custom event.
 */
export function ManagePanel({ onLogout }: ManagePanelProps) {
  const { t, i18n } = useTranslation();
  const { logout } = useAuth();
  const [statusSel, setStatusSel] = useState<Set<string>>(() => loadSet("filter_status", ["open", "in_progress"]));
  const [catSel, setCatSel] = useState<Set<string>>(() => loadSet("filter_category"));
  const [priSel, setPriSel] = useState<Set<string>>(() => loadSet("filter_priority"));
  const [query, setQuery] = useState(() => localStorage.getItem("filter_query") ?? "");
  const [codeSearch, setCodeSearch] = useState(() => localStorage.getItem("filter_code") ?? "");
  const [opacity, setOpacity] = useState(() => loadOpacity());
  const [previewing, setPreviewing] = useState(false);
  const previewTimer = useRef<number | null>(null);

  // Make sure a pending preview is cancelled if the panel unmounts.
  useEffect(() => {
    return () => {
      if (previewTimer.current) window.clearTimeout(previewTimer.current);
      document.getElementById("root")?.classList.remove("opacity-preview");
    };
  }, []);

  const changeOpacity = (v: number) => {
    setOpacity(v);
    localStorage.setItem("lock_opacity", String(v));
    window.dispatchEvent(new CustomEvent("lock-opacity-changed"));
  };

  const togglePreview = () => {
    const root = document.getElementById("root");
    if (!root) return;
    if (previewing) {
      if (previewTimer.current) window.clearTimeout(previewTimer.current);
      root.classList.remove("opacity-preview");
      setPreviewing(false);
      return;
    }
    root.classList.add("opacity-preview");
    setPreviewing(true);
    previewTimer.current = window.setTimeout(() => {
      root.classList.remove("opacity-preview");
      setPreviewing(false);
    }, 2500);
  };

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
    localStorage.setItem("lang", l);
    void i18n.changeLanguage(l);
  };

  const doLogout = async () => {
    onLogout();
    await logout();
  };

  return (
    <div className="manage-panel">
      <Field label={t("manage.language")}>
        <select
          className="text-input"
          value={i18n.language}
          onChange={(e) => changeLang(e.target.value)}
        >
          <option value="en">English</option>
          <option value="zh">简体中文</option>
        </select>
      </Field>

      <div className="detail-sep" />

      <Field label={`${t("manage.opacity")} · ${Math.round(opacity * 100)}%`}>
        <div className="opacity-row">
          <input
            type="range"
            className="opacity-slider"
            min={0.2}
            max={1}
            step={0.05}
            value={opacity}
            onChange={(e) => changeOpacity(parseFloat(e.target.value))}
          />
          <button className="btn btn--ghost btn--sm" onClick={togglePreview}>
            {previewing ? t("manage.previewing") : t("manage.preview")}
          </button>
        </div>
      </Field>

      <div className="detail-sep" />

      <Field label={t("manage.status")}>
        <ChipGroup
          options={STATUSES.map((s) => ({ value: s, label: statusLabel(t, s) }))}
          selected={statusSel}
          onToggle={(v) => toggle(statusSel, setStatusSel, v)}
        />
      </Field>
      <Field label={t("manage.category")}>
        <ChipGroup
          options={CATEGORIES.map((c) => ({ value: c, label: t(`category.${c}`) }))}
          selected={catSel}
          onToggle={(v) => toggle(catSel, setCatSel, v)}
        />
      </Field>
      <Field label={t("manage.priority")}>
        <ChipGroup
          options={PRIORITIES.map((p) => ({ value: p, label: t(`priority.${p}`) }))}
          selected={priSel}
          onToggle={(v) => toggle(priSel, setPriSel, v)}
        />
      </Field>

      <Field label={t("manage.titleSearch")}>
        <input
          className="text-input"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t("manage.titleSearchPlaceholder")}
        />
      </Field>
      <Field label={t("manage.codeExact")}>
        <input
          className="text-input"
          value={codeSearch}
          onChange={(e) => setCodeSearch(e.target.value)}
          placeholder={t("manage.codePlaceholder")}
        />
      </Field>

      <button className="btn btn--primary" onClick={apply}>
        {t("manage.apply")}
      </button>

      <div className="detail-sep" />

      <button className="btn btn--ghost" onClick={doLogout}>
        {t("manage.logout")}
      </button>
    </div>
  );
}

function loadOpacity(): number {
  const v = parseFloat(localStorage.getItem("lock_opacity") ?? "0.75");
  return isNaN(v) ? 0.75 : Math.min(1, Math.max(0.2, v));
}

function loadSet(key: string, defaults?: string[]): Set<string> {
  try {
    const raw = localStorage.getItem(key);
    if (raw) return new Set(JSON.parse(raw));
  } catch {
    /* ignore */
  }
  return new Set(defaults ?? []);
}

function saveSet(key: string, set: Set<string>) {
  localStorage.setItem(key, JSON.stringify([...set]));
}
