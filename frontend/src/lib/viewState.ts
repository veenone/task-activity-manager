// frontend/src/lib/viewState.ts
import { useCallback, useState } from "react";

// Module-level store: survives component unmount (so a view restores its state
// when you switch away and back), but NOT a full app reload. Keyed by
// "<profileId>:<viewKey>:<fieldKey>".
const store = new Map<string, unknown>();

function k(profileId: string, viewKey: string, fieldKey: string): string {
  return `${profileId}:${viewKey}:${fieldKey}`;
}

// useViewState behaves like useState but persists the value in the module store,
// so leaving and returning to a view restores the field. Pass the active
// profileId, a stable viewKey (e.g. "bugs"), and a fieldKey (e.g. "selected").
export function useViewState<T>(
  profileId: string,
  viewKey: string,
  fieldKey: string,
  initial: T,
): [T, (next: T | ((prev: T) => T)) => void] {
  const key = k(profileId, viewKey, fieldKey);
  const [value, setValue] = useState<T>(() =>
    store.has(key) ? (store.get(key) as T) : initial,
  );
  const set = useCallback(
    (next: T | ((prev: T) => T)) => {
      setValue((prev) => {
        const resolved =
          typeof next === "function" ? (next as (p: T) => T)(prev) : next;
        store.set(key, resolved);
        return resolved;
      });
    },
    [key],
  );
  return [value, set];
}

// clearViewState drops all stored state for a profile. Call this when the active
// profile changes so a new profile does not inherit stale selections.
export function clearViewState(profileId: string): void {
  const prefix = `${profileId}:`;
  for (const key of [...store.keys()]) {
    if (key.startsWith(prefix)) store.delete(key);
  }
}
