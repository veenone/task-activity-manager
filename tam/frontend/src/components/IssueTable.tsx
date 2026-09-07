import { useRef } from "react";
import type { CSSProperties } from "react";
import { GRID_COLUMNS } from "../api";
import type { Issue, SortColumn } from "../api";
import { TypeChip } from "./TypeChip";
import { statusClass } from "../lib/statusClass";
import { keyColumnWidth } from "../lib/keyColumn";

// The pending dot shares the key cell, so the key column has to be wide
// enough for the longest key plus the dot and its margin.
const PENDING_DOT_PX = 20;

interface Props {
  issues: Issue[];
  // The instance's own word for the sub-task level, for the type chip.
  subtaskLabel?: string;
  selectedKey: string;
  onSelect: (key: string) => void;
  // The column the store ordered these rows by, "" for its default rank
  // order, and which way. Sorting is server-side (see api.ts), so the table
  // reports a click and renders whatever comes back.
  sort: SortColumn | "";
  desc: boolean;
  onSort: (column: SortColumn) => void;
}

// IssueTable renders one page of issues in the mockup's seven columns.
//
// Two things this used to get wrong, both of them layout:
//
// The key column was a fixed 84px, about eleven characters. Jira DC allows a
// ten-character project key, so a legal key overflowed, wrapped inside a 34px
// row, and was painted over by the type chip's opaque background, with no
// ellipsis to say so. It is now sized from the keys on the page (see
// lib/keyColumn.ts) and handed to every row as one custom property, because
// each row is its own grid container and a per-row max-content track would
// stop the columns lining up.
//
// The header used to be a sibling of the scrolling body, which meant its
// own position:sticky never fired (its scrolling ancestor was not its
// sibling) and its columns sat a scrollbar's width off the body's whenever
// the list scrolled. It now lives inside the scroller, so it sticks, and
// both grids are laid out in the same content box.
//
// The rows are no longer virtualised. A page is 25 rows and the overscan was
// 30, so the virtualiser rendered every row of every page anyway: it bought
// nothing and cost absolutely positioned rows, which is what forced the
// header outside the scroller in the first place.
export function IssueTable({ issues, subtaskLabel, selectedKey, onSelect, sort, desc, onSort }: Props) {
  const bodyRef = useRef<HTMLDivElement>(null);

  // moveFocus walks the roving tabindex to a neighbouring row.
  function moveFocus(from: number, delta: number) {
    bodyRef.current?.querySelector<HTMLElement>(`[data-row-index="${from + delta}"]`)?.focus();
  }

  const keyWidth = keyColumnWidth(issues.map((i) => i.key), PENDING_DOT_PX);
  const sortedLabel = GRID_COLUMNS.find((c) => c.id === sort)?.label;

  return (
    <div
      className="issue-table"
      role="grid"
      aria-label="Issues"
      aria-multiselectable="false"
      aria-rowcount={issues.length + 1}
      style={{ "--issue-key-w": keyWidth } as CSSProperties}
    >
      {/* The order was otherwise invisible, and rank order is the whole point
          of a backlog: a row's position means something, so say so. */}
      <p className="issue-order muted small" role="status">
        {sortedLabel
          ? `Sorted by ${sortedLabel} ${desc ? "descending" : "ascending"}, drafts first. Click the column again to clear.`
          : "In Jira rank order, drafts first."}
      </p>
      <div className="issue-body" ref={bodyRef} role="rowgroup">
        <div className="issue-row issue-head" role="row" aria-rowindex={1}>
          {GRID_COLUMNS.map((col) => {
            const active = sort === col.id;
            return (
              <span
                key={col.id}
                role="columnheader"
                // aria-sort belongs on the header cell, not the button, and
                // "none" on the others is what says they are sortable but
                // not currently sorted.
                aria-sort={active ? (desc ? "descending" : "ascending") : "none"}
              >
                <button
                  type="button"
                  className={`issue-sort${active ? " issue-sort-active" : ""}`}
                  onClick={() => onSort(col.id)}
                  // The visible label is just the column name, so the button
                  // says what clicking it does. The visible text is contained
                  // in the accessible name, as WCAG 2.5.3 requires.
                  aria-label={`Sort by ${col.label}`}
                  title={`Sort by ${col.label}`}
                >
                  {col.label}
                  <span className="issue-sort-caret" aria-hidden="true">
                    {active ? (desc ? "▾" : "▴") : "▸"}
                  </span>
                </button>
              </span>
            );
          })}
        </div>
        {issues.map((iss, index) => {
          const selected = iss.key === selectedKey;
          return (
            <div
              key={iss.key}
              role="row"
              aria-selected={selected}
              aria-rowindex={index + 2}
              aria-label={`${iss.key} ${iss.summary}`}
              data-row-index={index}
              className={`issue-row${selected ? " issue-row-selected" : index % 2 ? " issue-row-alt" : ""}`}
              onClick={() => onSelect(iss.key)}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  onSelect(iss.key);
                } else if (e.key === "ArrowDown") {
                  e.preventDefault();
                  moveFocus(index, 1);
                } else if (e.key === "ArrowUp") {
                  e.preventDefault();
                  moveFocus(index, -1);
                }
              }}
              tabIndex={selected || (selectedKey === "" && index === 0) ? 0 : -1}
            >
              <span role="gridcell" className="issue-key" title={iss.key}>
                {iss.key}
                {iss.pending && <span className="pending-dot" role="img" aria-label="Pending changes" title="Has pending changes" />}
              </span>
              <span role="gridcell"><TypeChip type={iss.type} subtaskLabel={subtaskLabel} /></span>
              <span role="gridcell" className="issue-summary" title={iss.summary}>{iss.summary}</span>
              <span role="gridcell">
                {iss.draft
                  ? <span className="chip chip-draft">Draft</span>
                  : <span className={`chip chip-status chip-status-${statusClass(iss.status)}`} title={iss.status}>{iss.status}</span>}
              </span>
              <span role="gridcell" title={iss.assignee || undefined}>{iss.assignee || "-"}</span>
              <span role="gridcell" title={iss.sprintName}>{iss.sprintName || iss.sprintId || "-"}</span>
              <span role="gridcell">{iss.storyPoints ?? "-"}</span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
