import { useState, useRef, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useAuth } from "../stores/authContext";
import { setMode } from "../api/config";
import { Field } from "./ui/Field";
import { ChipGroup } from "./ui/ChipGroup";
import { HotkeyRecorder } from "./ui/HotkeyRecorder";
import { statusLabel, CATEGORIES, PRIORITIES, STATUSES } from "../utils/enumOptions";
import { applyHotkey } from "../utils/hotkey";
import { DEFAULT_FILTER_STATUS, DEFAULT_LOCK_OPACITY, DEFAULT_UNLOCK_HOTKEY, detectLang } from "../utils/settings";

interface ManagePanelProps {
  onLogout: () => void;
}

/**
 * ManagePanel — grouped settings sections: filters, interface, hotkeys.
 *
 * Filters are stored in localStorage so the TodoList can read them; the
 * Apply button triggers a list refresh via a custom event. New sections
 * are added by dropping in another <Section> block plus its i18n keys.
 */
export function ManagePanel({ onLogout }: ManagePanelProps) {
  const { t, i18n } = useTranslation();
  const { logout } = useAuth();
  const [statusSel, setStatusSel] = useState<Set<string>>(() => loadSet("filter_status", DEFAULT_FILTER_STATUS));
  const [catSel, setCatSel] = useState<Set<string>>(() => loadSet("filter_category"));
  const [priSel, setPriSel] = useState<Set<string>>(() => loadSet("filter_priority"));
  const [query, setQuery] = useState(() => localStorage.getItem("filter_query") ?? "");
  const [codeSearch, setCodeSearch] = useState(() => localStorage.getItem("filter_code") ?? "");
  const [opacity, setOpacity] = useState(() => loadOpacity());
  const [previewing, setPreviewing] = useState(false);
  const previewTimer = useRef<number | null>(null);

  // Two-step confirm state for restore-defaults.
  const [armRestore, setArmRestore] = useState(false);
  const restoreTimer = useRef<number | null>(null);
  // Bumped to make the HotkeyRecorder re-read the stored hotkey.
  const [hotkeySync, setHotkeySync] = useState(0);

  // Make sure pending timers are cancelled if the panel unmounts.
  useEffect(() => {
    return () => {
      if (previewTimer.current) window.clearTimeout(previewTimer.current);
      if (restoreTimer.current) window.clearTimeout(restoreTimer.current);
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

  // Clear the stored mode and log out, so the connection screen reappears and
  // the user can pick remote or local again. Switching does not delete data —
  // local SQLite and remote server data remain independent.
  const doSwitchConnection = async () => {
    await setMode("remote");
    onLogout();
    await logout();
  };

  // First click arms the button for 2.5s; a second click actually restores.
  const onRestoreClick = () => {
    if (!armRestore) {
      setArmRestore(true);
      restoreTimer.current = window.setTimeout(() => setArmRestore(false), 2500);
      return;
    }
    if (restoreTimer.current) window.clearTimeout(restoreTimer.current);
    setArmRestore(false);
    void restoreDefaults();
  };

  const restoreDefaults = async () => {
    // Filters — back to defaults and refresh the list immediately.
    const statusDef = new Set(DEFAULT_FILTER_STATUS);
    const empty = new Set<string>();
    setStatusSel(statusDef);
    setCatSel(empty);
    setPriSel(empty);
    saveSet("filter_status", statusDef);
    saveSet("filter_category", empty);
    saveSet("filter_priority", empty);
    setQuery("");
    setCodeSearch("");
    localStorage.setItem("filter_query", "");
    localStorage.setItem("filter_code", "");
    window.dispatchEvent(new CustomEvent("todo-filter-changed"));
    // Interface — opacity and language.
    changeOpacity(DEFAULT_LOCK_OPACITY);
    changeLang(detectLang());
    // Hotkey — re-register the default, then tell the recorder to re-read.
    try {
      await applyHotkey(DEFAULT_UNLOCK_HOTKEY);
    } catch (e) {
      console.error("restore hotkey failed:", e);
    }
    setHotkeySync((n) => n + 1);
  };

  return (
    <div className="manage-panel">
      <Section title={t("manage.secFilter")}>
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
      </Section>

      <div className="detail-sep" />

      <Section title={t("manage.secInterface")}>
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
      </Section>

      <div className="detail-sep" />

      <Section title={t("manage.secHotkeys")}>
        <Field label={t("manage.unlockHotkey")}>
          <HotkeyRecorder syncKey={hotkeySync} />
        </Field>
        <div className="manage-hint detail-muted">{t("manage.unlockHotkeyHint")}</div>
      </Section>

      <div className="detail-sep" />

      <button className="btn btn--ghost" onClick={onRestoreClick}>
        {armRestore ? t("manage.restoreConfirm") : t("manage.restoreDefaults")}
      </button>
      <button className="btn btn--ghost" onClick={doSwitchConnection}>
        {t("manage.switchConnection")}
      </button>
      <button className="btn btn--ghost" onClick={doLogout}>
        {t("manage.logout")}
      </button>
    </div>
  );
}

/** Section — a titled settings group. Add new blocks with another <Section>. */
function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="manage-section">
      <div className="detail-section__label">{title}</div>
      {children}
    </div>
  );
}

function loadOpacity(): number {
  const v = parseFloat(localStorage.getItem("lock_opacity") ?? String(DEFAULT_LOCK_OPACITY));
  return isNaN(v) ? DEFAULT_LOCK_OPACITY : Math.min(1, Math.max(0.2, v));
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
