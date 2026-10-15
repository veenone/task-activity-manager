import type { CSSProperties } from "react";
import { formatDate } from "../../dates";
import type { TestCase } from "../../api";
import type { SortCol, ColKey } from "./columns";

// renderCell returns the table cell for one column of one Test row. `style`
// carries the column's fixed width so the cell aligns as a flex item under the
// virtualized layout (P2).
export function renderCell(
  key: ColKey,
  t: TestCase,
  hasPending: boolean,
  callsOthers: boolean,
  style: CSSProperties,
) {
  switch (key) {
    case "key":
      return (
        <td key="key" className="mono" style={style}>
          {hasPending && (
            <span className="row-dirty-dot" title="Pending edits">
              ●
            </span>
          )}
          {t.key}
          {callsOthers && (
            <span
              className="row-calls-badge"
              title="This test calls another test in its steps"
            >
              ⮡
            </span>
          )}
        </td>
      );
    case "summary":
      return (
        <td key="summary" className="summary-cell" style={style}>
          {t.summary}
        </td>
      );
    case "status":
      return (
        <td key="status" style={style}>
          {t.status ? (
            <span className="status-pill">{t.status}</span>
          ) : (
            <span className="muted">(none)</span>
          )}
        </td>
      );
    case "priority":
      return (
        <td key="priority" style={style}>
          {t.priority || "(none)"}
        </td>
      );
    case "labels":
      return (
        <td key="labels" className="labels-cell" style={style}>
          {t.labels && t.labels.length > 0 ? (
            t.labels.map((l) => (
              <span key={l} className="label-chip">
                {l}
              </span>
            ))
          ) : (
            <span className="muted">(none)</span>
          )}
        </td>
      );
    case "components":
      return (
        <td key="components" className="labels-cell" style={style}>
          {t.components && t.components.length > 0 ? (
            t.components.map((c) => (
              <span key={c} className="component-chip">
                {c}
              </span>
            ))
          ) : (
            <span className="muted">(none)</span>
          )}
        </td>
      );
    case "execType":
      return (
        <td key="execType" style={style}>
          {t.execType || "(none)"}
        </td>
      );
    case "updated":
      return (
        <td key="updated" className="muted" style={style}>
          {formatDate(t.updated)}
        </td>
      );
  }
}

export function SortHeader({
  col,
  label,
  sortBy,
  desc,
  onSort,
  style,
}: {
  col: SortCol;
  label: string;
  sortBy: SortCol;
  desc: boolean;
  onSort: (c: SortCol) => void;
  style: CSSProperties;
}) {
  const active = sortBy === col;
  return (
    <th scope="col"
      className="sortable"
      style={style}
      aria-sort={active ? (desc ? "descending" : "ascending") : "none"}
    >
      <button type="button" className="sort-btn" onClick={() => onSort(col)}>
        {label}
        <span className="sort-caret" aria-hidden="true">
          {active ? (desc ? " ▼" : " ▲") : ""}
        </span>
      </button>
    </th>
  );
}
