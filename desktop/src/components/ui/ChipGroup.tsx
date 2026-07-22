interface ChipOption {
  value: string;
  label: string;
}

interface ChipGroupProps {
  options: ChipOption[];
  selected: Set<string>;
  onToggle: (value: string) => void;
}

/** ChipGroup — multi-select toggle chips used by the manage panel filters. */
export function ChipGroup({ options, selected, onToggle }: ChipGroupProps) {
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
