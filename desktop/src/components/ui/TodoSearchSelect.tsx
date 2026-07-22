import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { listTodos } from "../../api/todos";
import { formatDisplayCode } from "../../utils/displayCode";
import type { Todo, TodoSummary } from "../../types";
import { usePopover } from "./usePopover";
import { XIcon } from "./icons";

interface TodoSearchSelectProps {
  value: TodoSummary[];
  onChange: (items: TodoSummary[]) => void;
  placeholder?: string;
}

function toSummary(t: Todo): TodoSummary {
  return { id: t.id, code: t.code, title: t.title, category: t.category, status: t.status };
}

/**
 * TodoSearchSelect — remote-search multi-select mirroring the web frontend's
 * prerequisite picker: 300ms debounced search over open/in-progress todos,
 * preloads on first open, options render as "CODE - title", selected items
 * become removable pills.
 */
export function TodoSearchSelect({ value, onChange, placeholder }: TodoSearchSelectProps) {
  const { t } = useTranslation();
  const { open, setOpen, ref } = usePopover();
  const [input, setInput] = useState("");
  const [focused, setFocused] = useState(false);
  const [results, setResults] = useState<Todo[]>([]);
  const [loading, setLoading] = useState(false);
  const [activeIndex, setActiveIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);

  // Debounced remote search while the dropdown is open.
  useEffect(() => {
    if (!open) return;
    const timer = setTimeout(() => {
      void (async () => {
        setLoading(true);
        try {
          const res = await listTodos({
            q: input.trim() || undefined,
            page_size: 20,
            status: "open,in_progress",
          });
          setResults(res.items ?? []);
        } catch {
          setResults([]);
        } finally {
          setLoading(false);
        }
      })();
    }, 300);
    return () => clearTimeout(timer);
  }, [input, open]);

  useEffect(() => setActiveIndex(0), [input, open]);

  const selectedIds = new Set(value.map((v) => v.id));
  const options = results.filter((r) => !selectedIds.has(r.id));
  const showDropdown = open && focused;

  const select = (todo: Todo) => {
    onChange([...value, toSummary(todo)]);
    setInput("");
    inputRef.current?.focus();
  };

  const remove = (id: number) => onChange(value.filter((v) => v.id !== id));

  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      const opt = options[activeIndex];
      if (opt) select(opt);
    } else if (e.key === "Backspace" && !input && value.length) {
      remove(value[value.length - 1].id);
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIndex((i) => Math.min(options.length - 1, i + 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIndex((i) => Math.max(0, i - 1));
    }
  };

  return (
    <div className="select" ref={ref}>
      <div
        className={`token-input${focused ? " token-input--focused" : ""}`}
        onClick={() => inputRef.current?.focus()}
      >
        {value.map((item) => (
          <span key={item.id} className="pill">
            <span className="pill__text">
              {formatDisplayCode(item.category, item.code)} · {item.title}
            </span>
            <button type="button" className="pill__x" onClick={() => remove(item.id)}>
              <XIcon size={10} />
            </button>
          </span>
        ))}
        <input
          ref={inputRef}
          className="token-input__input"
          value={input}
          placeholder={value.length ? "" : placeholder}
          onChange={(e) => {
            setInput(e.target.value);
            setOpen(true);
          }}
          onFocus={() => {
            setFocused(true);
            setOpen(true);
          }}
          onBlur={() => setFocused(false)}
          onKeyDown={onKeyDown}
        />
      </div>
      {showDropdown && (
        <div className="select__dropdown">
          {loading && <div className="select__empty">{t("form.searching")}</div>}
          {!loading && options.length === 0 && (
            <div className="select__empty">{t("form.noMatch")}</div>
          )}
          {!loading &&
            options.map((todo, i) => (
              <button
                key={todo.id}
                type="button"
                className={`select__option${i === activeIndex ? " select__option--active" : ""}`}
                onMouseEnter={() => setActiveIndex(i)}
                onMouseDown={(e) => {
                  e.preventDefault();
                  select(todo);
                }}
              >
                <span className="select__option-label">{todo.title}</span>
                <span className="select__option-sub">
                  {formatDisplayCode(todo.category, todo.code)}
                </span>
              </button>
            ))}
        </div>
      )}
    </div>
  );
}
