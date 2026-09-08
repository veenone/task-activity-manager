import { useEffect, useMemo, useRef, useState } from "react";
import type { DragEvent } from "react";
import { announce, errMsg, useNotice } from "@agile-suite/core";
import { ENTITY_TRANSITION } from "../api";
import type { BoardView, CommitResult, Issue, Sprint } from "../api";
import { findCard } from "../lib/boardCells";
import type { Pos } from "../lib/boardCells";
import { columnDrop, isNoMove, isSameCell, keyboardMove } from "../lib/cardMove";
import type { Drop, MoveIntent } from "../lib/cardMove";
import { cardMoves, warningLine } from "../lib/cardMoveState";
import type { Warning } from "../lib/cardMoveState";
import { useCanTransition, useMoveToColumn, useMoveToSprint, useRankIssue } from "../queries/boards";
import { useDiscardChange, usePendingChanges } from "../queries/pending";

// useBoardMoves is everything a board move is, in one place: the three
// writes, the check that follows a column move, the warnings it produces,
// and the drag state the grid draws from. It lives beside BoardsView
// rather than inside it because the view is already the board's read
// path, and a move is the other half.

// DropTarget is what the grid draws while a drag is over a cell. index is
// null for a column move, which lands the card by its own rank rather than
// where the cursor is, so a line there would promise a placement this move
// does not make.
export interface DropTarget {
  lane: number;
  col: number;
  index: number | null;
}

interface Args {
  profileId: string;
  view: BoardView | undefined;
  boardId: number;
  commit: CommitResult | null;
  // committing is a Commit in flight. The drag is stopped by the card's
  // own draggable and the menu by its disabled items, so this is the
  // keyboard's half of the same answer.
  committing: boolean;
}

export function useBoardMoves({ profileId, view, boardId, commit, committing }: Args) {
  const { notice } = useNotice();
  const pending = usePendingChanges(profileId);
  const toColumn = useMoveToColumn(profileId);
  const rank = useRankIssue(profileId);
  const toSprint = useMoveToSprint(profileId);
  const check = useCanTransition(profileId);
  const discard = useDiscardChange(profileId);

  const [warnings, setWarnings] = useState<Map<string, Warning>>(new Map());
  const [checking, setChecking] = useState<Set<string>>(new Set());
  const [intent, setIntent] = useState<MoveIntent | null>(null);
  const [dragKey, setDragKey] = useState("");
  const [target, setTarget] = useState<DropTarget | null>(null);
  // The newest check per key. A standup is ten drags and the answers do
  // not come back in the order they were asked, so an older answer must
  // never overwrite a newer one.
  const asked = useRef(new Map<string, number>());

  const rows = useMemo(() => pending.data ?? [], [pending.data]);
  const moves = useMemo(
    () => cardMoves({ pending: rows, commit, warnings, checking }),
    [rows, commit, warnings, checking],
  );

  // A warning belongs to a journaled transition, and that row can be
  // discarded anywhere: this banner's own Put it back, the Pending changes
  // dialog, Discard all, or a Commit that pushed it. The card's marker
  // clears with the row, so the sentence above the board has to go with it
  // rather than stand there contradicting the card it names.
  useEffect(() => {
    if (!pending.data) return;
    setWarnings((prev) => {
      if (prev.size === 0) return prev;
      const journaled = new Set(
        pending.data.filter((r) => r.entityType === ENTITY_TRANSITION).map((r) => r.entityKey),
      );
      const next = new Map([...prev].filter(([key]) => journaled.has(key)));
      return next.size === prev.size ? prev : next;
    });
  }, [pending.data]);

  function forget(key: string) {
    setWarnings((prev) => {
      if (!prev.has(key)) return prev;
      const next = new Map(prev);
      next.delete(key);
      return next;
    });
  }

  function failed(e: unknown) {
    void notice({ title: "The card could not be moved", message: errMsg(e), tone: "error" });
  }

  // verify asks Jira whether the card can reach the column it was just
  // dropped in. It is best effort: an error means the check could not be
  // made, so nothing is said, rather than a warning nobody can stand
  // behind.
  function verify(key: string, statusId: string, columnName: string) {
    const n = (asked.current.get(key) ?? 0) + 1;
    asked.current.set(key, n);
    setChecking((prev) => new Set(prev).add(key));
    check.mutate(
      { key, statusId },
      {
        onSuccess: (answer) => {
          if (asked.current.get(key) !== n || answer.allowed) return;
          setWarnings((prev) => new Map(prev).set(key, { target: columnName, reachable: answer.reachable ?? [] }));
        },
        onSettled: () => {
          if (asked.current.get(key) !== n) return;
          setChecking((prev) => {
            const next = new Set(prev);
            next.delete(key);
            return next;
          });
        },
      },
    );
  }

  function moveToColumn(key: string, col: number) {
    if (!view) return;
    const column = view.columns[col];
    if (!column) return;
    const statusId = (column.statusIds ?? [])[0];
    // The keyboard refuses this column with a sentence, and a menu never
    // offers it, but a drag can still be dropped on one. Saying nothing
    // would make the card look as though it moved and came back.
    if (!statusId) {
      announce(`${column.name} collects no status, so a card cannot be moved into it`);
      return;
    }
    forget(key);
    toColumn.mutate(
      { key, statusId },
      {
        onSuccess: () => {
          setIntent({ key, kind: "column", target: column.name, at: Date.now() });
          verify(key, statusId, column.name);
        },
        onError: failed,
      },
    );
  }

  function rankTo(key: string, neighbourKey: string, before: boolean, kind: "up" | "down") {
    rank.mutate(
      { key, neighbourKey, before, boardId },
      {
        onSuccess: () => setIntent({ key, kind, target: "", at: Date.now() }),
        onError: failed,
      },
    );
  }

  function moveToSprint(key: string, sprint: Sprint | null) {
    const sprintId = sprint ? String(sprint.id) : "";
    const name = sprint ? sprint.name : "";
    forget(key);
    toSprint.mutate(
      { key, sprintId },
      {
        onSuccess: () => setIntent({ key, kind: "sprint", target: name, at: Date.now() }),
        onError: failed,
      },
    );
  }

  // moveByKeyboard answers Ctrl and an arrow on the focused card, and says
  // so when the move it asks for cannot be made. It returns whether the
  // key press was a move at all, so the caller can leave every other key
  // to the focus model 3a shipped.
  function moveByKeyboard(p: Pos, issue: Issue, key: string): boolean {
    if (!view) return false;
    const move = keyboardMove(view, p, issue.key, key);
    if (!move) return false;
    if (committing) {
      announce(`${issue.key} cannot be moved while a commit is running`);
      return true;
    }
    if (move.kind === "refused") {
      announce(move.message);
      return true;
    }
    if (move.kind === "column") moveToColumn(issue.key, move.col);
    else rankTo(issue.key, move.neighbourKey, move.before, move.index < p.index ? "up" : "down");
    return true;
  }

  function onDragStart(e: DragEvent, key: string) {
    e.dataTransfer.setData("text/plain", key);
    e.dataTransfer.effectAllowed = "move";
    setDragKey(key);
  }

  function onDragEnd() {
    setDragKey("");
    setTarget(null);
  }

  // refuse is the honest cursor for a drop that would do nothing: no line,
  // no highlight, and no preventDefault, so the browser draws the refusal
  // rather than promising a move and then making none.
  function refuse(e: DragEvent) {
    e.dataTransfer.dropEffect = "none";
    setTarget(null);
  }

  // dropIn reads the cursor's place in the cell it is over, from the cards
  // the cell has actually laid out.
  function dropIn(e: DragEvent<HTMLElement>, lane: number, col: number): Drop {
    const keys = (view?.lanes[lane]?.cells[col] ?? []).map((c) => c.key);
    const cards = [...e.currentTarget.querySelectorAll<HTMLElement>("[data-board-pos]")];
    return columnDrop(keys, e.clientY, cards.map((el) => el.getBoundingClientRect()));
  }

  function from(): Pos | undefined {
    return view && dragKey ? findCard(view, dragKey) : undefined;
  }

  // belowHidden is a drop under the last card of a capped cell: the rank
  // would be against a card the column is not showing. The drop refuses it
  // and says so, and the dragover must not draw a line promising it first.
  function belowHidden(lane: number, col: number, index: number): boolean {
    const cards = view?.lanes[lane]?.cells[col] ?? [];
    return index >= cards.length && (view?.lanes[lane]?.overflow[col] ?? 0) > 0;
  }

  function onCellDragOver(e: DragEvent<HTMLElement>, lane: number, col: number) {
    const start = from();
    if (!view || !start) return;
    const drop = dropIn(e, lane, col);
    const same = isSameCell(start, lane, col);
    if (!same && (start.lane !== lane || (view.columns[col]?.statusIds ?? []).length === 0)) return refuse(e);
    if (same && isNoMove(start, lane, col, drop.index)) return refuse(e);
    if (same && belowHidden(lane, col, drop.index)) return refuse(e);
    e.preventDefault();
    e.dataTransfer.dropEffect = "move";
    setTarget({ lane, col, index: same ? drop.index : null });
  }

  function onCellDrop(e: DragEvent<HTMLElement>, lane: number, col: number) {
    const start = from();
    const key = dragKey;
    if (!view || !start) return;
    e.preventDefault();
    const drop = dropIn(e, lane, col);
    onDragEnd();
    if (start.lane !== lane) return;
    if (!isSameCell(start, lane, col)) {
      moveToColumn(key, col);
      return;
    }
    if (isNoMove(start, lane, col, drop.index)) return;
    if (belowHidden(lane, col, drop.index)) {
      announce(`${key} cannot be placed below the cards this column is not showing`);
      return;
    }
    rankTo(key, drop.neighbourKey, drop.before, drop.index < start.index ? "up" : "down");
  }

  // The one warning on screen, not a stack of them: the oldest unanswered
  // check, with the row a "Put it back" discards.
  const warned = [...warnings.entries()][0];
  const warning = warned
    ? {
      key: warned[0],
      line: warningLine(warned[0], warned[1]),
      row: rows.find((r) => r.entityKey === warned[0] && r.entityType === ENTITY_TRANSITION),
    }
    : null;

  function putBack() {
    if (!warning?.row) return;
    const key = warning.key;
    discard.mutate(warning.row, { onSuccess: () => forget(key), onError: failed });
  }

  return {
    moves,
    warning,
    intent,
    dragKey,
    target,
    busy: discard.isPending,
    moveByKeyboard,
    moveToColumn,
    moveToSprint,
    onDragStart,
    onDragEnd,
    onCellDragOver,
    onCellDrop,
    putBack,
  };
}

// BoardMoves is what the hook hands the board: the state every card's move
// is in, the drag in progress, and the writes themselves. It travels as one
// object rather than as a dozen props threaded through BoardBody.
export type BoardMoves = ReturnType<typeof useBoardMoves>;
