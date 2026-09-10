import type { FormEvent } from "react";
import { Modal, announce, errMsg } from "@agile-suite/core";
import { useCreateSprint, useSprintSuggestion } from "../queries/boards";
import { useSync } from "../contexts/SyncContext";
import { SprintDraftFields, useSprintDraft } from "./SprintDraftForm";

interface Props {
  profileId: string;
  boardId: number;
  onClose: () => void;
  // onCreated hands back the sprint Jira made and the sentence saying so, so
  // the board can move its picker to it and report what happened where it
  // reports everything else.
  onCreated: (sprintId: string, line: string) => void;
}

// CreateSprintModal makes a new sprint on Jira. Like the two ceremonies it
// does not wait for Commit, because a sprint's id has to be real before
// anything, including a later Start on it, can point at it; unlike them it
// takes the app's lock quietly, through runQuietLock rather than
// runSprintCeremony, since a create is not a ceremony every other view needs
// to announce.
//
// It is the dialog's first home: Task 8's Sprints view reopens this same
// component rather than building its own.
export function CreateSprintModal({ profileId, boardId, onClose, onCreated }: Props) {
  const { runQuietLock } = useSync();
  const suggestion = useSprintSuggestion(profileId, boardId, true);
  const create = useCreateSprint(profileId, runQuietLock);
  const suggested = suggestion.data;
  const draft = useSprintDraft({ idPrefix: "create-sprint", suggestion: suggested });

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (create.isPending) return;
    const values = draft.validate();
    if (!values) return;
    create.mutate(
      { boardId, name: values.name, goal: values.goal, start: values.from, end: values.to },
      {
        // The note is the write's own postscript, empty almost always: Jira
        // made the sprint and the board's sprint list could not be re-read
        // afterwards, so the picker does not offer it yet. It rides with the
        // sentence into the board's banner, since this dialog closes on
        // success.
        onSuccess: (created) => {
          const made = `${created.sprint.name || values.name} was created, ${values.from} to ${values.to}.`;
          const line = created.note ? `${made} ${created.note}` : made;
          announce(line);
          // A zero id is a documented answer rather than a failure: core/jira
          // treats an empty create response, or one with no id, as "made, go
          // and refresh", because the sprint exists in Jira either way. So
          // there is no id to select and the board keeps the sprint it had,
          // rather than switching the picker to a sprint numbered zero.
          onCreated(created.sprint.id ? String(created.sprint.id) : "", line);
          onClose();
        },
        // The dialog stays open with what the user typed still in it: the
        // failure is usually Jira's own sentence about a permission, or the
        // busy guard naming whichever operation is already running, and
        // pressing the button again is the retry.
        onError: (err) => draft.fail(errMsg(err)),
      },
    );
  }

  return (
    // Escape is closed off while the create is in flight, and this is not
    // politeness. runQuietLock holds the per-profile lock without moving the
    // reducer, so the shell's Sync and Commit buttons stay enabled and inert
    // for the length of the call; the dialog holding focus is what keeps a
    // user away from them. Unmounting mid-call would also lose the outcome,
    // since a mutate-scoped onSuccess never runs once its observer is gone,
    // and the sprint would appear in the picker unannounced and unexplained.
    <Modal
      onClose={onClose}
      className="modal pending-modal"
      labelledBy="create-sprint-title"
      closeOnOverlayClick={false}
      closeOnEsc={!create.isPending}
    >
      <div className="pending-head">
        <h2 id="create-sprint-title">New sprint</h2>
        <span className="muted">Jira makes it now. This does not wait for Commit.</span>
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>

      <form id="create-sprint-form" ref={draft.formRef} className="bulk-body edit-form" onSubmit={onSubmit}>
        <SprintDraftFields draft={draft} suggestion={suggested} suggestionError={suggestion.error} />
      </form>

      <div className="pending-actions">
        <span className="new-issue-status">
          {draft.error && <span className="error-text small" role="alert">{draft.error}</span>}
        </span>
        <span className="pending-footer-buttons">
          <button type="button" className="btn" onClick={onClose} disabled={create.isPending}>Cancel</button>
          <button type="submit" form="create-sprint-form" className="btn btn-primary" disabled={create.isPending}>
            {create.isPending ? "Creating" : "Create sprint"}
          </button>
        </span>
      </div>
    </Modal>
  );
}
