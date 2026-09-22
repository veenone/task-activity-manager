import { useEffect, useId, useRef, useState } from "react";
import type { KeyboardEvent, ReactElement } from "react";
import { RichTextField } from "@agile-suite/core";
import type { RichFormat } from "@agile-suite/core";
import { BrowserOpenURL } from "../api";
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

// MultiSelect is the control for a Jira field that takes several values. It
// was a native <select multiple>: a scrolling box that needs ctrl-click to
// add a second value, and that shows the chosen ones only while they happen
// to be scrolled into view (issue #65 item 3). This is the listbox the
// assignee picker beside it already uses, marked aria-multiselectable, with
// the chosen values kept outside the list where they can be read and
// removed without opening it.
//
// The value it stores is unchanged: the chosen ids joined with a comma,
// which the Go side splits.
function MultiSelect({ spec, value, onChange, shared }: MetaInputProps) {
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const rootRef = useRef<HTMLSpanElement>(null);
  const listId = useId();
  const chosen = value === "" ? [] : value.split(",");
  const labelOf = (id: string) => spec.allowedValues.find((o) => o.id === id)?.value ?? id;

  // A click anywhere else closes the list, the same as the assignee picker.
  useEffect(() => {
    if (!open) return;
    function onDown(e: MouseEvent) {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [open]);

  function toggle(id: string) {
    onChange((chosen.includes(id) ? chosen.filter((c) => c !== id) : [...chosen, id]).join(","));
  }

  function onKeyDown(e: KeyboardEvent) {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      if (!open) {
        setOpen(true);
        setActive(0);
        return;
      }
      const step = e.key === "ArrowDown" ? 1 : -1;
      setActive(Math.max(0, Math.min(spec.allowedValues.length - 1, active + step)));
    } else if ((e.key === "Enter" || e.key === " ") && open && spec.allowedValues[active]) {
      // Enter on a closed trigger is the form's submit, which is what a
      // button in a form does and what this must not take away.
      e.preventDefault();
      toggle(spec.allowedValues[active].id);
    } else if (e.key === "Escape" && open) {
      // Stopped here, or the dialog this field sits in closes with it.
      e.preventDefault();
      e.stopPropagation();
      setOpen(false);
    }
  }

  return (
    <span className="edit-cell">
      <span className="multi-select" ref={rootRef}>
        <button
          {...shared}
          type="button"
          aria-haspopup="listbox"
          aria-expanded={open}
          aria-controls={listId}
          className="detail-input multi-select-trigger"
          onClick={() => setOpen((o) => !o)}
          onKeyDown={onKeyDown}
        >
          {chosen.length === 0 ? `Select a ${spec.name.toLowerCase()}` : `${chosen.length} chosen`}
          <span className="multi-select-caret" aria-hidden="true">&#9662;</span>
        </button>
        {open && (
          <ul className="multi-select-list" id={listId} role="listbox" aria-multiselectable aria-label={spec.name}>
            {spec.allowedValues.map((o, i) => (
              <li key={o.id}>
                <button
                  type="button"
                  role="option"
                  aria-selected={chosen.includes(o.id)}
                  className={`multi-select-option${i === active ? " multi-select-option-active" : ""}`}
                  onMouseEnter={() => setActive(i)}
                  onClick={() => toggle(o.id)}
                >
                  <span className="multi-select-mark" aria-hidden="true">{chosen.includes(o.id) ? "✓" : ""}</span>
                  <span>{o.value}</span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </span>
      {chosen.length > 0 && (
        <span className="multi-select-values">
          {chosen.map((id) => (
            <span key={id} className="chip multi-select-value">
              {labelOf(id)}
              <button
                type="button"
                className="btn btn-ghost multi-select-remove"
                aria-label={`Remove ${labelOf(id)}`}
                onClick={() => toggle(id)}
              >
                ✕
              </button>
            </span>
          ))}
        </span>
      )}
      <span className="muted small">Pick one or more.</span>
    </span>
  );
}

// OptionInput is a field whose create-meta listed allowed values and takes
// one of them.
function OptionInput({ spec, value, onChange, shared }: MetaInputProps) {
  return (
    <span className="edit-cell">
      <select {...shared} value={value} onChange={(e) => onChange(e.target.value)}>
        <option value="">Select a {spec.name.toLowerCase()}</option>
        {spec.allowedValues.map((o) => (
          <option key={o.id} value={o.id}>{o.value}</option>
        ))}
      </select>
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

// LongTextInput is Jira's "textarea" create-meta type: a long text field
// (Acceptance criteria, Steps to reproduce) that gets the same Write and
// Preview a description does. Each field holds its own syntax choice,
// starting at "auto" the way a fresh RichTextField always does, because two
// long-text fields on one draft have no reason to share a pick. onOpenLink
// reaches the browser the same way the detail panel's own description does:
// directly, since this field carries no profile context to route it through.
export function LongTextInput({ value, onChange, shared }: MetaInputProps) {
  const [format, setFormat] = useState<RichFormat | "auto">("auto");
  return (
    <RichTextField
      value={value}
      onChange={onChange}
      format={format}
      onFormatChange={setFormat}
      onOpenLink={BrowserOpenURL}
      textarea={shared}
      minRows={3}
    />
  );
}

// META_INPUTS picks the input for a field type that has no options. It is a
// table rather than a branch so a type gets a richer input by replacing its
// one entry.
export const META_INPUTS: Record<string, (p: MetaInputProps) => ReactElement> = {
  textarea: LongTextInput,
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
  const Input = spec.allowedValues.length === 0
    ? META_INPUTS[spec.type] ?? TextInput
    : spec.type === "array" ? MultiSelect : OptionInput;
  return (
    <div className={Input === LongTextInput ? "edit-row edit-row-stacked" : "edit-row"}>
      <label className="muted small" htmlFor={id}>
        {spec.name}
        {spec.required && <span className="field-required" aria-hidden="true"> *</span>}
      </label>
      <Input spec={spec} value={value} onChange={onChange} shared={shared} />
    </div>
  );
}
