import { useRef, useState } from "react";
import type { KeyboardEvent } from "react";
import { errMsg, useProfile } from "@agile-suite/core";
import type { BoardView, Issue, Profile, Settings, Swimlane } from "../api";
import { useBoard, useBoardSprints, useBoards, useBoardsUnavailable, useSyncBoards } from "../queries/boards";
import { useSyncState } from "../queries/issues";
import { useSync } from "../contexts/SyncContext";
import { clampFocus, findCard, moveFocus, parsePos, posId } from "../lib/boardCells";
import type { Pos } from "../lib/boardCells";
import { BoardBody } from "./BoardBody";
import { BoardMoveBanner } from "./BoardMoveBanner";
import { BoardsBanner } from "./BoardsBanner";
import { BoardSummaryLine } from "./BoardNotes";
import { BoardsToolbar } from "./BoardsToolbar";
import { CARD_MENU_CLASS } from "./CardMoveMenu";
import { IssueDetailPanel } from "./IssueDetailPanel";
import { useBoardMoves } from "./useBoardMoves";
import { useMovedCard } from "./useMovedCard";

// cardAtPos reads one card out of the view by its position.
function cardAtPos(view: BoardView, p: Pos | undefined): Issue | undefined {
  if (!p) return undefined;
  return view.lanes[p.lane]?.cells[p.col]?.[p.index];
}

// BoardsView is the board: XTM's board head over the board's own columns,
// the cards in them, and the lines that say what the board is not showing.
// Phase 3b makes it writable, through the journal: a drag, a key press, or
// the card's own menu moves a card, and Commit is what pushes any of it.
export function BoardsView() {
  const { activeId } = useProfile<Profile, Settings>();
  const { canSync, lastBoards, lastBoardsAt, lastCommit, status } = useSync();
  const [boardId, setBoardId] = useState(0);
  const [sprintId, setSprintId] = useState("");
  const [swimlane, setSwimlane] = useState<Swimlane>("none");
  const [selectedKey, setSelectedKey] = useState("");
  const [focusId, setFocusId] = useState("");
  const bodyRef = useRef<HTMLDivElement>(null);

  // The board, sprint, and swimlane choices belong to the profile they were
  // made for, so a switch clears them in the render that first sees the new
  // id. An effect would be one render too late: a board query would go out
  // pairing the new profile with the old board id.
  const [filtersFor, setFiltersFor] = useState(activeId);
  if (filtersFor !== activeId) {
    setFiltersFor(activeId);
    setBoardId(0);
    setSprintId("");
    setSwimlane("none");
    setSelectedKey("");
    setFocusId("");
  }

  const boards = useBoards(activeId);
  const boardList = boards.data ?? [];
  const board = boardList.find((b) => b.id === boardId) ?? boardList[0];
  const scrum = board?.type === "scrum";
  const sprints = useBoardSprints(activeId, scrum ? board.id : 0);
  // The picker offers the active and future sprints, which is exactly what
  // the sync fetched membership for, and defaults to the active one.
  const openSprints = (sprints.data ?? []).filter((s) => s.state !== "closed");
  const sprint = openSprints.find((s) => String(s.id) === sprintId)
    ?? openSprints.find((s) => s.state === "active")
    ?? openSprints[0];
  const effectiveSprintId = scrum && sprint ? String(sprint.id) : "";
  // A scrum board's real query waits for the sprint list to settle, so
  // exactly one correctly scoped board query fires per board: none of the
  // throwaway "" sprintId round trips a mount or a board switch used to
  // send while useBoardSprints was still loading.
  const sprintsReady = !scrum || sprints.isSuccess || sprints.isError;

  const view = useBoard(activeId, board?.id ?? 0, effectiveSprintId, swimlane, sprintsReady);
  const unavailable = useBoardsUnavailable(activeId);
  const syncState = useSyncState(activeId);
  const sync = useSyncBoards(activeId);

  const data = view.data;
  const moves = useBoardMoves({
    profileId: activeId,
    view: data,
    boardId: board?.id ?? 0,
    commit: lastCommit,
  });
  // A move is announced and focused only once the board has redrawn, so
  // the card is reported where it actually landed rather than where it was
  // sent.
  const flashKey = useMovedCard(data, !view.isFetching, moves.intent, (id) => {
    setFocusId(id);
    const card = bodyRef.current?.querySelector<HTMLElement>(`[data-board-pos="${id}"]`);
    // Focus follows the card only when the board already had it: a drag
    // made with the mouse must not pull focus out of wherever the user
    // put it.
    if (card && bodyRef.current?.contains(document.activeElement)) card.focus();
  });

  // Exactly one card is focusable, in every state: the one focus is on when
  // it survived the last change, else the selected card, else the first card
  // on the board.
  const focus = data ? clampFocus(data, focusId, findCard(data, selectedKey)) : "";

  const selectedPos = data ? findCard(data, selectedKey) : undefined;
  const selected = data ? cardAtPos(data, selectedPos) : undefined;

  function select(issue: Issue, id: string) {
    setSelectedKey(issue.key);
    setFocusId(id);
  }

  // openMenu is the keyboard's way into the card's move menu. Enter and
  // Space are taken by the selection that opens the detail panel, so the
  // menu key and Shift with F10 press the trigger the mouse presses, then
  // take focus into the panel it opened.
  function openMenu(id: string) {
    const card = bodyRef.current?.querySelector<HTMLElement>(`[data-board-pos="${id}"]`);
    card?.querySelector<HTMLElement>(`.${CARD_MENU_CLASS}`)?.click();
    card?.querySelector<HTMLElement>('[role="menuitem"]')?.focus();
  }

  function onKeyDown(e: KeyboardEvent, id: string) {
    if (!data) return;
    const p = parsePos(id);
    if (!p) return;
    const issue = cardAtPos(data, p);
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      if (issue) select(issue, id);
      return;
    }
    if (e.key === "ContextMenu" || (e.shiftKey && e.key === "F10")) {
      e.preventDefault();
      openMenu(id);
      return;
    }
    if (e.ctrlKey && issue && moves.moveByKeyboard(p, issue, e.key)) {
      e.preventDefault();
      return;
    }
    const next = moveFocus(data, p, e.key);
    if (!next) return;
    e.preventDefault();
    const nextId = posId(next);
    setFocusId(nextId);
    bodyRef.current?.querySelector<HTMLElement>(`[data-board-pos="${nextId}"]`)?.focus();
  }

  const refreshing = sync.isPending || (view.isFetching && !view.isLoading);
  // Two passes can have written these boards: this view's own Refresh and
  // the boards half of an ordinary sync. The banner reports whichever ran
  // last, so a Refresh's stale result never hides what a later sync found.
  const pass = sync.data && sync.submittedAt >= lastBoardsAt ? sync.data : lastBoards;

  return (
    <section className="backlog" aria-label="Boards">
      <BoardsToolbar
        boards={boardList}
        board={board}
        onBoard={(id) => {
          setBoardId(id);
          setSprintId("");
          setSelectedKey("");
          setFocusId("");
        }}
        sprints={openSprints}
        sprint={sprint}
        onSprint={(id) => {
          setSprintId(id);
          setSelectedKey("");
          setFocusId("");
        }}
        swimlane={swimlane}
        onSwimlane={(s) => {
          setSwimlane(s);
          setFocusId("");
        }}
        refreshing={refreshing}
        canRefresh={canSync && !sync.isPending}
        onRefresh={() => sync.mutate()}
      />

      <BoardsBanner
        error={sync.isError ? errMsg(sync.error) : ""}
        dropped={pass?.dropped ?? []}
        unavailable={!!pass?.unavailable}
        hasBoards={boardList.length > 0}
        canRetry={canSync && !sync.isPending}
        onRetry={() => sync.mutate()}
      />

      <BoardMoveBanner
        line={moves.warning?.line ?? ""}
        canPutBack={!!moves.warning?.row}
        busy={moves.busy}
        onPutBack={moves.putBack}
      />

      {data && <BoardSummaryLine view={data} sprint={sprint} lastSynced={syncState.data?.lastSynced ?? ""} />}

      <div className="boards-body" ref={bodyRef}>
        <div className="boards-pane">
          <BoardBody
            boards={boards}
            view={view}
            unavailable={!!unavailable.data}
            hasBoards={boardList.length > 0}
            hasSprint={!!effectiveSprintId}
            swimlane={swimlane}
            selectedKey={selectedKey}
            focusId={focus}
            moves={moves}
            flashKey={flashKey}
            sprints={openSprints}
            sprintId={effectiveSprintId}
            committing={status === "committing"}
            canSync={canSync && !sync.isPending}
            onSync={() => sync.mutate()}
            onSelect={(issue) => {
              const p = data ? findCard(data, issue.key) : undefined;
              select(issue, p ? posId(p) : "");
            }}
            onFocusCard={setFocusId}
            onKeyDown={onKeyDown}
          />
        </div>
        {selected && (
          <IssueDetailPanel key={selected.key} profileId={activeId} issue={selected} onClose={() => setSelectedKey("")} />
        )}
      </div>
    </section>
  );
}
