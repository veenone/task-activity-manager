import { useState } from "react";
import { Modal, announce, errMsg, toPlainText } from "@agile-suite/core";
import type { Issue, Sprint } from "../api";
import { useCompleteSprint } from "../queries/boards";
import { useSync } from "../contexts/SyncContext";
import { plural, sprintDates } from "../lib/format";

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
  // board as TAM last synced it, read from the cache: Commit reads Jira
  // again and moves what is unfinished then, so this list is a preview.
  incomplete: Issue[];
  // hidden is how many of this board's cards are in no drawn cell at all, so
  // the list above cannot name them however honest it is about the rest: the
  // cards past a cell's or the view's cap, the ones the sync has not
  // fetched, and the ones whose status no column collects. Commit's read of
  // Jira sees all of them.
  hidden: number;
  // lastColumn names the column that decides it, so the rule is on the
  // dialog and not only in the plan that chose it.
  lastColumn: string;
  onClose: () => void;
  // onCompleted hands back the sprint the completion was saved for, so the
  // board keeps its picker there, and the sentence the banner prints.
  onCompleted: (sprintId: string, line: string) => void;
}

// CompleteSprintModal saves a completion of one sprint for Commit, and says
// which cards it would move if Commit ran now and where they go.
//
// The count is "about": nothing here calls Jira, so the list is the cached
// board, and Commit works the unfinished set out again from Jira at the
// moment it closes the sprint. The Commit result is where the real number
// is reported. The list still names the cards rather than counting them,
// because a number alone is a claim the user has no way to check.
export function CompleteSprintModal({
  profileId, boardId, sprint, futures, incomplete, hidden, lastColumn, onClose, onCompleted,
}: Props) {
  const { runQuietLock } = useSync();
  const complete = useCompleteSprint(profileId, runQuietLock);
  const [moveTo, setMoveTo] = useState("");
  const dates = sprintDates(sprint);
  const destination = futures.find((s) => String(s.id) === moveTo);
  const where = destination ? destination.name : "the backlog";
  const n = incomplete.length;

  function onSubmit() {
    if (complete.isPending) return;
    complete.mutate(
      { boardId, sprintId: sprint.id, moveTo },
      {
        onSuccess: () => {
          const line = `${sprint.name} will be completed on Commit. Unfinished cards move to ${where}.`;
          announce(line);
          onCompleted(String(sprint.id), line);
          onClose();
        },
      },
    );
  }

  return (
    <Modal onClose={onClose} className="modal pending-modal" labelledBy="complete-sprint-title" closeOnOverlayClick={false}>
      <div className="pending-head">
        <div className="edit-sprint-title">
          <h2 id="complete-sprint-title">{`Complete ${sprint.name}`}</h2>
          <p>Saved locally. Commit completes the sprint in Jira.</p>
        </div>
        {dates && <span className="muted">{dates}</span>}
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>

      <div className="bulk-body">
        <div className="pending-banner pending-banner-warn">
          <p>Once Commit sends it, Jira closes the sprint. That cannot be reversed from TAM.</p>
          <p className="muted small">{`A card counts as finished when it sits in ${lastColumn}, this board's last column.`}</p>
        </div>

        {/* A refusal is a sentence that stands on its own: pending changes
            on cards in the sprint, a sprint that never started, a delete
            waiting for Commit, or the busy guard. */}
        {complete.error && <p className="error-text" role="alert">{errMsg(complete.error)}</p>}

        <p>
          {n === 0
            ? "No card in this sprint looks unfinished right now."
            : `About ${plural(n, "card is", "cards are")} not finished.`}
          {" The exact set is worked out again on Commit, and the Commit result reports how many moved."}
        </p>
        {n > 0 && (
          <>
            <p className="muted small">The list is the board as TAM last synced it.</p>
            <div className="pending-list sprint-incomplete">
              {incomplete.map((i) => (
                <div key={i.key} className="pending-card sprint-incomplete-card">
                  <span className="b accent-text">{i.key}</span>
                  <span className="pending-card-summary">{toPlainText(i.summary, "summary")}</span>
                </div>
              ))}
            </div>
          </>
        )}
        {hidden > 0 && (
          <p className="muted small">
            {`${plural(hidden, "card", "cards")} on this board ${hidden === 1 ? "is" : "are"} not drawn (a status no column collects, a card the sync has not fetched, or a cell past what it renders), so the list cannot name ${hidden === 1 ? "it" : "them"} and more than these can move.`}
          </p>
        )}
        <label className="edit-row" htmlFor="complete-sprint-destination">
          <span className="muted small">Move unfinished cards to</span>
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
      </div>

      <div className="pending-actions">
        <span className="muted small">{`On Commit, unfinished cards move to ${where}.`}</span>
        <span className="pending-footer-buttons">
          <button type="button" className="btn" onClick={onClose} disabled={complete.isPending}>Cancel</button>
          <button type="button" className="btn btn-primary" disabled={complete.isPending} onClick={onSubmit}>
            {complete.isPending ? "Saving" : "Complete sprint"}
          </button>
        </span>
      </div>
    </Modal>
  );
}
