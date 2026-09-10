import { useMemo, useState } from "react";
import type { MouseEvent } from "react";
import { errMsg, useProfile } from "@agile-suite/core";
import type { Issue, Profile, Settings, Swimlane } from "../api";
import { useBoard, useBoardSprints, useBoards, useBoardsUnavailable, useSyncBoards } from "../queries/boards";
import { useSyncState } from "../queries/issues";
import { filterBoard } from "../lib/boardFilter";
import { useSync } from "../contexts/SyncContext";
import { cardAtPos, cardKeys, findCard, posId } from "../lib/boardCells";
import { BoardBody } from "./BoardBody";
import { BoardCeremonies, useCompleteGuard } from "./BoardCeremonies";
import type { Ceremony } from "./BoardCeremonies";
import { CreateSprintModal } from "./CreateSprintModal";
import { BoardSelectionBar } from "./BoardSelectionBar";
import { useBoardSelection } from "./useBoardSelection";
import { useBoardKeys } from "./useBoardKeys";
import { BoardMoveBanner } from "./BoardMoveBanner";
import { BoardsBanner } from "./BoardsBanner";
import { BoardSummaryLine } from "./BoardNotes";
import { BoardsToolbar } from "./BoardsToolbar";
import { IssueDetailPanel } from "./IssueDetailPanel";
import { useBoardMoves } from "./useBoardMoves";

// BoardsView is the board: XTM's board head over the board's own columns,
// the cards in them, and the lines that say what the board is not showing.
// Phase 3b makes it writable, through the journal: a drag, a key press, or
// the card's own menu moves a card, and Commit is what pushes any of it.
// Phase 3c adds the ceremonies beside it: a selection of cards moved into a
// sprint in one gesture, and the two dialogs that start and complete one,
// which are the only writes here that do not wait for Commit.
export function BoardsView() {
  const { activeId } = useProfile<Profile, Settings>();
  const { canSync, lastBoards, lastCommit, runBoardsRefresh, status } = useSync();
  // A commit in flight is the one thing that holds a move back, and it
  // holds all three of them back: the drag, the menu, and the keys.
  const committing = status === "committing";
  const [boardId, setBoardId] = useState(0);
  const [sprintId, setSprintId] = useState("");
  const [swimlane, setSwimlane] = useState<Swimlane>("none");
  const [selectedKey, setSelectedKey] = useState("");
  const [focusId, setFocusId] = useState("");
  // Which ceremony dialog is open, and the sentence the last one to finish
  // left behind. A completion is irreversible and its result is the one
  // thing on this screen worth keeping until the user moves on.
  const [ceremony, setCeremony] = useState<Ceremony>("");
  const [ceremonyLine, setCeremonyLine] = useState("");
  // Whether the New sprint dialog is open. It is its own flag rather than a
  // third Ceremony value: BoardCeremonies bails out with nothing rendered
  // when no sprint is on screen, and a create needs no sprint already
  // picked, so it cannot share that component's early return.
  const [creatingSprint, setCreatingSprint] = useState(false);

  // The board, sprint, and swimlane choices belong to the profile they were
  // made for, so a switch clears them in the render that first sees the new
  // id. An effect would be one render too late: a board query would go out
  // pairing the new profile with the old board id. The checked cards are
  // cleared the same way, inside useBoardSelection, which does not exist yet
  // at this point in the render.
  const [filtersFor, setFiltersFor] = useState(activeId);
  if (filtersFor !== activeId) {
    setFiltersFor(activeId);
    setBoardId(0);
    setSprintId("");
    setSwimlane("none");
    setSelectedKey("");
    setFocusId("");
    setCeremony("");
    setCeremonyLine("");
    setCreatingSprint(false);
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
  const sync = useSyncBoards(activeId, runBoardsRefresh);
  // The filter is a reading aid over the drawn board, not a query: it never
  // refetches, so clearing it costs nothing and a filtered board still holds
  // every card the sync pulled.
  const [filter, setFilter] = useState("");

  // Filtered before anything reads it, so the cells, the counts, the
  // keyboard walk, and the drop targets all agree about which cards exist.
  const data = useMemo(() => (view.data ? filterBoard(view.data, filter) : view.data), [view.data, filter]);
  // The board in reading order, which is what a shift gesture measures a run
  // against and the order a bulk move sends its keys in. It is the drawn
  // board, filter and all, so the selection can only ever hold cards the
  // reader can see.
  const order = useMemo(() => (data ? cardKeys(data) : []), [data]);
  // The selection resets itself on a profile switch, from the id handed in
  // here: the block above runs before this hook exists, so it cannot clear
  // something it has not built yet.
  const selection = useBoardSelection(order, activeId);
  const askBeforeCompleting = useCompleteGuard(activeId);
  const moves = useBoardMoves({
    profileId: activeId,
    view: data,
    boardId: board?.id ?? 0,
    commit: lastCommit,
    committing,
  });
  // The focus model and the key map, which are their own concern: the board
  // is a grid with one tab stop, and a move unmounts the card focus was on.
  const { bodyRef, focus, flashKey, onKeyDown, rememberFocus } = useBoardKeys({
    data,
    fetching: view.isFetching,
    moves,
    selection,
    selectedKey,
    focusId,
    setFocusId,
    onOpen: (issue, id) => {
      selection.clearTo(issue.key);
      select(issue, id);
    },
  });

  const selectedPos = data ? findCard(data, selectedKey) : undefined;
  const selected = data ? cardAtPos(data, selectedPos) : undefined;

  function select(issue: Issue, id: string) {
    setSelectedKey(issue.key);
    setFocusId(id);
  }

  // A click on a card is one of three gestures. Control toggles the card's
  // check and shift extends the run from the anchor; neither opens the
  // panel, since neither is about one card. A plain click is the one that
  // is, so it clears the multi-selection and leaves the clicked card as the
  // anchor the next shift gesture measures from.
  function clickCard(issue: Issue, e: MouseEvent<HTMLDivElement>) {
    const p = data ? findCard(data, issue.key) : undefined;
    const id = p ? posId(p) : "";
    if (e.shiftKey) {
      selection.extendTo(issue.key);
      setFocusId(id);
      return;
    }
    if (e.ctrlKey || e.metaKey) {
      selection.check(issue.key);
      setFocusId(id);
      return;
    }
    selection.clearTo(issue.key);
    select(issue, id);
  }

  // afterCeremony is what the two ceremonies and a create all leave behind:
  // the picker on the sprint the action was about, nothing checked, and one
  // sentence saying what happened. A create shares it rather than getting
  // its own handler because there is nothing about the aftermath that is
  // specific to how the sprint came to exist.
  function afterCeremony(nextSprintId: string, line: string) {
    setCeremony("");
    setCeremonyLine(line);
    selection.reset();
    setSelectedKey("");
    setFocusId("");
    if (nextSprintId) setSprintId(nextSprintId);
  }

  const refreshing = sync.isPending || (view.isFetching && !view.isLoading);
  // Two passes can have written these boards: this view's own Refresh and
  // the boards half of an ordinary sync. The banner reports whichever ran
  // last, so a Refresh's stale result never hides what a later sync found.
  // Both passes now land in the same place: the Refresh writes its summary
  // through SyncContext, exactly where the sync's own boards half writes
  // one, so the banner reads one channel instead of racing two.
  const pass = lastBoards;

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
          selection.reset();
          setCeremonyLine("");
        }}
        sprints={openSprints}
        sprint={sprint}
        onSprint={(id) => {
          setSprintId(id);
          setSelectedKey("");
          setFocusId("");
          selection.reset();
          setCeremonyLine("");
        }}
        swimlane={swimlane}
        onSwimlane={(s) => {
          setSwimlane(s);
          setFocusId("");
          selection.reset();
        }}
        refreshing={refreshing}
        canRefresh={canSync && !sync.isPending}
        onRefresh={() => sync.mutate()}
        filter={filter}
        onFilter={setFilter}
        onStart={() => setCeremony("start")}
        onComplete={() => {
          if (!sprint) return;
          void askBeforeCompleting(sprint.id).then((ok) => {
            if (ok) setCeremony("complete");
          });
        }}
        onCreate={() => setCreatingSprint(true)}
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

      {/* Named, because the sentence is also announced through the shared
          live region and the two status regions are otherwise the same
          thing to anything reading the page. */}
      {ceremonyLine && (
        <div className="pending-banner" role="status" aria-label="Sprint ceremony">
          <p>{ceremonyLine}</p>
        </div>
      )}

      {data && <BoardSummaryLine view={data} sprint={sprint} lastSynced={syncState.data?.lastSynced ?? ""} />}

      {selection.count > 0 && (
        <BoardSelectionBar
          count={selection.count}
          sprints={openSprints}
          sprintId={effectiveSprintId}
          busy={moves.movingMany || committing}
          onMove={(target) => moves.moveManyToSprint(selection.keys, target, selection.reset)}
          onClear={selection.reset}
        />
      )}

      <div className="boards-body" ref={bodyRef} onFocusCapture={rememberFocus} onBlurCapture={rememberFocus}>
        <div className="boards-pane">
          <BoardBody
            boards={boards}
            view={view}
            filtered={data}
            unavailable={!!unavailable.data}
            hasBoards={boardList.length > 0}
            hasSprint={!!effectiveSprintId}
            swimlane={swimlane}
            selectedKey={selectedKey}
            checked={selection.checked}
            focusId={focus}
            moves={moves}
            flashKey={flashKey}
            sprints={openSprints}
            sprintId={effectiveSprintId}
            committing={committing}
            canSync={canSync && !sync.isPending}
            onSync={() => sync.mutate()}
            onSelect={clickCard}
            onFocusCard={setFocusId}
            onKeyDown={onKeyDown}
          />
        </div>
        {/* A panel describing one card while three are checked would be
            lying about what the next action touches, so the multi-selection
            takes the pane; dropping back to one card gives it back. */}
        {selected && selection.count <= 1 && (
          <IssueDetailPanel
            key={selected.key}
            profileId={activeId}
            issue={selected}
            // Handed over unconditionally, empty or not: an empty list here
            // is not "the profile has no open sprints", the way the Backlog
            // and Epics tree's profile-wide list can say. It is also a
            // kanban board's disabled sprint query, a scrum board whose list
            // has not loaded yet, or one whose sprints are all closed right
            // after a completion, and none of those should turn the field
            // read-only: the backlog is still a destination the panel can
            // reach. Passing no emptyNote is what keeps this view silent
            // about why the list is empty, which only the profile-wide
            // callers are in a position to explain.
            sprints={openSprints}
            onClose={() => setSelectedKey("")}
          />
        )}
      </div>

      <BoardCeremonies
        profileId={activeId}
        boardId={board?.id ?? 0}
        open={ceremony}
        sprint={sprint}
        sprints={openSprints}
        view={view.data}
        onClose={() => setCeremony("")}
        onStarted={afterCeremony}
        onCompleted={afterCeremony}
      />

      {creatingSprint && (
        <CreateSprintModal
          profileId={activeId}
          boardId={board?.id ?? 0}
          onClose={() => setCreatingSprint(false)}
          onCreated={afterCeremony}
        />
      )}
    </section>
  );
}
