import { useRef, useState } from "react";
import type { CSSProperties } from "react";
import { RowLead, toPlainText } from "@agile-suite/core";
import { GRID_COLUMNS } from "../api";
import type { Issue, SortColumn } from "../api";
import { TypeChip } from "./TypeChip";
import { statusClass } from "../lib/statusClass";
import { keyColumnWidth } from "../lib/keyColumn";
import { drawnParents, familyPlace, subtaskCounts, visibleFamilyIssues } from "../lib/issueFamilies";
import { SubtaskToggle } from "./SubtaskToggle";

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
  const [collapsed, setCollapsed] = useState(new Set<string>());
  const counts = subtaskCounts(issues);
  const rows = visibleFamilyIssues(issues, collapsed);
  function toggleChildren(key: string) {
    if (!collapsed.has(key) && issues.some((i) => i.key === selectedKey && i.type === "subtask" && i.parentKey === key)) onSelect(key);
    setCollapsed((prev) => { const next = new Set(prev); if (!next.delete(key)) next.add(key); return next; });
  }

  // moveFocus walks the roving tabindex to a neighbouring row.
  function moveFocus(from: number, delta: number) {
    bodyRef.current?.querySelector<HTMLElement>(`[data-row-index="${from + delta}"]`)?.focus();
  }

  const keyWidth = keyColumnWidth(issues.map((i) => i.key), PENDING_DOT_PX);
  const sortedLabel = GRID_COLUMNS.find((c) => c.id === sort)?.label;
  const parentKeys = drawnParents(issues);

  return (
    <div
      className="issue-table"
      // A treegrid, not a grid: the rows are a hierarchy the arrow keys
      // already walk and the toggles already open, and aria-level and
      // aria-expanded on a row mean nothing in a plain grid.
      role="treegrid"
      aria-label="Issues"
      aria-multiselectable="false"
      aria-rowcount={rows.length + 1}
      style={{ "--issue-key-w": keyWidth } as CSSProperties}
    >
      {/* The order was otherwise invisible, and rank order is the whole point
          of a backlog: a row's position means something, so say so. */}
      <p className="issue-order muted small" role="status">
        {sortedLabel
          ? `Sorted by ${sortedLabel} ${desc ? "descending" : "ascending"}, drafts first. Click the column again to clear.`
          : "In Jira rank order, drafts first."}
        {" Subtasks stay under their parent. Filters include the whole group."}
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
        {rows.map((iss, index) => {
          const selected = iss.key === selectedKey;
          const place = familyPlace(iss, parentKeys);
          const nested = place === "child";
          const summary = toPlainText(iss.summary, "summary");
          return (
            <div
              key={iss.key}
              role="row"
              aria-selected={selected}
              aria-rowindex={index + 2}
              aria-level={place === "root" ? 1 : 2}
              aria-expanded={counts.has(iss.key) ? !collapsed.has(iss.key) : undefined}
              aria-label={`${iss.key} ${summary}${iss.type === "subtask" && iss.parentKey ? `, subtask of ${iss.parentKey}` : ""}`}
              data-row-index={index}
              className={`issue-row${selected ? " issue-row-selected" : index % 2 ? " issue-row-alt" : ""}`}
              onClick={() => onSelect(iss.key)}
              onKeyDown={(e) => {
                if (e.key === "ArrowRight" && counts.has(iss.key)) {
                  e.preventDefault();
                  if (collapsed.has(iss.key)) toggleChildren(iss.key);
                } else if (e.key === "ArrowLeft") {
                  e.preventDefault();
                  if (counts.has(iss.key) && !collapsed.has(iss.key)) toggleChildren(iss.key);
                  else if (nested) bodyRef.current?.querySelector<HTMLElement>(`[data-row-index="${rows.findIndex((p) => p.key === iss.parentKey)}"]`)?.focus();
                } else if (e.key === "Enter" || e.key === " ") {
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
              <span role="gridcell" className="issue-summary row-summary">
                <RowLead
                  place={place}
                  toggle={<SubtaskToggle issueKey={iss.key} count={counts.get(iss.key) ?? 0} expanded={!collapsed.has(iss.key)} onToggle={() => toggleChildren(iss.key)} />}
                />
                <span className="row-summary-text" title={summary}>{summary}</span>
              </span>
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
