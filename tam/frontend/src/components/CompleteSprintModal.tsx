import { useState } from "react";
import { Modal, announce, errMsg } from "@agile-suite/core";
import type { Issue, Sprint, SprintCompletion } from "../api";
import { useCompleteSprint } from "../queries/boards";
import { useSync } from "../contexts/SyncContext";
import { plural } from "../lib/format";

interface Props {
  profileId: string;
  boardId: number;
  // sprint is the active sprint being completed.
  sprint: Sprint;
  // futures are this board's future sprints, the only destinations besides
  // the backlog. A closed sprint is not one, and the sprint being completed
  // is not one either.
  futures: Sprint[];
  // incomplete are the cards this board is not showing in its last column,
  // which is what "not finished" means here and on the wire. They are the
  // board as TAM last synced it: Jira is re-read at the moment of the
  // completion, so this list is what the user is deciding about rather than
  // a promise about what will move.
  incomplete: Issue[];
  // hidden is how many of this board's cards are in no drawn cell at all, so
  // the list above cannot name them however honest it is about the rest: the
  // cards past a cell's or the view's cap, the ones the sync has not
  // fetched, and the ones whose status no column collects. Jira's own re-read
  // sees all of them.
  hidden: number;
  // lastColumn names the column that decides it, so the rule is on the
  // dialog and not only in the plan that chose it.
  lastColumn: string;
  onClose: () => void;
  // onCompleted hands back the destination, so the board's picker moves to
  // the sprint the cards went to rather than falling back to whatever is
  // first, and the sentence the banner prints.
  onCompleted: (moveTo: string, line: string) => void;
}

// day renders a sprint's own start or end as a calendar day, the way the
// board's summary line does.
function day(iso: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleDateString(undefined, { day: "numeric", month: "short" });
}

function sprintDates(s: Sprint): string {
  const from = day(s.startDate);
  const to = day(s.endDate);
  if (from && to) return `${from} to ${to}`;
  return to ? `ends ${to}` : from ? `started ${from}` : "";
}

// CompleteSprintModal closes one sprint on Jira and says, before it does,
// exactly which cards that moves and where they go.
//
// It lists the incomplete cards rather than counting them. This is the one
// action in TAM that TAM cannot undo, and "12 issues will move to the
// backlog" is a claim the user has no way to check against a board of
// forty; Jira's own dialog names them, and so does this one.
export function CompleteSprintModal({
  profileId, boardId, sprint, futures, incomplete, hidden, lastColumn, onClose, onCompleted,
}: Props) {
  const { runSprintCeremony } = useSync();
  const complete = useCompleteSprint(profileId, runSprintCeremony);
  const [moveTo, setMoveTo] = useState("");
  const [error, setError] = useState("");
  // stopped is a completion that came back saying it did not finish. It is
  // not an error: it reached Jira and moved cards, and from here on the
  // dialog is about what it did rather than about what it was going to do.
  // Two failures arrive this way, and the second has an empty failed list:
  // a push that stopped partway, which names the cards still in the sprint,
  // and a close Jira refused after every card had already left it, which
  // has none to name.
  const [stopped, setStopped] = useState<SprintCompletion | null>(null);
  const dates = sprintDates(sprint);
  const destination = futures.find((s) => String(s.id) === moveTo);
  const where = destination ? destination.name : "the backlog";
  // The rows on screen: the cards that did not move once a completion has
  // stopped, the board's own unfinished cards until then. A key Jira named
  // and this board never drew is listed as the bare key rather than dropped,
  // which is the case the hidden count warns about coming true.
  const summaries = new Map(incomplete.map((i) => [i.key, i.summary]));
  const rows = stopped
    ? stopped.failed.map((key) => ({ key, summary: summaries.get(key) ?? "" }))
    : incomplete.map((i) => ({ key: i.key, summary: i.summary }));

  function onSubmit() {
    if (complete.isPending) return;
    setError("");
    setStopped(null);
    complete.mutate(
      { boardId, sprintId: sprint.id, moveTo },
      {
        onSuccess: (done) => {
          // A completion that reached Jira and then failed comes back
          // rather than throwing, because what it did is what the user
          // needs and Wails drops a value that travels with an error. The
          // sprint is open either way, so the dialog stays open and stops
          // describing a move it has already made: a push that stopped
          // names the cards still in the sprint, and a refused close has
          // none to name and says so with its message alone.
          if (done.message) {
            setStopped(done);
            return;
          }
          const line = done.moved > 0
            ? `${sprint.name} is closed. ${plural(done.moved, "unfinished card", "unfinished cards")} moved to ${done.movedTo}.`
            : `${sprint.name} is closed, with nothing left unfinished.`;
          announce(line);
          onCompleted(moveTo, line);
          onClose();
        },
        // What is left as an error is the refusals that happen before
        // anything moves, and each of those is a sentence that stands on
        // its own with no cards to report beside it.
        onError: (e) => setError(errMsg(e)),
      },
    );
  }

  return (
    <Modal onClose={onClose} className="modal pending-modal" labelledBy="complete-sprint-title" closeOnOverlayClick={false}>
      <div className="pending-head">
        <h2 id="complete-sprint-title">{`Complete ${sprint.name}`}</h2>
        {dates && <span className="muted">{dates}</span>}
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>

      <div className="bulk-body">
        <div className="pending-banner pending-banner-warn">
          <p>Jira closes the sprint. This cannot be reversed from TAM.</p>
          <p className="muted small">{`A card counts as finished when it sits in ${lastColumn}, this board's last column.`}</p>
        </div>

        {(error || stopped) && (
          <p className="error-text" role="alert">
            {error || stopped?.message}
          </p>
        )}

        {/* What this list is, and what it cannot be. Jira decides the
            completion from its own read, so the board can only ever be the
            last thing TAM saw, and the cards it is not drawing are not in
            the list however carefully the rest of it is built. Both
            sentences are about a move that has not happened yet, so both go
            once one has. */}
        {!stopped && (
          <p className="muted small">
            This is the board as TAM last synced it. Jira is re-read when the sprint is completed, and that read is what decides which cards move.
          </p>
        )}
        {!stopped && hidden > 0 && (
          <p className="muted small">
            {`${plural(hidden, "card", "cards")} on this board ${hidden === 1 ? "is" : "are"} not drawn (a status no column collects, a card the sync has not fetched, or a cell past what it renders), so the list cannot name ${hidden === 1 ? "it" : "them"} and more than these can move.`}
          </p>
        )}

        {/* Nothing to list, and nothing to promise. A completion that
            stopped with no card left behind is the close that was refused
            after every card had already moved: the message above is the
            whole story, and a list headed "will move out of the sprint"
            under it would be describing a move that has happened. */}
        {rows.length === 0 ? (
          stopped ? null : (
            <p className="muted">Every card on this board's sprint is finished, so nothing moves.</p>
          )
        ) : (
          <>
            <p>
              {stopped
                ? `${plural(rows.length, "card", "cards")} did not move and ${rows.length === 1 ? "is" : "are"} still in ${sprint.name}:`
                : `${plural(rows.length, "card is", "cards are")} not finished and will move out of the sprint:`}
            </p>
            <div className="pending-list sprint-incomplete">
              {rows.map((row) => (
                <div key={row.key} className="pending-card sprint-incomplete-card">
                  <span className="b accent-text">{row.key}</span>
                  <span className="pending-card-summary">{row.summary}</span>
                </div>
              ))}
            </div>
            <label className="edit-row" htmlFor="complete-sprint-destination">
              <span className="muted small">Move them to</span>
              <select
                id="complete-sprint-destination"
                aria-label="Move them to"
                className="detail-input"
                value={moveTo}
                onChange={(e) => setMoveTo(e.target.value)}
              >
                <option value="">The backlog</option>
                {futures.map((s) => (
                  <option key={s.id} value={String(s.id)}>{s.name}</option>
                ))}
              </select>
            </label>
          </>
        )}
      </div>

      <div className="pending-actions">
        {/* An outcome once there is one to report, and a promise only while
            the promise is still the truth. */}
        <span className="muted small">
          {stopped
            ? `${plural(stopped.moved, "card", "cards")} moved to ${stopped.movedTo}.`
            : rows.length === 0
              ? "Nothing moves; the sprint is closed as it is."
              : `${plural(rows.length, "card", "cards")} move to ${where}.`}
        </span>
        <span className="pending-footer-buttons">
          <button type="button" className="btn" onClick={onClose} disabled={complete.isPending}>Cancel</button>
          <button type="button" className="btn btn-primary" disabled={complete.isPending} onClick={onSubmit}>
            {complete.isPending ? "Completing" : "Complete sprint"}
          </button>
        </span>
      </div>
    </Modal>
  );
}
