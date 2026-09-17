import type { FormEvent } from "react";
import { Modal, announce, errMsg } from "@agile-suite/core";
import type { Sprint } from "../api";
import { useSprintSuggestion, useStartSprint } from "../queries/boards";
import { useSync } from "../contexts/SyncContext";
import { SprintDraftFields, useSprintDraft } from "./SprintDraftForm";

interface Props {
  profileId: string;
  boardId: number;
  // sprint is the future sprint being started, or a draft Commit has not
  // created yet; this dialog fills in what starting one needs.
  sprint: Sprint;
  // active is the sprint already running on this board, when there is one.
  // Naming it is reading the picker's own data, which is a fact TAM holds;
  // whether Jira will accept a second active sprint is Jira's answer to
  // give, and this dialog does not guess at it.
  active: Sprint | undefined;
  onClose: () => void;
  // onStarted hands back the sprint the start was saved for and the sentence
  // saying so, so the board can keep its picker on it and report what
  // happened where the board reports everything else.
  onStarted: (sprintId: string, line: string) => void;
}

// StartSprintModal saves a start of one sprint. Like every other write on
// the board it goes through the journal: nothing reaches Jira until Commit,
// which starts the sprint with these values.
export function StartSprintModal({ profileId, boardId, sprint, active, onClose, onStarted }: Props) {
  const { runQuietLock } = useSync();
  const suggestion = useSprintSuggestion(profileId, boardId, true);
  const start = useStartSprint(profileId, runQuietLock);
  const suggested = suggestion.data;
  // The goal comes from the sprint. Starting one sends the goal box to Jira
  // whatever is in it, so a dialog that opened empty over a real goal was a
  // blank field the user typed into without ever seeing what they were
  // replacing. It is empty for a sprint cached before the goal column
  // existed, which a boards refresh fills in.
  const draft = useSprintDraft({
    idPrefix: "start-sprint",
    initialName: sprint.name,
    initialGoal: sprint.goal,
    suggestion: suggested,
  });
  function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (start.isPending) return;
    const values = draft.validate();
    if (!values) return;
    start.mutate(
      { boardId, sprintId: sprint.id, name: values.name, goal: values.goal, start: values.from, end: values.to },
      {
        // The sentence rides into the board's banner, since this dialog
        // closes on success.
        onSuccess: () => {
          const line = `${values.name} will start on Commit, ${values.from} to ${values.to}.`;
          announce(line);
          onStarted(String(sprint.id), line);
          onClose();
        },
        // The dialog stays open with what the user typed still in it: the
        // refusal is a sprint that is not future, a delete waiting for
        // Commit, or the busy guard naming whichever operation is running.
        onError: (err) => draft.fail(errMsg(err)),
      },
    );
  }

  return (
    <Modal onClose={onClose} className="modal pending-modal" labelledBy="start-sprint-title" closeOnOverlayClick={false}>
      <div className="pending-head">
        <div className="edit-sprint-title">
          <h2 id="start-sprint-title">{`Start ${sprint.name}`}</h2>
          <p>{sprint.draft ? "A draft sprint. Commit creates it in Jira, then starts it." : "Saved locally. Commit starts the sprint in Jira."}</p>
        </div>
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>

      <form id="start-sprint-form" ref={draft.formRef} className="bulk-body edit-form" onSubmit={onSubmit}>
        {active && (
          <p className="muted small">
            {`${active.name} is already active on this board, and Jira may refuse a second one when Commit sends the start.`}
          </p>
        )}
        <SprintDraftFields draft={draft} suggestion={suggested} suggestionError={suggestion.error} />
      </form>

      <div className="pending-actions">
        <span className="new-issue-status">
          {draft.error && <span className="error-text small" role="alert">{draft.error}</span>}
        </span>
        <span className="pending-footer-buttons">
          <button type="button" className="btn" onClick={onClose} disabled={start.isPending}>Cancel</button>
          <button type="submit" form="start-sprint-form" className="btn btn-primary" disabled={start.isPending}>
            {start.isPending ? "Saving" : "Start sprint"}
          </button>
        </span>
      </div>
    </Modal>
  );
}
