import type { ReactElement } from "react";
import type { FieldSpec } from "../api";

// FORM_OWNED_FIELDS are the create-meta ids the New issue dialog carries
// itself. The Go side never offers them; this is the second fence, so a
// backend that did would still not draw a second Parent or Summary input
// under the one the dialog already has.
export const FORM_OWNED_FIELDS = new Set([
  "project", "issuetype", "summary", "description", "priority", "labels", "assignee", "reporter", "parent",
]);

// splitMetaFields drops the form's own fields and splits the rest into what
// Jira requires, shown at once, and what it merely allows, kept behind More
// fields.
export function splitMetaFields(specs: FieldSpec[]): { required: FieldSpec[]; optional: FieldSpec[] } {
  const own = specs.filter((s) => !FORM_OWNED_FIELDS.has(s.id));
  return { required: own.filter((s) => s.required), optional: own.filter((s) => !s.required) };
}

export interface MetaInputShared {
  id: string;
  className: string;
  "aria-required"?: boolean;
  "aria-invalid"?: boolean;
}

export interface MetaInputProps {
  spec: FieldSpec;
  value: string;
  onChange: (v: string) => void;
  shared: MetaInputShared;
}

// OptionInput is any field whose create-meta listed allowed values. An array
// field gets a multi-select, because a Jira array takes more than one value;
// the chosen ids are joined with a comma, which the Go side splits.
function OptionInput({ spec, value, onChange, shared }: MetaInputProps) {
  const isList = spec.type === "array";
  const selected = value === "" ? [] : value.split(",");
  return (
    <span className="edit-cell">
      <select
        {...shared}
        multiple={isList}
        size={isList ? Math.min(spec.allowedValues.length, 4) : undefined}
        value={isList ? selected : value}
        onChange={(e) =>
          onChange(isList ? Array.from(e.target.selectedOptions, (o) => o.value).join(",") : e.target.value)
        }
      >
        {!isList && <option value="">Select a {spec.name.toLowerCase()}</option>}
        {spec.allowedValues.map((o) => (
          <option key={o.id} value={o.id}>{o.value}</option>
        ))}
      </select>
      {isList && <span className="muted small">Pick one or more.</span>}
    </span>
  );
}

// TextInput is everything without options: text, a number, a date, a
// username, a comma list. The Go side shapes each from the field's schema.
function TextInput({ spec, value, onChange, shared }: MetaInputProps) {
  const dated = spec.type === "date" || spec.type === "datetime";
  return (
    <span className="edit-cell">
      <input
        {...shared}
        type={dated ? "date" : "text"}
        inputMode={spec.type === "number" ? "decimal" : undefined}
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
      {spec.type === "array" && <span className="muted small">A comma list.</span>}
      {spec.type === "user" && <span className="muted small">A Jira username.</span>}
    </span>
  );
}

function TextareaInput({ value, onChange, shared }: MetaInputProps) {
  return (
    <span className="edit-cell">
      <textarea {...shared} rows={3} value={value} onChange={(e) => onChange(e.target.value)} />
    </span>
  );
}

// META_INPUTS picks the input for a field type that has no options. It is a
// table rather than a branch so a type gets a richer input by replacing its
// one entry: the long-text Write and Preview input takes over "textarea"
// here and touches nothing else.
export const META_INPUTS: Record<string, (p: MetaInputProps) => ReactElement> = {
  textarea: TextareaInput,
};

// MetaField renders one create-meta field: its label, a required mark, and
// the input its type and options call for.
export function MetaField({
  spec,
  value,
  invalid,
  onChange,
}: {
  spec: FieldSpec;
  value: string;
  invalid: boolean;
  onChange: (v: string) => void;
}) {
  const id = `meta-${spec.id}`;
  const shared: MetaInputShared = {
    id,
    className: "detail-input",
    "aria-required": spec.required || undefined,
    "aria-invalid": invalid || undefined,
  };
  const Input = spec.allowedValues.length > 0 ? OptionInput : META_INPUTS[spec.type] ?? TextInput;
  return (
    <div className="edit-row">
      <label className="muted small" htmlFor={id}>
        {spec.name}
        {spec.required && <span className="field-required" aria-hidden="true"> *</span>}
      </label>
      <Input spec={spec} value={value} onChange={onChange} shared={shared} />
    </div>
  );
}
