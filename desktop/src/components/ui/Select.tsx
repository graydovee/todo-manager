import { useEffect, useRef, useState } from "react";
import { usePopover } from "./usePopover";
import { ChevronDownIcon, CheckIcon } from "./icons";

export interface SelectOption {
  value: string;
  label: string;
  /** Optional dot color (CSS color value) shown before the label. */
  color?: string;
}

interface SelectProps {
  options: SelectOption[];
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
}

/**
 * Select — single-select dropdown. Open on click / Enter / ArrowDown, close on
 * outside click or Escape, ArrowUp/ArrowDown to move, Enter to commit. The
 * selected option shows a checkmark; options may carry a colored dot.
 */
export function Select({ options, value, onChange, placeholder }: SelectProps) {
  const { open, setOpen, ref } = usePopover();
  const selectedIndex = options.findIndex((o) => o.value === value);
  const [activeIndex, setActiveIndex] = useState(Math.max(0, selectedIndex));
  const optionRefs = useRef<(HTMLButtonElement | null)[]>([]);

  useEffect(() => {
    if (open) {
      setActiveIndex(Math.max(0, selectedIndex));
    }
  }, [open, selectedIndex]);

  useEffect(() => {
    if (open) optionRefs.current[activeIndex]?.scrollIntoView({ block: "nearest" });
  }, [open, activeIndex]);

  const commit = (v: string) => {
    onChange(v);
    setOpen(false);
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (!open) {
      if (e.key === "Enter" || e.key === " " || e.key === "ArrowDown") {
        e.preventDefault();
        setOpen(true);
      }
      return;
    }
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIndex((i) => Math.min(options.length - 1, i + 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIndex((i) => Math.max(0, i - 1));
    } else if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      const opt = options[activeIndex];
      if (opt) commit(opt.value);
    } else if (e.key === "Tab") {
      setOpen(false);
    }
  };

  const selected = selectedIndex >= 0 ? options[selectedIndex] : null;

  return (
    <div className="select" ref={ref} onKeyDown={onKeyDown}>
      <button
        type="button"
        className={`select__trigger${open ? " select__trigger--open" : ""}`}
        onClick={() => setOpen(!open)}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        {selected?.color && <span className="dot" style={{ background: selected.color }} />}
        <span className={`select__label${selected ? "" : " select__label--placeholder"}`}>
          {selected ? selected.label : (placeholder ?? "")}
        </span>
        <ChevronDownIcon size={14} className="select__chevron" />
      </button>
      {open && (
        <div className="select__dropdown" role="listbox">
          {options.map((opt, i) => (
            <button
              key={opt.value}
              type="button"
              role="option"
              aria-selected={opt.value === value}
              ref={(el) => {
                optionRefs.current[i] = el;
              }}
              className={`select__option${i === activeIndex ? " select__option--active" : ""}${opt.value === value ? " select__option--selected" : ""}`}
              onMouseEnter={() => setActiveIndex(i)}
              onClick={() => commit(opt.value)}
            >
              {opt.color && <span className="dot" style={{ background: opt.color }} />}
              <span className="select__option-label">{opt.label}</span>
              <CheckIcon size={14} className="select__check" />
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
