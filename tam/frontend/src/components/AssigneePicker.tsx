import { useEffect, useId, useRef, useState } from "react";
import { useUserSearch } from "../queries/people";
import { useDebounced } from "../lib/useDebounced";

const SEARCH_DELAY_MS = 200;

interface Props {
  profileId: string;
  // The username the form holds, "" for unassigned.
  value: string;
  onChange: (username: string) => void;
  id: string;
  // What to show when the picker has a username it has not resolved to a
  // person yet, e.g. the display name a sync left on the issue.
  fallbackLabel?: string;
  disabled?: boolean;
}

// AssigneePicker finds a person the way Jira's own field does: type part of a
// name and pick from who comes back. It matters that the value it stores is
// the username, not the display name: the write path sends
// {"assignee": {"name": …}}, and the grid only ever had the display name, so
// a free-text field could not produce a value Jira would accept unless the
// two happened to be identical.
//
// The list is served by App.SearchUsers, which asks Jira, caches the answer in
// tam.db, and falls back to that cache when Jira cannot be reached, so this
// keeps working offline and on a demo profile.
export function AssigneePicker({ profileId, value, onChange, id, fallbackLabel, disabled }: Props) {
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const rootRef = useRef<HTMLDivElement>(null);
  const listId = useId();
  const search = useDebounced(query, SEARCH_DELAY_MS, profileId);
  const users = useUserSearch(profileId, open ? search : "");
  const results = users.data ?? [];
  // When nobody can be looked up at all, the control degrades to the free-text
  // username field it used to be, the same way the priority field does. A
  // picker that cannot reach its list must not be the reason an issue cannot
  // be assigned.
  const freeText = users.isError;

  // The label for a value the user has not just picked: whatever the last
  // search knew about it, else what the caller could tell us, else the
  // username itself.
  const known = results.find((u) => u.name === value);
  const label = known?.displayName || fallbackLabel || value;

  // A click anywhere else closes the list. Escape does too, from the input.
  useEffect(() => {
    if (!open) return;
    function onDown(e: MouseEvent) {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [open]);

  useEffect(() => {
    setActive(0);
  }, [search, open]);

  function choose(username: string, display: string) {
    onChange(username);
    setQuery(display);
    setOpen(false);
  }

  function onKeyDown(e: React.KeyboardEvent) {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      if (!open) {
        setOpen(true);
        return;
      }
      const next = e.key === "ArrowDown" ? active + 1 : active - 1;
      setActive(Math.max(0, Math.min(results.length - 1, next)));
    } else if (e.key === "Enter" && open && results[active]) {
      e.preventDefault();
      choose(results[active].name, results[active].displayName);
    } else if (e.key === "Escape" && open) {
      e.preventDefault();
      e.stopPropagation();
      setOpen(false);
    }
  }

  return (
    <div className="assignee-picker" ref={rootRef}>
      <div className="assignee-field">
        <input
          id={id}
          className="detail-input"
          type="text"
          role="combobox"
          autoComplete="off"
          aria-expanded={open}
          aria-controls={listId}
          aria-autocomplete="list"
          disabled={disabled}
          placeholder={value ? label : "Unassigned"}
          value={open ? query : value ? label : ""}
          onFocus={() => setOpen(true)}
          onChange={(e) => {
            setQuery(e.target.value);
            setOpen(true);
            if (freeText) onChange(e.target.value.trim());
          }}
          onKeyDown={onKeyDown}
        />
        {value !== "" && !disabled && (
          <button
            type="button"
            className="btn btn-ghost assignee-clear"
            title="Unassign"
            aria-label="Unassign"
            onClick={() => {
              onChange("");
              setQuery("");
              setOpen(false);
            }}
          >
            ✕
          </button>
        )}
      </div>
      {open && (
        <ul className="assignee-list" id={listId} role="listbox" aria-label="People">
          {users.isPending ? (
            <li className="muted small assignee-note">Looking up people</li>
          ) : users.isError ? (
            <li className="muted small assignee-note">
              Nobody could be looked up ({users.error.message}). Type a Jira username instead.
            </li>
          ) : results.length === 0 ? (
            <li className="muted small assignee-note">Nobody matches that.</li>
          ) : (
            results.map((u, i) => (
              <li key={u.name}>
                <button
                  type="button"
                  role="option"
                  aria-selected={u.name === value}
                  className={`assignee-option${i === active ? " assignee-option-active" : ""}`}
                  onMouseEnter={() => setActive(i)}
                  onClick={() => choose(u.name, u.displayName)}
                >
                  <span>{u.displayName}</span>
                  <span className="muted small">{u.name}</span>
                </button>
              </li>
            ))
          )}
        </ul>
      )}
    </div>
  );
}
