import { useEffect, useRef, useState } from "react";
import type { KeyboardEvent } from "react";
import { useFocusWhen } from "./useFocusWhen";

export interface MultiOption {
  value: string;
  label: string;
  // An option that may stay selected but cannot be newly checked, such as a
  // release that has shipped.
  disabled?: boolean;
}

interface Props {
  options: MultiOption[];
  selected: string[];
  onChange: (next: string[]) => void;
  // Label shown when nothing is selected (e.g. "All requirements").
  allLabel?: string;
  title?: string;
  // id goes on the trigger, so a caller's <label htmlFor> names it.
  id?: string;
  // label names the trigger and the search box for assistive technology.
  // The trigger reads "label: summary", which keeps the visible summary in
  // its accessible name.
  label?: string;
  describedBy?: string;
  disabled?: boolean;
}

// MultiSelect is a compact checkbox dropdown: a button shows the selection
// summary ("All …" / one label / "N selected") and opens a checkbox list.
//
// It lives here rather than in one app because both pick several values from
// one control: XTM's Sankey filters since it was written, and TAM's issue
// type filter since #323, which could not import it while it sat in
// xtm/frontend. The panel anchors right; a caller whose control sits near
// the left edge overrides that in its own stylesheet, as both apps do.
export function MultiSelect({
  options,
  selected,
  onChange,
  allLabel = "All",
  title,
  id,
  label,
  describedBy,
  disabled = false,
}: Props) {
  const [open, setOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const focusRef = useFocusWhen<HTMLInputElement>(open);
  const [query, setQuery] = useState("");
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDoc = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [open]);

  // Reset the filter each time the panel closes so it reopens showing everything.
  useEffect(() => {
    if (!open) setQuery("");
  }, [open]);

  const q = query.trim().toLowerCase();
  const shown = q
    ? options.filter((o) => o.label.toLowerCase().includes(q))
    : options;

  // Escape shuts the panel and hands focus back to the trigger, so a
  // keyboard user is not left on a checkbox that just disappeared.
  function onKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    if (e.key !== "Escape" || !open) return;
    e.stopPropagation();
    setOpen(false);
    triggerRef.current?.focus();
  }

  const sel = new Set(selected);
  function toggle(v: string) {
    const next = new Set(sel);
    if (next.has(v)) next.delete(v);
    else next.add(v);
    onChange([...next]);
  }

  const summary =
    selected.length === 0
      ? allLabel
      : selected.length === 1
        ? (options.find((o) => o.value === selected[0])?.label ?? selected[0])
        : `${selected.length} selected`;

  return (
    <div className="multiselect" ref={ref} onKeyDown={onKeyDown}>
      <button
        type="button"
        className="multiselect-btn"
        ref={triggerRef}
        id={id}
        aria-label={label ? `${label}: ${summary}` : undefined}
        aria-describedby={describedBy}
        aria-haspopup="true"
        aria-expanded={open}
        disabled={disabled}
        onClick={() => setOpen((o) => !o)}
        title={title}
      >
        <span className="multiselect-summary">{summary}</span>
        <span className="multiselect-caret" aria-hidden="true">
          ▾
        </span>
      </button>
      {open && (
        <div className="multiselect-panel">
          {options.length > 8 && (
            <input
              className="multiselect-search"
              type="search"
              ref={focusRef}
              aria-label={label ? `Search ${label}` : "Search"}
              placeholder="Search…"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
          )}
          {selected.length > 0 && (
            <button
              type="button"
              className="link-btn multiselect-clear"
              onClick={() => onChange([])}
            >
              Clear all
            </button>
          )}
          <ul className="multiselect-list">
            {shown.map((o) => (
              <li key={o.value}>
                <label>
                  <input
                    type="checkbox"
                    checked={sel.has(o.value)}
                    disabled={o.disabled && !sel.has(o.value)}
                    onChange={() => toggle(o.value)}
                  />
                  <span>{o.label}</span>
                </label>
              </li>
            ))}
            {options.length === 0 && <li className="muted">No options</li>}
            {options.length > 0 && shown.length === 0 && (
              <li className="muted">No matches</li>
            )}
          </ul>
        </div>
      )}
    </div>
  );
}
