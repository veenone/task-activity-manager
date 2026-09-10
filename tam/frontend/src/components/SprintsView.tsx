import { useEffect, useMemo, useState } from "react";
import { announce, errMsg, useConfirm, useNotice, useProfile } from "@agile-suite/core";
import type { Issue, Profile, Settings, SprintDetail } from "../api";
import { UNASSIGNED_SPRINT_STATE } from "../api";
import { useBoard, useBoards, useJournalSprintMoves } from "../queries/boards";
import { usePendingChanges } from "../queries/pending";
import { useBoardSprintDetails, useDeleteSprint } from "../queries/sprints";
import { useSync } from "../contexts/SyncContext";
import { MOVED_FLASH_MS } from "../lib/flash";
import { dayOfSprint, plural, progressText } from "../lib/format";
import { journalTouchesSprint } from "../lib/moveValue";
import { unfinished } from "../lib/unfinished";
import { useCompleteGuard } from "./BoardCeremonies";
import { CompleteSprintModal } from "./CompleteSprintModal";
import { CreateSprintModal } from "./CreateSprintModal";
import { EditSprintModal } from "./EditSprintModal";
import { ImmediateWriteChip } from "./ImmediateWriteChip";
import { IssueDetailPanel } from "./IssueDetailPanel";
import { SprintFillBar } from "./SprintFillBar";
import { SprintList, issueOrder, rowIdOf } from "./SprintList";
import { StartSprintModal } from "./StartSprintModal";
import { useSprintSelection } from "./useSprintSelection";

// SprintsView is the board's sprints as a list rather than as a picker: every
// sprint the board has, the work in each one, and every action a sprint has,
// on one screen. The Boards view answers "where is this card"; this one
// answers "what is in this sprint, and what happens to it next".
export function SprintsView() {
  const { activeId, activeProfile } = useProfile<Profile, Settings>();
  const { runQuietLock } = useSync();
  const { confirm } = useConfirm();
  const { notice } = useNotice();
  const [boardId, setBoardId] = useState(0);
  const [showClosed, setShowClosed] = useState(false);
  const [selectedKey, setSelectedKey] = useState("");
  // The sentence the last sprint write left behind. A write here reaches
  // Jira at once and the dialog that made it closes on success, so the
  // banner is where the outcome, and any note riding with it, is read.
  const [line, setLine] = useState("");
  const [movedRowId, setMovedRowId] = useState("");
  // Which dialog is open. Each one holds the sprint it was opened on rather
  // than that sprint's id: the list is refetched underneath while a dialog
  // is up, and a dialog whose subject can vanish mid-edit is worse than one
  // describing the sprint as it was when it was opened.
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<SprintDetail | null>(null);
  const [starting, setStarting] = useState<SprintDetail | null>(null);
  const [completing, setCompleting] = useState<SprintDetail | null>(null);
  // The sprint whose own write is in flight, by row id, so its menu closes
  // for the duration and no other row's does.
  const [busyRowId, setBusyRowId] = useState("");

  // Everything above belongs to the profile it was chosen for, so a switch
  // clears it in the render that first sees the new id. An effect would be
  // one render too late: a read would go out pairing the new profile with
  // the old board id, and a dialog would still be describing a sprint from
  // the profile that is no longer on screen. The checked cards are cleared
  // the same way inside useSprintSelection, which does not exist yet at this
  // point in the render.
  const [madeFor, setMadeFor] = useState(activeId);
  if (madeFor !== activeId) {
    setMadeFor(activeId);
    setBoardId(0);
    setShowClosed(false);
    setSelectedKey("");
    setLine("");
    setMovedRowId("");
    setCreating(false);
    setEditing(null);
    setStarting(null);
    setCompleting(null);
    setBusyRowId("");
  }

  const boards = useBoards(activeId);
  // Only a scrum board has sprints at all, so a kanban board is not offered
  // here rather than offered and then explained.
  const scrumBoards = (boards.data ?? []).filter((b) => b.type === "scrum");
  const board = scrumBoards.find((b) => b.id === boardId) ?? scrumBoards[0];
  const details = useBoardSprintDetails(activeId, board?.id ?? 0);
  // The board's columns, read only while a completion is being prepared. The
  // completion dialog decides which cards are unfinished by the board's last
  // column, and the composed board is the one read that carries the columns;
  // this view otherwise never needs it, and pulling a board's cards on every
  // board switch to have a column list ready would cost more than it saves.
  // The scope is the board's own list, the smallest one the read takes.
  const boardView = useBoard(activeId, board?.id ?? 0, "", "none", !!completing);

  const all = useMemo(() => details.data ?? [], [details.data]);
  // The board's own unassigned work arrives as the last node of the same
  // list and is not a sprint: it is never filtered out by the closed toggle,
  // never counted as a sprint, and never started, edited or deleted.
  const sprints = all.filter((d) => d.state !== UNASSIGNED_SPRINT_STATE);
  const openSprints = sprints.filter((d) => d.state === "active" || d.state === "future");
  const visible = useMemo(
    () => (showClosed ? all : all.filter((d) => d.state !== "closed")),
    [all, showClosed],
  );
  const visibleSprints = visible.filter((d) => d.state !== UNASSIGNED_SPRINT_STATE);

  // One list of issue keys per sprint, which is what a shift gesture is
  // measured inside and what the fill sends.
  const order = useMemo(() => issueOrder(visible), [visible]);
  const selection = useSprintSelection(order, activeId);
  const fill = useJournalSprintMoves(activeId);
  // The profile's journal, read here for one sentence: the delete
  // confirmation has to know whether a pending move has moved the count it
  // is about to quote. It is the same query the shell's pending badge runs,
  // so this view joins a read that is already in the cache.
  const pending = usePendingChanges(activeId);
  const del = useDeleteSprint(activeId, runQuietLock);
  const askBeforeCompleting = useCompleteGuard(activeId);

  // The flash is cleared here rather than in the tree, so the id that caused
  // it cannot re-fire the effect when the same sprint is written twice.
  useEffect(() => {
    if (!movedRowId) return;
    const t = setTimeout(() => setMovedRowId(""), MOVED_FLASH_MS);
    return () => clearTimeout(t);
  }, [movedRowId]);

  const selected: Issue | undefined = visible
    .flatMap((d) => d.issues)
    .find((i) => i.key === selectedKey);

  function switchBoard(id: number) {
    setBoardId(id);
    setSelectedKey("");
    setLine("");
    setMovedRowId("");
    selection.reset();
  }

  // afterWrite is what every sprint write leaves behind: one sentence in the
  // banner, nothing checked, and the sprint it was about flashed and scrolled
  // to when there is one to point at.
  function afterWrite(rowId: string, sentence: string) {
    setLine(sentence);
    setMovedRowId(rowId);
    selection.reset();
  }

  function onFill(sprintId: string) {
    const keys = selection.keys;
    if (keys.length === 0 || fill.isPending) return;
    const to = openSprints.find((s) => String(s.id) === sprintId);
    const where = to ? to.name : "the backlog";
    fill.mutate(
      { keys, sprintId },
      {
        onSuccess: (moved) => {
          // Every card the cache holds is moved; one it does not hold is
          // skipped, and the count is how the difference is reported.
          const sentence = moved === keys.length
            ? `${plural(moved, "card", "cards")} moved to ${where}. Commit sends this to Jira.`
            : `${moved} of ${keys.length} cards moved to ${where}. The rest are not in this cache, so they were left where they are.`;
          announce(sentence);
          afterWrite("", sentence);
        },
        onError: (e) => void notice({ title: "The cards were not moved", message: errMsg(e), tone: "error" }),
      },
    );
  }

  async function askDelete(detail: SprintDetail) {
    // The count is a floor whenever this view knows it cannot be checked,
    // and three separate things make it so, not one. notSynced counts keys
    // this sprint holds that the issue cache does not, so the total is
    // short by exactly them. truncated means the shared card budget stopped
    // this sprint's list short, so the number cannot be checked against
    // what is on screen. And the third is this view's own doing: a pending
    // sprint move is replayed over the scope before the total is counted,
    // so a card the fill bar has journaled out of this sprint has already
    // left the count while Jira still holds it. The replay runs the other
    // way too, which is why any pending move naming this sprint counts,
    // whichever end of it the sprint is on: saying "at least" over a count
    // that turns out to be right costs a word, and quoting a count that
    // turns out to be low costs a sprint nobody can get back.
    const moving = journalTouchesSprint(pending.data ?? [], detail.id);
    const floor = detail.truncated || detail.notSynced > 0 || moving;
    const issues = plural(detail.total, "issue", "issues");
    const ok = await confirm({
      title: `Delete ${detail.name}?`,
      message: (
        <>
          <p>{`Jira deletes ${detail.name}.`}</p>
          <p>{`Jira moves ${floor ? `at least ${issues}` : `its ${issues}`} back to the backlog. The issues themselves are not deleted.`}</p>
          {floor && (
            <p className="muted small">
              {moving
                ? "Cards are waiting for Commit to move in or out of this sprint, so the number above is the one TAM holds and not the one Jira does."
                : detail.notSynced > 0
                  ? "Some of this sprint's issues are not in this cache, so it holds more than the number above."
                  : "This view stopped short of drawing all of this sprint's issues, so the number above cannot be checked against the list."}
            </p>
          )}
          <p>This cannot be undone, from TAM or from Jira.</p>
          <p><ImmediateWriteChip /></p>
        </>
      ),
      confirmLabel: "Delete sprint",
      cancelLabel: "Keep it",
      danger: true,
    });
    if (!ok) return;
    const rowId = rowIdOf(detail);
    setBusyRowId(rowId);
    del.mutate(
      { boardId: board?.id ?? 0, sprintId: detail.id },
      {
        onSuccess: (note) => {
          const deleted = `${detail.name} was deleted.`;
          const sentence = note ? `${deleted} ${note}` : deleted;
          announce(sentence);
          // Nothing to flash: the row the write was about is gone.
          afterWrite("", sentence);
        },
        // The refusals that arrive after the confirmation has closed have no
        // dialog left to live in. A missing Manage Sprints permission is the
        // common one, and it comes back long after the button was pressed.
        onError: (e) => void notice({ title: "The sprint was not deleted", message: errMsg(e), tone: "error" }),
        onSettled: () => setBusyRowId(""),
      },
    );
  }

  async function askComplete(detail: SprintDetail) {
    // The same guard the Boards toolbar asks first: a card dragged to Done
    // an hour ago is Done here and not in Jira, and completing the sprint
    // would send it to the backlog as unfinished.
    if (await askBeforeCompleting(detail.id)) setCompleting(detail);
  }

  const active = sprints.find((d) => d.state === "active");
  // The line carries the qualification the tree prints under an expanded
  // sprint, for the same reason it prints it: a key this sprint holds that
  // the cache does not is counted in neither half of the progress. This
  // line is on screen whether or not the sprint is open, so quoting the
  // numbers bare here would claim a completeness the tree refuses to claim
  // about the same sprint.
  const summary = active
    ? [
        active.name,
        dayOfSprint(active.startDate, active.endDate),
        active.total > 0 ? progressText(active.done, active.total, active.donePoints, active.points) : "",
        active.notSynced > 0
          ? `plus ${plural(active.notSynced, "card", "cards")} this cache does not hold`
          : "",
      ].filter(Boolean).join(", ")
    : "No sprint is running on this board.";

  function tree() {
    if (boards.isError) {
      return (
        <p className="error-text">
          Could not load the boards: {boards.error.message}{" "}
          <button type="button" className="btn" onClick={() => void boards.refetch()}>Retry</button>
        </p>
      );
    }
    if (boards.isLoading) return <p className="muted">Loading the boards</p>;
    if (!board) {
      return <p className="muted">No scrum board has been synced for this project, so there are no sprints to show.</p>;
    }
    if (details.isError) {
      return (
        <p className="error-text">
          Could not load this board's sprints: {details.error.message}{" "}
          <button type="button" className="btn" onClick={() => void details.refetch()}>Retry</button>
        </p>
      );
    }
    if (details.isLoading) return <p className="muted">Loading the sprints</p>;
    if (sprints.length === 0) {
      return <p className="muted">This board has no sprints yet. New sprint makes the first one.</p>;
    }
    if (visibleSprints.length === 0) {
      return <p className="muted">Every sprint on this board is closed. Show closed sprints brings them back.</p>;
    }
    return (
      <SprintList
        details={visible}
        selectedKey={selectedKey}
        onSelect={setSelectedKey}
        checked={selection.checked}
        onCheck={selection.check}
        onExtend={selection.extendTo}
        onClearTo={selection.clearTo}
        movedRowId={movedRowId}
        busyRowId={busyRowId}
        onStart={setStarting}
        onComplete={(d) => void askComplete(d)}
        onEdit={setEditing}
        onDelete={(d) => void askDelete(d)}
      />
    );
  }

  return (
    <section className="backlog" aria-label="Sprints">
      <div className="board-head">
        {/* One scrum board needs no picker, and a select holding one option
            is a control that cannot be used. The board is still named, since
            the sprints on screen belong to it and nothing else says so. */}
        {scrumBoards.length > 1 ? (
          <label className="board-picker">
            <span>Board</span>
            <select aria-label="Board" value={board?.id ?? ""} onChange={(e) => switchBoard(Number(e.target.value))}>
              {scrumBoards.map((b) => (
                <option key={b.id} value={b.id}>{b.name}</option>
              ))}
            </select>
          </label>
        ) : (
          board && <h2 className="board-head-name">{board.name}</h2>
        )}

        <label className="check-row" htmlFor="sprints-show-closed">
          <input
            id="sprints-show-closed"
            type="checkbox"
            checked={showClosed}
            onChange={(e) => {
              setShowClosed(e.target.checked);
              // A closed sprint's cards leaving the tree would leave them
              // checked and invisible, which checkedIn would then quietly
              // drop from the next fill. Clearing says so instead.
              selection.reset();
            }}
          />
          Show closed sprints
        </label>

        <div className="board-head-actions">
          {details.isFetching && !details.isLoading && <span className="muted small">Refreshing</span>}
          <button type="button" className="btn" disabled={!board} onClick={() => setCreating(true)}>New sprint</button>
        </div>
      </div>

      {/* Named, because the sentence is announced through the shared live
          region as well, and the two status regions are otherwise the same
          thing to anything reading the page. */}
      {line && (
        <div className="pending-banner" role="status" aria-label="Sprint outcome">
          <p>{line}</p>
        </div>
      )}

      {board && !details.isLoading && !details.isError && <p className="muted board-summary">{summary}</p>}

      {selection.count > 0 && (
        <SprintFillBar
          count={selection.count}
          sprints={openSprints}
          busy={fill.isPending}
          onFill={onFill}
          onClear={selection.reset}
        />
      )}

      <div className="epics-body">
        <div className="epics-tree-pane">{tree()}</div>
        {/* A panel describing one card while three are checked would be
            lying about what the next action touches, so a multi-selection
            takes the pane. The pane is kept at the panel's own width for as
            long as anything is checked, panel or no panel: the tree is
            sized by what is beside it, so letting it widen the moment the
            panel left would slide every row sideways in the middle of the
            gesture that was checking them. That includes a first row
            checked with Space, which selects nothing and so opens no panel
            of its own. */}
        {selected && selection.count <= 1 ? (
          <IssueDetailPanel
            key={selected.key}
            profileId={activeId}
            issue={selected}
            jiraUrl={activeProfile?.jiraUrl}
            // This board's open sprints, handed over empty or not. An empty
            // list here is not "the profile has no open sprints", the way
            // the Backlog's profile-wide list can say, so no emptyNote goes
            // with it.
            sprints={openSprints}
            onClose={() => setSelectedKey("")}
          />
        ) : (
          selection.count > 0 && <div className="detail-panel-reserve" aria-hidden="true" />
        )}
      </div>

      {creating && board && (
        <CreateSprintModal
          profileId={activeId}
          boardId={board.id}
          onClose={() => setCreating(false)}
          onCreated={(sprintId, sentence) => {
            // A create can come back with no id, which core/jira treats as
            // "made, go and refresh" rather than as a failure, and then
            // there is no row to point at.
            afterWrite(sprintId ? `sprint:${sprintId}` : "", sentence);
          }}
        />
      )}

      {editing && board && (
        <EditSprintModal
          profileId={activeId}
          boardId={board.id}
          sprint={editing}
          otherNames={sprints.filter((s) => s.id !== editing.id).map((s) => s.name)}
          onClose={() => setEditing(null)}
          onEdited={(sentence) => afterWrite(rowIdOf(editing), sentence)}
        />
      )}

      {starting && board && (
        <StartSprintModal
          profileId={activeId}
          boardId={board.id}
          sprint={starting}
          active={sprints.find((s) => s.state === "active" && s.id !== starting.id)}
          onClose={() => setStarting(null)}
          onStarted={(_, sentence) => afterWrite(rowIdOf(starting), sentence)}
        />
      )}

      {completing && board && (
        boardView.data ? (
          <CompleteSprintModal
            profileId={activeId}
            boardId={board.id}
            sprint={completing}
            futures={sprints.filter((s) => s.state === "future")}
            incomplete={unfinished(completing.issues, boardView.data.columns)}
            // What the list above cannot name: this sprint's keys the cache
            // does not hold, and the cards the shared budget stopped it
            // drawing. Jira's own re-read at completion time sees both.
            hidden={completing.notSynced + (completing.total - completing.issues.length)}
            lastColumn={boardView.data.columns[boardView.data.columns.length - 1]?.name ?? "the last column"}
            onClose={() => setCompleting(null)}
            onCompleted={(_, sentence) => afterWrite(rowIdOf(completing), sentence)}
          />
        ) : boardView.isError ? (
          <p className="error-text" role="alert">
            {`This board's columns could not be read (${boardView.error.message}), and a completion cannot say which cards are unfinished without them.`}
          </p>
        ) : (
          <p className="muted" role="status">Reading this board's columns.</p>
        )
      )}
    </section>
  );
}
