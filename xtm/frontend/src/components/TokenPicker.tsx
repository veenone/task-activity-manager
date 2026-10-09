import { useId, useMemo, useRef, useState } from "react";
import type { ClipboardEvent, FocusEvent, KeyboardEvent } from "react";

export interface TokenPickerProps {
  value: string[];
  onChange: (next: string[]) => void;
  suggestions: string[];
  allowCreate: boolean;
  onCreate?: (v: string) => Promise<boolean>;
  validate?: (v: string) => string | null;
  label: string;
  placeholder?: string;
  onBlur?: () => void;
  // How pasted or unconfirmed text splits into values; null keeps it whole
  // (component names contain spaces).
  separator?: RegExp | null;
}

const MAX_LABEL = 255;

// validateLabel is the one rule every label picker uses: Jira rejects labels
// with whitespace and labels longer than 255 characters.
export function validateLabel(v: string): string | null {
  if (/\s/.test(v)) return "A label cannot contain spaces.";
  if ([...v].length > MAX_LABEL) return "A label can be at most 255 characters.";
  return null;
}

interface Option {
  value: string;
  create: boolean;
}

// TokenPicker is a multi-value combobox: chosen values render as removable
// chips, typing filters the suggestions, and with allowCreate an unknown value
// can be added. Membership is case-sensitive because Jira labels and
// components are; filtering is case-insensitive so "smo" finds both "smoke"
// and "Smoke".
export function TokenPicker({
  value,
  onChange,
  suggestions,
  allowCreate,
  onCreate,
  validate,
  label,
  placeholder,
  onBlur,
  separator = /\s+/,
}: TokenPickerProps) {
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const [error, setError] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);
  const listId = useId();
  const errorId = useId();

  const options = useMemo<Option[]>(() => {
    const q = query.trim();
    if (!q) return [];
    const chosen = new Set(value);
    const lower = q.toLowerCase();
    const matches = suggestions
      .filter((s) => !chosen.has(s) && s.toLowerCase().includes(lower))
      .map((s) => ({ value: s, create: false }));
    const exact = suggestions.includes(q) || chosen.has(q);
    if (allowCreate && !exact) matches.push({ value: q, create: true });
    return matches;
  }, [query, suggestions, value, allowCreate]);

  async function add(raw: string[], create: boolean) {
    const next = [...value];
    for (const v of raw) {
      const msg = validate?.(v) ?? null;
      if (msg) {
        setError(msg);
        return;
      }
      if (next.includes(v)) continue;
      const known = suggestions.includes(v);
      if (!known && !allowCreate) return;
      if (!known && create && onCreate && !(await onCreate(v))) return;
      next.push(v);
    }
    setError("");
    setQuery("");
    setActive(0);
    if (next.length !== value.length) onChange(next);
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setOpen(true);
      setActive((i) => Math.min(i + 1, Math.max(options.length - 1, 0)));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive((i) => Math.max(i - 1, 0));
    } else if (e.key === "Enter") {
      e.preventDefault();
      const o = options[active];
      if (o) void add([o.value], o.create);
    } else if (e.key === "Escape" && showList) {
      // Close only the list. Without this the Escape reaches the Modal around
      // a bulk dialog and closes it, discarding every pick.
      e.preventDefault();
      e.stopPropagation();
      setOpen(false);
    } else if (e.key === "Backspace" && query === "" && value.length > 0) {
      onChange(value.slice(0, -1));
    }
  }

  const splitText = (t: string) =>
    separator ? t.split(separator).filter(Boolean) : t.trim() ? [t.trim()] : [];

  function onPaste(e: ClipboardEvent<HTMLInputElement>) {
    if (!separator) return;
    const parts = splitText(e.clipboardData.getData("text"));
    if (parts.length < 2) return;
    e.preventDefault();
    void add(parts, true);
  }

  function onWrapperBlur(e: FocusEvent<HTMLDivElement>) {
    if (e.currentTarget.contains(e.relatedTarget as Node | null)) return;
    setOpen(false);
    // Text typed but never confirmed with Enter would otherwise vanish, and a
    // "Replace all" applied without it would clear the field.
    const pending = splitText(query);
    if (pending.length > 0) void add(pending, true);
    onBlur?.();
  }

  const showList = open && options.length > 0;

  return (
    <div className="token-picker" onBlur={onWrapperBlur}>
      <div className="token-picker-field">
        {value.map((v) => (
          <span key={v} className="token-chip">
            {v}
            <button
              type="button"
              className="token-chip-remove"
              aria-label={`Remove ${v}`}
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => {
                onChange(value.filter((x) => x !== v));
                inputRef.current?.focus();
              }}
            >
              ×
            </button>
          </span>
        ))}
        <input
          ref={inputRef}
          className="token-picker-input"
          role="combobox"
          aria-label={label}
          aria-expanded={showList}
          aria-controls={listId}
          aria-autocomplete="list"
          aria-activedescendant={showList ? `${listId}-${active}` : undefined}
          aria-describedby={error ? errorId : undefined}
          value={query}
          placeholder={value.length === 0 ? placeholder : undefined}
          onChange={(e) => {
            setQuery(e.target.value);
            setOpen(true);
            setActive(0);
            setError("");
          }}
          onKeyDown={onKeyDown}
          onPaste={onPaste}
        />
      </div>
      {showList && (
        <ul id={listId} role="listbox" aria-label={label} className="token-picker-list">
          {options.map((o, i) => (
            <li
              key={`${o.create ? "new:" : ""}${o.value}`}
              id={`${listId}-${i}`}
              role="option"
              aria-selected={i === active}
              className={i === active ? "token-option token-option-active" : "token-option"}
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => void add([o.value], o.create)}
              onKeyDown={(e) => {
                if (e.key === "Enter") void add([o.value], o.create);
              }}
            >
              {o.create ? `Create "${o.value}"` : o.value}
            </li>
          ))}
        </ul>
      )}
      {error && (
        <p id={errorId} role="alert" className="token-picker-error">
          {error}
        </p>
      )}
    </div>
  );
}
