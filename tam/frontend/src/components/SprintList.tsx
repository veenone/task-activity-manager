import { useEffect, useRef, useState } from "react";
import type { CSSProperties, KeyboardEvent, MouseEvent } from "react";
import type { Issue, SprintDetail } from "../api";
import { MAX_CARDS_PER_VIEW, UNASSIGNED_SPRINT_STATE } from "../api";
import { groupByAssignee } from "../lib/sprintGroups";
import { plural, points } from "../lib/format";
import { keyColumnWidth } from "../lib/keyColumn";
import { statusClass } from "../lib/statusClass";
import { MOVED_FLASH_MS } from "../lib/flash";
import { TypeChip } from "./TypeChip";
import { SprintRow } from "./SprintRow";
import { CARD_MENU_CLASS } from "./CardMoveMenu";

// SCOPE_PREFIX namespaces a sprint's row id so it can never collide with an
// issue key, which matters more here than it looks: the selection's order
// array holds issue keys only, and lib/boardSelection's extend slices that
// array blindly, so an id that could pass for a key would end up checked and
// then in a bulk move.
const SCOPE_PREFIX = "sprint:";
const UNASSIGNED_ROW_ID = `${SCOPE_PREFIX}unassigned`;

// rowIdOf names one node of the list. The board's own unassigned work has no
// sprint id to be named by, so it gets a name of its own.
export function rowIdOf(detail: SprintDetail): string {
  return detail.state === UNASSIGNED_SPRINT_STATE ? UNASSIGNED_ROW_ID : `${SCOPE_PREFIX}${detail.id}`;
}

interface TreeRow {
  id: string;
  kind: "sprint" | "issue";
  // scope is the sprint row the row belongs to, and its own id for a sprint
  // row. A shift gesture is measured inside one scope and nowhere else.
  scope: string;
}

// visibleRows flattens the list into the order it is drawn in, which is the
// keyboard model: each sprint, then its cards when it is open. The assignee
// separators are not rows here because they are not tree items: they are
// labels drawn between cards, so the tree stays two levels deep and nothing
// lands focus on a band heading that does nothing when activated.
function visibleRows(details: SprintDetail[], expanded: ReadonlySet<string>): TreeRow[] {
  const rows: TreeRow[] = [];
  for (const detail of details) {
    const id = rowIdOf(detail);
    rows.push({ id, kind: "sprint", scope: id });
    if (!expanded.has(id)) continue;
    for (const group of groupByAssignee(detail.issues)) {
      for (const issue of group.issues) rows.push({ id: issue.key, kind: "issue", scope: id });
    }
  }
  return rows;
}

// issueOrder is the selection's reading order, per scope: the cards of one
// sprint in the order they are drawn. A shift gesture extends inside one of
// these lists, which is what keeps a drag from the top of Sprint 12 to the
// bottom of Sprint 14 from checking three sprints' work at once. It cannot
// happen on a board, which draws one sprint at a time, and it is one drag
// away here.
export function issueOrder(details: SprintDetail[]): Map<string, string[]> {
  const out = new Map<string, string[]>();
  for (const detail of details) {
    out.set(rowIdOf(detail), groupByAssignee(detail.issues).flatMap((g) => g.issues.map((i) => i.key)));
  }
  return out;
}

interface Props {
  details: SprintDetail[];
  selectedKey: string;
  onSelect: (key: string) => void;
  checked: ReadonlySet<string>;
  // check toggles one card, extend runs from the anchor to this one, and
  // clearTo drops the multi-selection and leaves this card as the anchor.
  onCheck: (key: string) => void;
  onExtend: (key: string, scope: string) => void;
  onClearTo: (key: string) => void;
  // movedRowId is a sprint the view has just written: it scrolls into view
  // and flashes once, the same two seconds a reparented row flashes for in
  // the Epics tree.
  movedRowId: string;
  // busyRowId is the sprint whose own write is in flight, whose menu is
  // closed for the duration.
  busyRowId: string;
  onStart: (detail: SprintDetail) => void;
  onComplete: (detail: SprintDetail) => void;
  onEdit: (detail: SprintDetail) => void;
  onDelete: (detail: SprintDetail) => void;
}

// SprintList is the Sprints view's tree, and it is the Epics tree's shape:
// one roving tab stop, arrows to walk it, a caret per branch, and the same
// folder classes. Two things differ, both because this tree has a
// multi-selection and that one does not: Space checks a card here, where the
// Epics tree binds Space and Enter both to select, and a checked card wears
// a real checkbox rather than a tint, since the tint that would have said
// "checked" is already what says "selected" and amber is already what says
// "waiting for Commit".
export function SprintList({
  details, selectedKey, onSelect, checked, onCheck, onExtend, onClearTo,
  movedRowId, busyRowId, onStart, onComplete, onEdit, onDelete,
}: Props) {
  const rootRef = useRef<HTMLElement>(null);
  const seenRef = useRef<Set<string>>(new Set());
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [flashId, setFlashId] = useState("");
  const [focusId, setFocusId] = useState("");

  // Only the active sprint opens by itself. A board carries every sprint it
  // has ever run, so opening what has not been seen before, the way the
  // Epics tree opens a new epic, would open the whole of last quarter on the
  // first render. A sprint the user has collapsed stays collapsed, which is
  // what seenRef is for.
  useEffect(() => {
    const toOpen = details
      .filter((d) => d.state === "active" && !seenRef.current.has(rowIdOf(d)))
      .map((d) => rowIdOf(d));
    if (toOpen.length === 0) return;
    for (const id of toOpen) seenRef.current.add(id);
    setExpanded((prev) => {
      const next = new Set(prev);
      for (const id of toOpen) next.add(id);
      return next;
    });
  }, [details]);

  // A sprint the view has just written flashes, which is the one report a
  // create gets on this screen besides its announcement.
  useEffect(() => {
    if (!movedRowId) return;
    setFlashId(movedRowId);
    const t = setTimeout(() => setFlashId(""), MOVED_FLASH_MS);
    return () => clearTimeout(t);
  }, [movedRowId]);

  // The scroll is an effect of its own because a created sprint is named
  // before this tree has it: the write invalidates the board's sprints and
  // the row arrives with that refetch, a round trip after the id does, so
  // a scroll that ran only on the id would look for a row that is not there
  // yet and quietly do nothing. Watching the details as well means the row
  // is scrolled to when it turns up. A row already in view is not moved,
  // so the repeat a refetch causes costs nothing.
  useEffect(() => {
    if (!movedRowId) return;
    rootRef.current
      ?.querySelector<HTMLElement>(`[data-tree-key="${movedRowId}"]`)
      ?.scrollIntoView({ block: "nearest" });
  }, [movedRowId, details]);

  const rows = visibleRows(details, expanded);
  const indexOf = new Map(rows.map((r, i) => [r.id, i] as const));
  // One key width for the whole tree, for the reason lib/keyColumn gives:
  // every row is its own grid container, so a per-row track would size each
  // row to its own key and the columns would stop lining up.
  const keyWidth = keyColumnWidth(details.flatMap((d) => d.issues.map((i) => i.key)));

  // Exactly one row keeps tabIndex 0. It follows the selection, and falls
  // back to the first row whenever a collapse or a refetch takes away
  // whichever row was holding it.
  useEffect(() => {
    setFocusId((prev) => {
      if (indexOf.has(prev)) return prev;
      if (indexOf.has(selectedKey)) return selectedKey;
      return rows[0]?.id ?? "";
    });
  }, [details, expanded, selectedKey]);

  function toggle(id: string, focus?: boolean) {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (!next.delete(id)) next.add(id);
      return next;
    });
    if (focus) setFocusId(id);
  }

  function moveFocus(index: number, shift: boolean) {
    const target = rows[index];
    if (!target) return;
    setFocusId(target.id);
    rootRef.current?.querySelector<HTMLElement>(`[data-tree-index="${index}"]`)?.focus();
    // A shift gesture from the keyboard checks the run it walks, the same
    // one a shift click draws, and stops at a sprint header rather than
    // pretending a header is part of the run.
    if (shift && target.kind === "issue") onExtend(target.id, target.scope);
  }

  // openMenu is the keyboard's way into a sprint's actions: the menu key and
  // Shift with F10 open the menu the mouse opens. The trigger belongs to the
  // Menu primitive, so its class is the only handle there is.
  function openMenu(id: string) {
    rootRef.current
      ?.querySelector<HTMLElement>(`[data-tree-key="${id}"] .${CARD_MENU_CLASS}`)
      ?.click();
  }

  function onKeyDown(e: KeyboardEvent, row: TreeRow) {
    const index = indexOf.get(row.id) ?? 0;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      moveFocus(Math.min(rows.length - 1, index + 1), e.shiftKey);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      moveFocus(Math.max(0, index - 1), e.shiftKey);
    } else if (e.key === "ArrowRight") {
      e.preventDefault();
      if (row.kind === "sprint") setExpanded((prev) => new Set(prev).add(row.id));
    } else if (e.key === "ArrowLeft") {
      e.preventDefault();
      if (row.kind === "sprint") {
        setExpanded((prev) => {
          const next = new Set(prev);
          next.delete(row.id);
          return next;
        });
      } else {
        moveFocus(indexOf.get(row.scope) ?? index, false);
      }
    } else if (e.key === "Enter") {
      e.preventDefault();
      if (row.kind === "sprint") toggle(row.id, true);
      else {
        onClearTo(row.id);
        onSelect(row.id);
        setFocusId(row.id);
      }
    } else if (e.key === " ") {
      // The deliberate divergence from the Epics tree, which binds Space and
      // Enter both to select. A tree with a multi-selection needs a key for
      // checking, Space is that key everywhere else in this app, and one key
      // cannot mean two things on one row.
      e.preventDefault();
      if (row.kind === "issue") onCheck(row.id);
    } else if (e.key === "ContextMenu" || (e.key === "F10" && e.shiftKey)) {
      e.preventDefault();
      if (row.kind === "sprint") openMenu(row.id);
    }
  }

  // A click on a card is one of three gestures, the same three the board
  // reads: control checks it, shift runs from the anchor, and a plain click
  // is the one that is about a single card, so it opens the panel and leaves
  // that card as the anchor.
  function clickIssue(e: MouseEvent, row: TreeRow) {
    if (e.shiftKey) {
      onExtend(row.id, row.scope);
      setFocusId(row.id);
      return;
    }
    if (e.ctrlKey || e.metaKey) {
      onCheck(row.id);
      setFocusId(row.id);
      return;
    }
    onClearTo(row.id);
    onSelect(row.id);
    setFocusId(row.id);
  }

  function issueRow(issue: Issue, scope: string) {
    const row: TreeRow = { id: issue.key, kind: "issue", scope };
    const isChecked = checked.has(issue.key);
    return (
      <div
        key={issue.key}
        role="treeitem"
        aria-selected={issue.key === selectedKey}
        aria-label={`${issue.key} ${issue.summary}`}
        tabIndex={focusId === issue.key ? 0 : -1}
        data-tree-index={indexOf.get(issue.key)}
        data-tree-key={issue.key}
        className={`folder-item sprint-issue-row${issue.key === selectedKey ? " folder-selected" : ""}`}
        onClick={(e) => clickIssue(e, row)}
        onKeyDown={(e) => onKeyDown(e, row)}
      >
        <input
          type="checkbox"
          className="sprint-check"
          // Out of the tab order: the tree is one tab stop, and a checkbox
          // per card would make it one stop per card. Space on the row is
          // what checks it from the keyboard.
          tabIndex={-1}
          checked={isChecked}
          aria-label={`Check ${issue.key}`}
          onClick={(e) => e.stopPropagation()}
          onChange={() => onCheck(issue.key)}
        />
        <TypeChip type={issue.type} />
        <span className="sprint-cell epic-cell-key accent-text" title={issue.key}>{issue.key}</span>
        <span className="sprint-cell epic-cell-summary" title={issue.summary}>{issue.summary}</span>
        <span className="sprint-cell">
          <span className={`chip chip-status chip-status-${statusClass(issue.status)}`} title={issue.status}>
            {issue.status}
          </span>
        </span>
        <span className="sprint-cell epic-cell-points">{issue.storyPoints ?? "-"}</span>
        {issue.pending && <span className="pending-dot" role="img" aria-label="Pending changes" />}
      </div>
    );
  }

  function body(detail: SprintDetail, id: string) {
    const isSprint = detail.state !== UNASSIGNED_SPRINT_STATE;
    // The gap between what this node holds and what the view drew of it,
    // which is the one thing issues.length is the right number for: the
    // read spends one card budget across every sprint and the unassigned
    // node together, so a node whose share ran out draws fewer cards than
    // it counted. Every other number on this screen comes from total.
    const notDrawn = detail.total - detail.issues.length;
    return (
      <div className="folder-children sprint-children" role="group">
        {isSprint && (
          <p className="muted small sprint-goal">
            {detail.goal || "No goal recorded yet; refresh the board."}
          </p>
        )}
        {groupByAssignee(detail.issues).map((group) => (
          <div key={group.id || "sprint-unassigned"}>
            {/* A band heading, and not a tree item: it labels the cards
                under it and does nothing when clicked, so putting it in the
                tree would give the keyboard a stop that leads nowhere. */}
            <div className="sprint-group" role="presentation">
              <span className="sprint-group-name">{group.label}</span>
              <span className="muted small">
                {plural(group.issues.length, "card", "cards")}
                {group.points > 0 ? `, ${points(group.points)} pts` : ""}
              </span>
            </div>
            {group.issues.map((issue) => issueRow(issue, id))}
          </div>
        ))}
        {detail.total === 0 && (
          <p className="muted small">
            {detail.notSynced > 0
              ? `Nothing here has been synced. ${plural(detail.notSynced, "card is", "cards are")} in it but not in this cache.`
              : !isSprint
                ? "Every card on this board is in a sprint."
                : detail.membershipCached
                  ? "Nothing is in this sprint."
                  : "A closed sprint's cards are not synced, so this list is empty whatever the sprint held."}
          </p>
        )}
        {notDrawn > 0 && (
          <p className="muted small">
            {`${plural(notDrawn, "card", "cards")} more ${notDrawn === 1 ? "is" : "are"} in this list than the view draws: the read stops at ${MAX_CARDS_PER_VIEW.toLocaleString()} cards across every sprint together.`}
          </p>
        )}
        {detail.total > 0 && detail.notSynced > 0 && (
          <p className="muted small">
            {`${plural(detail.notSynced, "card", "cards")} in this list ${detail.notSynced === 1 ? "has" : "have"} not been synced, so ${detail.notSynced === 1 ? "it is" : "they are"} counted nowhere above.`}
          </p>
        )}
      </div>
    );
  }

  return (
    <nav
      className="folder-tree sprint-tree"
      aria-label="Sprints"
      role="tree"
      ref={rootRef}
      style={{ "--sprint-key-w": keyWidth } as CSSProperties}
    >
      {details.map((detail) => {
        const id = rowIdOf(detail);
        const open = expanded.has(id);
        return (
          <div className="folder-node" key={id}>
            <SprintRow
              detail={detail}
              rowId={id}
              index={indexOf.get(id)}
              open={open}
              focused={focusId === id}
              flashed={flashId === id}
              busy={busyRowId === id}
              onToggle={() => toggle(id, true)}
              onKeyDown={(e) => onKeyDown(e, { id, kind: "sprint", scope: id })}
              actions={{
                onStart: () => onStart(detail),
                onComplete: () => onComplete(detail),
                onEdit: () => onEdit(detail),
                onDelete: () => onDelete(detail),
              }}
            />
            {open && body(detail, id)}
          </div>
        );
      })}
    </nav>
  );
}
