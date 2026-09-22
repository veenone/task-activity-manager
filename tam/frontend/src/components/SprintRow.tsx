import type { KeyboardEvent } from "react";
import { Menu, StatusBadge } from "@agile-suite/core";
import type { BadgeTone, MenuItem } from "@agile-suite/core";
import type { SprintDetail } from "../api";
import { UNASSIGNED_SPRINT_STATE } from "../api";
import { plural, sprintDates } from "../lib/format";
import { DRAFT_SPRINT_HINT } from "../lib/sprintOptions";
import { CARD_MENU_CLASS } from "./CardMoveMenu";
import { SprintTimeline } from "./SprintTimeline";

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
  // waiting names what Commit will do to this sprint, "Deleting on Commit",
  // or is empty when nothing waits.
  waiting?: string;
  onToggle: () => void;
  onKeyDown: (e: KeyboardEvent) => void;
  actions: SprintActions;
}

// DRAFT_GOAL is what stands in for a draft sprint's goal. A draft is not in
// Jira yet, so it has no goal to have recorded, and the blank the row would
// otherwise leave says nothing about why.
const DRAFT_GOAL = "not created in Jira yet";

// UNCOUNTED is the scope cell for a closed sprint the backfill has not
// reached. "0 cards" would be a count, and this is the absence of one.
const UNCOUNTED = "Not counted yet";

// stateLabel prints Jira's lowercase state as a word rather than a shout.
// The badge's capitals are text-transform in the stylesheet: this string is
// also the row's accessible name, and a screen reader spells an uppercased
// one out letter by letter.
function stateLabel(state: string): string {
  return state.charAt(0).toUpperCase() + state.slice(1);
}

// badgeTone maps Jira's states onto the badge's four. Jira has no state for
// a sprint TAM has not created yet, so a draft is decided before this.
function badgeTone(state: string): BadgeTone {
  if (state === "active") return "active";
  if (state === "closed") return "closed";
  return "future";
}

// SprintRow is one sprint's header: a caret, its name and goal, its state
// over its dates, where it is in its calendar and how much of it has landed,
// what it holds, and the menu its actions live in.
//
// The goal is drawn here as well as under an expanded sprint. The name
// column has the widest track and the goal clips with a title attribute the
// way the name already does, which is the answer to the objection that kept
// it off the row: a reader can see what a sprint is for without opening it,
// and the copy for a sprint with no goal at all still belongs under the
// expanded sprint, where a sentence has a line to itself.
export function SprintRow({
  detail, rowId, index, open, focused, flashed, busy, waiting, onToggle, onKeyDown, actions,
}: Props) {
  // The board's own unassigned work arrives in the same list and under the
  // same shape, and is not a sprint: it has no state to badge, no dates to
  // print, and nothing that can be started, edited or deleted.
  const isSprint = detail.state !== UNASSIGNED_SPRINT_STATE;
  const dates = isSprint ? sprintDates(detail) || "no dates" : "no dates";
  const closed = detail.state === "closed";
  // A closed sprint whose membership has not been read yet is every closed
  // sprint on a board's first sync. Its numbers are all zero because nobody
  // has asked, not because the sprint was empty, so the cell says which.
  const uncounted = closed && !detail.membershipCached;

  const items: MenuItem[] = [];
  // A draft can be started, since Commit creates it before it starts it,
  // but a sprint that is not in Jira cannot have run, so Complete is shown
  // and held back to say what Commit unlocks.
  if (detail.draft || detail.state === "future") {
    items.push({ key: "start", label: "Start sprint…", disabled: busy, onClick: actions.onStart });
  }
  if (detail.draft) {
    items.push({ key: "complete", label: "Complete sprint…", disabled: true, title: DRAFT_SPRINT_HINT });
  } else if (detail.state === "active") {
    items.push({ key: "complete", label: "Complete sprint…", disabled: busy, onClick: actions.onComplete });
  }
  if (items.length > 0) items.push({ key: "manage", divider: true });
  items.push({ key: "edit", label: "Edit sprint…", disabled: busy, onClick: actions.onEdit });
  items.push({ key: "delete", label: "Delete sprint…", danger: true, disabled: busy, onClick: actions.onDelete });
  // A draft reads "Draft" in the badge and the tree's own label, in the
  // amber every other thing waiting for Commit wears.
  const label = detail.draft ? "Draft" : stateLabel(detail.state);
  const goal = detail.draft ? DRAFT_GOAL : detail.goal;

  return (
    <div
      role="treeitem"
      aria-expanded={open}
      aria-label={isSprint ? `${detail.name}, ${label}` : detail.name}
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
      <span className="sprint-cell sprint-cell-stack">
        <span className="sprint-cell-title" title={detail.name}>{detail.name}</span>
        {isSprint && goal && <span className="sprint-goal-line" title={goal}>{goal}</span>}
      </span>
      <span className="sprint-cell sprint-cell-stack">
        {isSprint && (
          <span className="sprint-cell-badges">
            <StatusBadge tone={detail.draft ? "draft" : badgeTone(detail.state)} label={label} />
            {waiting && <span className="chip chip-draft">{waiting}</span>}
          </span>
        )}
        <span className="sprint-cell-dates">{dates}</span>
      </span>
      <span className="sprint-cell sprint-cell-timeline">
        <SprintTimeline detail={detail} />
      </span>
      <span className="sprint-cell sprint-cell-stack sprint-cell-scope">
        {uncounted ? (
          <span className="sprint-cell-secondary">{UNCOUNTED}</span>
        ) : (
          <>
            <span>{plural(detail.total, "card", "cards")}</span>
            {detail.total > 0 && (
              <span className="sprint-cell-secondary">
                {closed ? `${detail.total - detail.done} carried over` : `${detail.done} done`}
              </span>
            )}
          </>
        )}
      </span>
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
