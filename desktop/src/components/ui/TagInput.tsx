import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { fetchTags } from "../../api/todos";
import { usePopover } from "./usePopover";
import { XIcon } from "./icons";

interface TagInputProps {
  value: string[];
  onChange: (tags: string[]) => void;
  placeholder?: string;
}

/**
 * TagInput — free-form token entry mirroring the web frontend's
 * `Select mode="tags"`: comma or Enter commits a tag, suggestions come from
 * the backend tag list (prefix match, case-insensitive, selected excluded),
 * Backspace on an empty input removes the last tag.
 */
export function TagInput({ value, onChange, placeholder }: TagInputProps) {
  const { t } = useTranslation();
  const { open, setOpen, ref } = usePopover();
  const [input, setInput] = useState("");
  const [focused, setFocused] = useState(false);
  const [allTags, setAllTags] = useState<string[]>([]);
  const [activeIndex, setActiveIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    void (async () => {
      try {
        setAllTags(await fetchTags());
      } catch {
        // suggestions are best-effort
      }
    })();
  }, []);

  const q = input.trim().toLowerCase();
  const suggestions = allTags.filter(
    (tag) => !value.includes(tag) && (!q || tag.toLowerCase().startsWith(q)),
  );
  const exactDup = value.some((tag) => tag.toLowerCase() === q);
  const canCreate = q.length > 0 && !exactDup && !suggestions.some((s) => s.toLowerCase() === q);
  // The "create" row sits on top of the suggestion rows.
  const rowCount = suggestions.length + (canCreate ? 1 : 0);
  const showDropdown = open && focused && rowCount > 0;

  useEffect(() => setActiveIndex(0), [input, open]);

  const commit = (tag: string) => {
    const clean = tag.trim();
    if (!clean) return;
    if (!value.some((v) => v.toLowerCase() === clean.toLowerCase())) {
      onChange([...value, clean]);
    }
    setInput("");
  };

  const remove = (tag: string) => onChange(value.filter((v) => v !== tag));

  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      if (showDropdown) {
        const tag = canCreate
          ? activeIndex === 0
            ? input
            : suggestions[activeIndex - 1]
          : suggestions[activeIndex];
        if (tag) commit(tag);
      } else {
        commit(input);
      }
    } else if (e.key === ",") {
      e.preventDefault();
      commit(input);
    } else if (e.key === "Backspace" && !input && value.length) {
      remove(value[value.length - 1]);
    } else if (e.key === "ArrowDown" && showDropdown) {
      e.preventDefault();
      setActiveIndex((i) => Math.min(rowCount - 1, i + 1));
    } else if (e.key === "ArrowUp" && showDropdown) {
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
        {value.map((tag) => (
          <span key={tag} className="pill">
            <span className="pill__text">{tag}</span>
            <button type="button" className="pill__x" onClick={() => remove(tag)}>
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
          {canCreate && (
            <button
              type="button"
              className={`select__option${activeIndex === 0 ? " select__option--active" : ""}`}
              onMouseEnter={() => setActiveIndex(0)}
              onMouseDown={(e) => {
                e.preventDefault();
                commit(input);
              }}
            >
              <span className="select__option-label">{t("form.createTag", { tag: input.trim() })}</span>
            </button>
          )}
          {suggestions.map((tag, i) => {
            const idx = canCreate ? i + 1 : i;
            return (
              <button
                key={tag}
                type="button"
                className={`select__option${idx === activeIndex ? " select__option--active" : ""}`}
                onMouseEnter={() => setActiveIndex(idx)}
                onMouseDown={(e) => {
                  e.preventDefault();
                  commit(tag);
                }}
              >
                <span className="select__option-label">{tag}</span>
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
