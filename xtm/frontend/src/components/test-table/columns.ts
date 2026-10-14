import type { CSSProperties } from "react";

// The grid's column model: which columns exist, their fixed widths, sort
// keys, and the visibility and order saved in localStorage.

// Page-size choices for the grid pager. The backend caps a page at 500.
export const PAGE_SIZE_OPTIONS = [50, 100, 200, 500];
export const DEFAULT_PAGE_SIZE = 100;
// Stable empty fallback so an unloaded caller-keys query doesn't mint a new Set
// each render.
export const EMPTY_CALLER_KEYS: ReadonlySet<string> = new Set();

export type SortCol = "key" | "summary" | "status" | "updated";

// Configurable grid columns (FR-11.3). The select-checkbox column is fixed and
// not part of this list. Column visibility + order persist in localStorage.
export type ColKey =
  | "key"
  | "summary"
  | "status"
  | "priority"
  | "labels"
  | "components"
  | "execType"
  | "updated";

export interface ColDef {
  key: ColKey;
  label: string;
  sortCol?: SortCol;
}

export const ALL_COLUMNS: ColDef[] = [
  { key: "key", label: "Key", sortCol: "key" },
  { key: "summary", label: "Summary", sortCol: "summary" },
  { key: "status", label: "Status", sortCol: "status" },
  { key: "priority", label: "Priority" },
  { key: "labels", label: "Labels" },
  { key: "components", label: "Components" },
  { key: "execType", label: "Exec type" },
  { key: "updated", label: "Updated", sortCol: "updated" },
];

// EXEC_TYPE_OPTIONS is the fixed Xray Test Type (execution type) vocabulary
// offered in the Browse filter bar and the bulk-edit / detail editors.
export const EXEC_TYPE_OPTIONS = ["Manual", "Automated", "Generic", "Cucumber"];

// Fixed column geometry. Row virtualization positions rows independently, so
// they only line up with each other and the sticky header when every column
// has a stable width (P2). `summary` grows to absorb slack; the rest are fixed,
// and content that overflows a cell is clipped with an ellipsis. Rows are a
// fixed height so the virtualizer's size estimate is exact.
export const SELECT_W = 40;
export const ROW_HEIGHT = 34;
export const COL_WIDTH: Record<ColKey, number> = {
  key: 130,
  summary: 260,
  status: 130,
  priority: 100,
  labels: 200,
  components: 200,
  execType: 110,
  updated: 130,
};
export function colStyle(key: ColKey): CSSProperties {
  return key === "summary"
    ? { flex: "1 1 0", minWidth: COL_WIDTH.summary }
    : { flex: `0 0 ${COL_WIDTH[key]}px`, width: COL_WIDTH[key] };
}
export const SELECT_STYLE: CSSProperties = { flex: `0 0 ${SELECT_W}px`, width: SELECT_W };

export const COL_LABEL = Object.fromEntries(
  ALL_COLUMNS.map((c) => [c.key, c.label]),
) as Record<ColKey, string>;
export const COL_SORT = Object.fromEntries(
  ALL_COLUMNS.filter((c) => c.sortCol).map((c) => [c.key, c.sortCol]),
) as Partial<Record<ColKey, SortCol>>;

export interface ColState {
  key: ColKey;
  visible: boolean;
}

export const COLUMNS_STORAGE_KEY = "xtm.gridColumns";

export function defaultColumns(): ColState[] {
  return ALL_COLUMNS.map((c) => ({ key: c.key, visible: true }));
}

// loadColumns reads the saved config and reconciles it with the known columns,
// so a column added in a newer version still appears and an unknown one is
// dropped.
export function loadColumns(): ColState[] {
  try {
    const raw = localStorage.getItem(COLUMNS_STORAGE_KEY);
    if (!raw) return defaultColumns();
    const saved = JSON.parse(raw) as ColState[];
    const known = new Set<string>(ALL_COLUMNS.map((c) => c.key));
    const seen = new Set<string>();
    const result: ColState[] = [];
    for (const s of saved) {
      if (known.has(s.key) && !seen.has(s.key)) {
        result.push({ key: s.key, visible: !!s.visible });
        seen.add(s.key);
      }
    }
    for (const c of ALL_COLUMNS) {
      if (!seen.has(c.key)) result.push({ key: c.key, visible: true });
    }
    return result;
  } catch {
    return defaultColumns();
  }
}
