export interface SortField {
  value: string;
  label: string;
}

interface Props {
  fields: SortField[];
  field: string;
  desc: boolean;
  onChange: (field: string, desc: boolean) => void;
}

// SortControl is a compact field-select plus an ascending/descending toggle,
// used by the Requirements / Preconditions / Bugs panels and the container
// picker. It owns no state — the parent holds (field, desc) and re-sorts.
export function SortControl({ fields, field, desc, onChange }: Props) {
  return (
    <div className="sort-control">
      <span className="sort-control-label muted">Sort</span>
      <select
        className="sort-field"
        value={field}
        onChange={(e) => onChange(e.target.value, desc)}
        title="Sort field"
      >
        {fields.map((f) => (
          <option key={f.value} value={f.value}>
            {f.label}
          </option>
        ))}
      </select>
      <button
        type="button"
        className="btn sort-dir"
        onClick={() => onChange(field, !desc)}
        title={desc ? "Descending — click for ascending" : "Ascending — click for descending"}
        aria-label="Toggle sort direction"
      >
        {desc ? "↓" : "↑"}
      </button>
    </div>
  );
}
