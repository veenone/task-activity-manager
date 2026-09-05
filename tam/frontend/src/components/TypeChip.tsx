import { ISSUE_TYPES } from "../api";

// TypeChip is the coloured type pill the grid, the filter bar, and the panel
// share. Unknown types (none should reach the UI) show their raw name.
export function TypeChip({ type }: { type: string }) {
  const t = ISSUE_TYPES.find((x) => x.id === type);
  return <span className={`chip chip-type chip-type-${type || "none"}`}>{t?.short ?? type}</span>;
}
