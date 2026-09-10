import type { KeyboardEvent } from "react";
import { Menu } from "@agile-suite/core";
import type { MenuItem } from "@agile-suite/core";
import type { SprintDetail } from "../api";
import { UNASSIGNED_SPRINT_STATE } from "../api";
import { progressText, sprintDates } from "../lib/format";
import { CARD_MENU_CLASS } from "./CardMoveMenu";

// The four things a sprint row can open, each of them a dialog. The row
// itself performs none of them.
export interface SprintActions {
  onStart: () => void;
  onComplete: () => void;
  onEdit: () => void;
  onDelete: () => void;
}

interface Props {
  detail: SprintDetail;
  // rowId is the tree's own id for this row, which is not the sprint id: it
  // is namespaced so it can never collide with an issue key, and the
  // unassigned node has no sprint id to be named by at all.
  rowId: string;
  index: number | undefined;
  open: boolean;
  focused: boolean;
  flashed: boolean;
  // busy is this sprint's own write in flight. Every other row's menu stays
  // usable: the lock is per profile, so a second write would be refused, but
  // greying out the whole tree for one rename would be a worse lie than
  // letting Jira answer.
  busy: boolean;
  onToggle: () => void;
  onKeyDown: (e: KeyboardEvent) => void;
  actions: SprintActions;
}

// stateLabel prints Jira's lowercase state as a word rather than a shout.
// "ACTIVE" in a chip is how a board says "look here"; a list where most
// rows are closed and one is active does not need shouting to say which.
function stateLabel(state: string): string {
  return state.charAt(0).toUpperCase() + state.slice(1);
}

// stateClass reuses the three status colours the issue chips already carry,
// so a running sprint and an in-progress card read the same on one screen.
function stateClass(state: string): string {
  if (state === "active") return "active";
  if (state === "closed") return "done";
  return "todo";
}

// SprintRow is one sprint's header: a caret, its name, its state, its dates,
// how far through it is, and the menu its actions live in.
//
// The goal is deliberately not here. It is a sentence, often a long one,
// and every cell on this row clips to its track; worse, opening the detail
// panel narrows the whole pane, so the one cell long enough to hold a goal
// would be the first to lose its width. It is rendered under the sprint
// when the sprint is expanded, where a sentence has a line to itself.
export function SprintRow({
  detail, rowId, index, open, focused, flashed, busy, onToggle, onKeyDown, actions,
}: Props) {
  // The board's own unassigned work arrives in the same list and under the
  // same shape, and is not a sprint: it has no state to chip, no dates to
  // print, and nothing that can be started, edited or deleted.
  const isSprint = detail.state !== UNASSIGNED_SPRINT_STATE;
  const dates = isSprint ? sprintDates(detail) : "";
  // A scope with no cards has no progress to report, and "0 of 0 done" in
  // the column where every other row says something is worse than a gap.
  const progress = detail.total > 0
    ? progressText(detail.done, detail.total, detail.donePoints, detail.points)
    : "";

  const items: MenuItem[] = [];
  if (detail.state === "future") {
    items.push({ key: "start", label: "Start sprint…", disabled: busy, onClick: actions.onStart });
  }
  if (detail.state === "active") {
    items.push({ key: "complete", label: "Complete sprint…", disabled: busy, onClick: actions.onComplete });
  }
  if (items.length > 0) items.push({ key: "manage", divider: true });
  items.push({ key: "edit", label: "Edit sprint…", disabled: busy, onClick: actions.onEdit });
  items.push({ key: "delete", label: "Delete sprint…", danger: true, disabled: busy, onClick: actions.onDelete });

  return (
    <div
      role="treeitem"
      aria-expanded={open}
      aria-label={isSprint ? `${detail.name}, ${stateLabel(detail.state)}` : detail.name}
      tabIndex={focused ? 0 : -1}
      data-tree-index={index}
      data-tree-key={rowId}
      className={`folder-item sprint-row${flashed ? " epic-row-moved" : ""}`}
      onClick={onToggle}
      onKeyDown={onKeyDown}
    >
      <span
        className="folder-caret folder-caret-toggle"
        onClick={(e) => { e.stopPropagation(); onToggle(); }}
      >
        {open ? "▾" : "▸"}
      </span>
      <span className="sprint-cell sprint-cell-name" title={detail.name}>{detail.name}</span>
      <span className="sprint-cell sprint-cell-state">
        {isSprint && (
          <span className={`chip chip-status chip-status-${stateClass(detail.state)}`}>
            {stateLabel(detail.state)}
          </span>
        )}
      </span>
      <span className="sprint-cell sprint-cell-dates">{dates}</span>
      <span className="sprint-cell folder-count sprint-cell-progress">{progress}</span>
      <span className="sprint-cell sprint-cell-actions">
        {isSprint && (
          // The row answers its own keys and toggles on click, so the menu
          // stops both before either reaches it, exactly as the board card's
          // menu does. It carries that menu's class and its tab index too:
          // the tree is one tab stop, so a trigger per row would add one
          // stop per sprint, and the class is the handle the keyboard path
          // clicks, since a trigger the Menu primitive owns has no other.
          <span
            className="board-card-menu-wrap"
            onClick={(e) => e.stopPropagation()}
            onKeyDown={(e) => e.stopPropagation()}
          >
            <Menu
              label="Actions"
              items={items}
              align="right"
              title={`Actions on ${detail.name}`}
              triggerLabel={`Actions on ${detail.name}`}
              triggerClassName={`btn ${CARD_MENU_CLASS}`}
              triggerTabIndex={-1}
            />
          </span>
        )}
      </span>
    </div>
  );
}
