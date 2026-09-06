import { useEffect, useState } from "react";

// useDebounced returns value after it has stopped changing for delay ms, so
// rapid changes (typing, quick edits) do not trigger a query on every one of
// them. resetKey cuts the wait short: while the stored value belongs to an
// older key the incoming one is returned straight away, so a switch to a new
// context (a different profile, a newly picked file) never applies a
// debounced value meant for the old one.
export function useDebounced<T>(value: T, delay: number, resetKey: unknown): T {
  const [stored, setStored] = useState({ key: resetKey, value });
  useEffect(() => {
    const t = setTimeout(() => setStored({ key: resetKey, value }), delay);
    return () => clearTimeout(t);
  }, [value, delay, resetKey]);
  return stored.key === resetKey ? stored.value : value;
}
