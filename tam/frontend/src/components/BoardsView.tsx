import { useRef, useState } from "react";
import type { KeyboardEvent } from "react";
import { useProfile } from "@agile-suite/core";
import type { BoardView, Issue, Profile, Settings, Swimlane } from "../api";
import { useBoard, useBoardSprints, useBoards, useBoardsUnavailable, useSyncBoards } from "../queries/boards";
import { useSyncState } from "../queries/issues";
import { useSync } from "../contexts/SyncContext";
import { clampFocus, findCard, moveFocus, parsePos, posId } from "../lib/boardCells";
import type { Pos } from "../lib/boardCells";
import { plural } from "../lib/format";
import { BoardBody } from "./BoardBody";
import { BoardSummaryLine } from "./BoardNotes";
import { BoardsToolbar } from "./BoardsToolbar";
import { IssueDetailPanel } from "./IssueDetailPanel";

// cardAtPos reads one card out of the view by its position.
function cardAtPos(view: BoardView, p: Pos | undefined): Issue | undefined {
  if (!p) return undefined;
  return view.lanes[p.lane]?.cells[p.col]?.[p.index];
}

// BoardsView is the read-only board: XTM's board head over the board's own
// columns, the cards in them, and the lines that say what the board is not
// showing. Phase 3a draws; nothing here writes.
export function BoardsView() {
  const { activeId } = useProfile<Profile, Settings>();
  const { canSync } = useSync();
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

  const view = useBoard(activeId, board?.id ?? 0, effectiveSprintId, swimlane);
  const unavailable = useBoardsUnavailable(activeId);
  const syncState = useSyncState(activeId);
  const sync = useSyncBoards(activeId);
  const dropped = sync.data?.dropped ?? [];

  const data = view.data;
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

  function onKeyDown(e: KeyboardEvent, id: string) {
    if (!data) return;
    const p = parsePos(id);
    if (!p) return;
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      const issue = cardAtPos(data, p);
      if (issue) select(issue, id);
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

      {dropped.length > 0 && (
        <div className="pending-banner">
          <p>{`${plural(dropped.length, "board", "boards")} ${dropped.length === 1 ? "was" : "were"} skipped: ${dropped.join(", ")}`}</p>
        </div>
      )}

      {data && <BoardSummaryLine view={data} sprint={sprint} lastSynced={syncState.data?.lastSynced ?? ""} />}

      <div className="boards-body" ref={bodyRef}>
        <div className="boards-pane">
          <BoardBody
            boards={boards}
            view={view}
            unavailable={!!unavailable.data}
            hasBoards={boardList.length > 0}
            swimlane={swimlane}
            selectedKey={selectedKey}
            focusId={focus}
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
