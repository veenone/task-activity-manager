import type { FormEvent } from "react";
import { Modal, announce, errMsg } from "@agile-suite/core";
import type { Sprint } from "../api";
import { useSprintSuggestion, useStartSprint } from "../queries/boards";
import { useSync } from "../contexts/SyncContext";
import { SprintDraftFields, useSprintDraft } from "./SprintDraftForm";

interface Props {
  profileId: string;
  boardId: number;
  // sprint is the future sprint being started. It already exists in Jira,
  // with a name Jira gave it; this dialog fills in what starting one needs.
  sprint: Sprint;
  // active is the sprint already running on this board, when there is one.
  // Naming it is reading the picker's own data, which is a fact TAM holds;
  // whether Jira will accept a second active sprint is Jira's answer to
  // give, and this dialog does not guess at it.
  active: Sprint | undefined;
  onClose: () => void;
  // onStarted hands back the sprint that was started and the sentence saying
  // so, so the board can move its picker to it rather than falling back to
  // whatever is first, and report what happened where the board reports
  // everything else.
  onStarted: (sprintId: string, line: string) => void;
}

// StartSprintModal starts one sprint on Jira. Unlike every other write on
// the board it does not go through the journal: a sprint's start is a
// timestamped fact a whole team reads, so it happens now or not at all, and
// this dialog is where "not at all" is reported.
export function StartSprintModal({ profileId, boardId, sprint, active, onClose, onStarted }: Props) {
  const { runSprintCeremony } = useSync();
  const suggestion = useSprintSuggestion(profileId, boardId, true);
  const start = useStartSprint(profileId, runSprintCeremony);
  const suggested = suggestion.data;
  const draft = useSprintDraft({ idPrefix: "start-sprint", initialName: sprint.name, suggestion: suggested });
  function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (start.isPending) return;
    const values = draft.validate();
    if (!values) return;
    start.mutate(
      { boardId, sprintId: sprint.id, name: values.name, goal: values.goal, start: values.from, end: values.to },
      {
        // The note is the ceremony's own postscript, empty almost always:
        // Jira started the sprint and the board's sprint list could not be
        // re-read afterwards, so the picker still calls it future and the
        // toolbar still offers Start. It rides with the sentence into the
        // board's banner, since this dialog closes on success.
        onSuccess: (note) => {
          const started = `${values.name} is running, ${values.from} to ${values.to}.`;
          const line = note ? `${started} ${note}` : started;
          announce(line);
          onStarted(String(sprint.id), line);
          onClose();
        },
        // The dialog stays open with what the user typed still in it: the
        // failure is usually Jira's own sentence about a second active
        // sprint or a permission, and pressing the button again is the
        // retry.
        onError: (err) => draft.fail(errMsg(err)),
      },
    );
  }

  return (
    <Modal onClose={onClose} className="modal pending-modal" labelledBy="start-sprint-title" closeOnOverlayClick={false}>
      <div className="pending-head">
        <h2 id="start-sprint-title">{`Start ${sprint.name}`}</h2>
        <span className="muted">Jira starts it now. This does not wait for Commit.</span>
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>

      <form id="start-sprint-form" ref={draft.formRef} className="bulk-body edit-form" onSubmit={onSubmit}>
        {active && (
          <p className="muted small">
            {`${active.name} is already active on this board, and Jira may refuse a second one.`}
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
            {start.isPending ? "Starting" : "Start sprint"}
          </button>
        </span>
      </div>
    </Modal>
  );
}
