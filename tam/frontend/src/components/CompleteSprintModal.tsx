import { useState } from "react";
import { Modal, announce, errMsg } from "@agile-suite/core";
import type { Issue, Sprint } from "../api";
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
  profileId, boardId, sprint, futures, incomplete, lastColumn, onClose, onCompleted,
}: Props) {
  const { runSprintCeremony } = useSync();
  const complete = useCompleteSprint(profileId, runSprintCeremony);
  const [moveTo, setMoveTo] = useState("");
  const [error, setError] = useState("");
  const dates = sprintDates(sprint);
  const destination = futures.find((s) => String(s.id) === moveTo);
  const where = destination ? destination.name : "the backlog";

  function onSubmit() {
    if (complete.isPending) return;
    setError("");
    complete.mutate(
      { boardId, sprintId: sprint.id, moveTo },
      {
        onSuccess: (done) => {
          const line = done.moved > 0
            ? `${sprint.name} is closed. ${plural(done.moved, "unfinished card", "unfinished cards")} moved to ${done.movedTo}.`
            : `${sprint.name} is closed, with nothing left unfinished.`;
          announce(line);
          onCompleted(moveTo, line);
          onClose();
        },
        // A completion that fell over has usually already moved some of the
        // cards, and Go's own sentence is the only thing that knows how
        // many. It is printed word for word, above the list of the cards it
        // was moving.
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

        {error && (
          <p className="error-text" role="alert">
            {error}
          </p>
        )}

        {incomplete.length === 0 ? (
          <p className="muted">Every card on this board's sprint is finished, so nothing moves.</p>
        ) : (
          <>
            <p>{`${plural(incomplete.length, "card is", "cards are")} not finished and will move out of the sprint:`}</p>
            <div className="pending-list sprint-incomplete">
              {incomplete.map((issue) => (
                <div key={issue.key} className="pending-card sprint-incomplete-card">
                  <span className="b accent-text">{issue.key}</span>
                  <span className="pending-card-summary">{issue.summary}</span>
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
        <span className="muted small">
          {incomplete.length === 0
            ? "Nothing moves; the sprint is closed as it is."
            : `${plural(incomplete.length, "card", "cards")} move to ${where}.`}
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
