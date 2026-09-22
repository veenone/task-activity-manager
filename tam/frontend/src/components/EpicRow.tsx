import type { KeyboardEvent, ReactNode } from "react";
import { RowLead, toPlainText } from "@agile-suite/core";
import type { RowPlace } from "@agile-suite/core";
import type { EpicNode, Issue } from "../api";
import { TypeChip } from "./TypeChip";
import { SubtaskToggle } from "./SubtaskToggle";
import { statusClass } from "../lib/statusClass";
import type { Row } from "../lib/epicTreeItems";
import { progressText } from "../lib/format";

interface EpicRowProps {
  kind: "epic" | "noepic";
  subtaskLabel?: string;
  rowKey: string;
  node?: EpicNode;
  count?: number;
  index: number | undefined;
  open: boolean;
  selected: boolean;
  focused: boolean;
  flashed: boolean;
  onActivate: () => void;
  onToggle: () => void;
  onKeyDown: (e: KeyboardEvent) => void;
}

// EpicRow renders the header treeitem for either an epic node or the
// orphans group: same caret, expand state, and selection wiring, with the
// icon, name, count, and pending dot filled in from whichever kind it is.
export function EpicRow({
  kind, subtaskLabel, rowKey, node, count, index, open, selected, focused, flashed, onActivate, onToggle, onKeyDown,
}: EpicRowProps) {
  const isEpic = kind === "epic" && node;
  const typeIcon: ReactNode = isEpic ? <TypeChip type={node.issue.type} subtaskLabel={subtaskLabel} /> : <span />;
  const countText = isEpic ? progressText(node.done, node.total, node.donePoints, node.points) : String(count ?? 0);
  const summary = isEpic ? toPlainText(node.issue.summary, "summary") : "";
  const ariaLabel = isEpic ? `${rowKey} ${summary}` : undefined;
  const pending = isEpic && node.issue.pending;
  return (
    <div
      role="treeitem"
      aria-expanded={open}
      aria-selected={selected}
      tabIndex={focused ? 0 : -1}
      {...(ariaLabel ? { "aria-label": ariaLabel } : {})}
      data-tree-index={index}
      data-tree-key={rowKey}
      className={`folder-item epic-row${selected ? " folder-selected" : ""}${flashed ? " epic-row-moved" : ""}`}
      onClick={onActivate}
      onKeyDown={onKeyDown}
    >
      <span className="folder-caret folder-caret-toggle" onClick={(e) => { e.stopPropagation(); onToggle(); }}>
        {open ? "▾" : "▸"}
      </span>
      {typeIcon}
      <span className="epic-cell epic-cell-key accent-text" title={isEpic ? rowKey : undefined}>
        {isEpic ? rowKey : "No epic"}
      </span>
      <span className="epic-cell epic-cell-summary" title={isEpic ? summary : undefined}>
        {isEpic ? summary : ""}
      </span>
      <span className="epic-cell folder-count epic-cell-progress">{countText}</span>
      {pending && <span className="pending-dot" role="img" aria-label="Pending changes" />}
    </div>
  );
}

interface EpicChildRowProps {
  subtaskCount?: number;
  subtasksExpanded?: boolean;
  onToggleSubtasks?: () => void;
  child: Issue;
  subtaskLabel?: string;
  ownerKey: string;
  place?: RowPlace;
  index: number | undefined;
  selected: boolean;
  focused: boolean;
  flashed: boolean;
  onActivate: (key: string) => void;
  onKeyDown: (e: KeyboardEvent, row: Row) => void;
}

// EpicChildRow renders one leaf row, whether it hangs off an epic or off
// the orphans group; ownerKey tells the keyboard model which it is.
export function EpicChildRow({ child, subtaskCount = 0, subtasksExpanded = true, onToggleSubtasks, subtaskLabel, ownerKey, place = "root", index, selected, focused, flashed, onActivate, onKeyDown }: EpicChildRowProps) {
  const row: Row = { id: child.key, kind: "child", ownerKey };
  const summary = toPlainText(child.summary, "summary");
  const nested = place !== "root";
  return (
    <div
      role="treeitem"
      aria-level={nested ? 3 : 2}
      aria-expanded={subtaskCount > 0 ? subtasksExpanded : undefined}
      aria-selected={selected}
      tabIndex={focused ? 0 : -1}
      data-tree-index={index}
      data-tree-key={child.key}
      className={`folder-item epic-row${selected ? " folder-selected" : ""}${flashed ? " epic-row-moved" : ""}`}
      onClick={() => onActivate(child.key)}
      onKeyDown={(e) => onKeyDown(e, row)}
    >
      {/* The caret track stays, empty: it is the column the epic rows above
          put their own caret in, and the branch belongs beside the summary
          it marks rather than a column to its left. */}
      <span className="folder-caret" aria-hidden="true" />
      <TypeChip type={child.type} subtaskLabel={subtaskLabel} />
      <span className="epic-cell epic-cell-key" title={child.key}>{child.key}</span>
      <span className="epic-cell epic-cell-summary row-summary">
        <RowLead
          place={place}
          toggle={onToggleSubtasks ? <SubtaskToggle issueKey={child.key} count={subtaskCount} expanded={subtasksExpanded} onToggle={onToggleSubtasks} /> : undefined}
        />
        <span className="row-summary-text" title={summary}>{summary}</span>
      </span>
      <span className="epic-cell epic-cell-status">
        <span className={`chip chip-status chip-status-${statusClass(child.status)}`} title={child.status}>{child.status}</span>
      </span>
      <span className="epic-cell epic-cell-points">{child.storyPoints ?? "-"}</span>
      {child.pending && <span className="pending-dot" role="img" aria-label="Pending changes" />}
    </div>
  );
}
