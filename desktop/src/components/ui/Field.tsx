interface FieldProps {
  label: string;
  required?: boolean;
  error?: string;
  children: React.ReactNode;
}

/** Field — a labelled form row with optional inline error. */
export function Field({ label, required, error, children }: FieldProps) {
  return (
    <div className="field">
      <label className="field__label">
        {label}
        {required ? " *" : ""}
      </label>
      {children}
      {error && <div className="field__error">{error}</div>}
    </div>
  );
}
