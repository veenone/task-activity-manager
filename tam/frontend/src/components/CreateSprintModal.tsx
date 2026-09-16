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

// CreateSprintModal drafts a new sprint. Like a new issue it waits for
// Commit: the sprint is journaled under a negative id, every picker offers
// it at once, and Commit creates it in Jira before any card moved into it is
// sent. It still takes the app's lock quietly, through runQuietLock, because
// the Go binding serialises a draft against a Commit rewriting the same rows.
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
        // The create is a local draft now, so the Go side always answers
        // with an empty note and the draft's negative id. The note still
        // rides into the board's banner when present, since this dialog
        // closes on success, and the id is what the picker switches to.
        onSuccess: (created) => {
          const made = `${created.sprint.name || values.name} was drafted, ${values.from} to ${values.to}. Commit creates it in Jira.`;
          const line = created.note ? `${made} ${created.note}` : made;
          announce(line);
          // A zero id cannot come from a draft; the guard only keeps the
          // picker off a sprint numbered zero if one ever did.
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
        <p className="muted small">Drafted locally. Commit creates it in Jira.</p>
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
