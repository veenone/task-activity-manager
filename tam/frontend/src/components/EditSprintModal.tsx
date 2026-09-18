import { useEffect } from "react";
import type { FormEvent } from "react";
import { Modal, announce, errMsg, useConfirm } from "@agile-suite/core";
import type { Sprint } from "../api";
import { useEditSprint } from "../queries/sprints";
import { useSync } from "../contexts/SyncContext";
import { dayInput } from "../lib/format";
import { SprintDraftFields, useSprintDraft } from "./SprintDraftForm";

interface Props {
  profileId: string;
  boardId: number;
  // sprint is the one being edited, with the name, goal and dates the dialog
  // opens on. Those are the cache's copy: Jira is not re-read first, so an
  // edit made on the web an hour ago is overwritten by whatever is on screen
  // here, the same way every other write in TAM works.
  sprint: Sprint;
  // otherNames are the names of this board's other sprints, for the nudge. A
  // sprint being renamed to what it is already called is not a duplicate, so
  // its own name is not in this list.
  otherNames: string[];
  onClose: () => void;
  // onEdited hands back the sentence saying what changed, so the view reports
  // it where it reports every other sprint write.
  onEdited: (line: string) => void;
}

// EditSprintModal renames a sprint, rewrites its goal, or moves its dates.
// It is the third dialog over SprintDraftForm's four fields, after starting
// a sprint and creating one. Like creating one, it saves locally and Commit
// sends the change to Jira.
export function EditSprintModal({ profileId, boardId, sprint, otherNames, onClose, onEdited }: Props) {
  const { runQuietLock } = useSync();
  const { confirm } = useConfirm();
  const edit = useEditSprint(profileId, runQuietLock);
  // No suggestion is read here, and that is the difference from the other
  // two dialogs: a suggestion is a guess at a sprint that has no dates, and
  // this one has them.
  const draft = useSprintDraft({
    idPrefix: "edit-sprint",
    initialName: sprint.name,
    initialGoal: sprint.goal,
    initialFrom: dayInput(sprint.startDate),
    initialTo: dayInput(sprint.endDate),
    suggestion: undefined,
  });
  // Nothing else claims focus here, since no suggestion arrives to take it.
  useEffect(() => {
    draft.nameRef.current?.focus();
  }, []);

  const duplicate = otherNames.some(
    (n) => n.trim().toLowerCase() === draft.values.name.trim().toLowerCase() && draft.values.name.trim() !== "",
  );
  const originalStart = dayInput(sprint.startDate);
  const originalEnd = dayInput(sprint.endDate);
  const dateChanged = draft.values.from !== originalStart || draft.values.to !== originalEnd;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (edit.isPending) return;
    const values = draft.validate();
    if (!values) return;
    if (duplicate) {
      const ok = await confirm({
        title: "Use a duplicate sprint name?",
        message: `Another sprint on this board is already called ${values.name}. Jira allows duplicate names, but they can make reports and picker choices harder to tell apart.`,
        confirmLabel: "Use this name",
        cancelLabel: "Keep editing",
        danger: false,
      });
      if (!ok) return;
    }
    // Moving a running sprint's end date is the one edit here with a
    // consequence outside this window: once Commit sends it, every burndown
    // and every board reading that sprint moves with it. It is a question
    // rather than a refusal, since moving the date is a real and ordinary
    // thing for a team to decide.
    if (sprint.state === "active" && values.to !== originalEnd) {
      const ok = await confirm({
        title: "Move a running sprint's end date?",
        message: `${sprint.name} is running. Once Commit sends it, everyone reading this board sees the sprint end on ${values.to} instead of ${originalEnd}.`,
        confirmLabel: "Move the end date",
        cancelLabel: "Leave it",
        // Not a destructive confirm: nothing is thrown away, and the red
        // button belongs to the actions that throw something away.
        danger: false,
      });
      if (!ok) return;
    }
    edit.mutate(
      {
        boardId,
        sprintId: sprint.id,
        name: values.name,
        goal: values.goal,
        start: values.from,
        end: values.to,
        // An empty goal box means two different things, and only the dialog
        // knows which: nothing was ever there, or what was there is being
        // taken away. The sprint's own goal is what tells them apart.
        clearGoal: values.goal === "" && sprint.goal !== "",
      },
      {
        // The sentence rides into the view's banner, since this dialog closes
        // on success.
        onSuccess: () => {
          const line = `${values.name} was updated. Commit sends the change to Jira.`;
          announce(line);
          onEdited(line);
          onClose();
        },
        // The dialog stays open with what the user typed still in it. The
        // refusal is a closed sprint, a sprint waiting to be deleted, or the
        // busy guard naming whichever operation is already running: that
        // guard is Go's, and it refuses whether or not anything on screen is
        // showing a banner, so this is the only place the user can read why
        // nothing happened.
        onError: (err) => draft.fail(errMsg(err)),
      },
    );
  }

  return (
    // Escape is closed off while the edit is in flight for the same reason
    // the create dialog closes it off: the quiet lock leaves the shell's own
    // buttons enabled and inert for the length of the call, and this dialog
    // holding focus is what keeps a user away from them. Unmounting mid-call
    // would also lose the outcome, since a mutate-scoped onSuccess never runs
    // once its observer is gone.
    <Modal
      onClose={onClose}
      className="modal pending-modal edit-sprint-modal"
      labelledBy="edit-sprint-title"
      closeOnOverlayClick={false}
      closeOnEsc={!edit.isPending}
    >
      <div className="pending-head">
        <div className="edit-sprint-title">
          <h2 id="edit-sprint-title">{`Edit ${sprint.name}`}</h2>
          <p>{sprint.draft ? "A draft sprint. Changes stay local until Commit." : "Changes are saved locally and sent to Jira on Commit."}</p>
        </div>
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>

      <form id="edit-sprint-form" ref={draft.formRef} className="bulk-body edit-form" onSubmit={onSubmit}>
        <SprintDraftFields
          draft={draft}
          suggestion={undefined}
          suggestionError={null}
          suggesting={false}
          dateChanged={dateChanged}
          nameNote={duplicate ? "Another sprint on this board is already called that." : undefined}
        />
      </form>

      <div className="pending-actions">
        <span className="new-issue-status">
          {draft.error && <span className="error-text small" role="alert">{draft.error}</span>}
        </span>
        <span className="pending-footer-buttons">
          <button type="button" className="btn" onClick={onClose} disabled={edit.isPending}>Cancel</button>
          <button type="submit" form="edit-sprint-form" className="btn btn-primary" disabled={edit.isPending}>
            {edit.isPending ? "Saving" : "Save changes"}
          </button>
        </span>
      </div>
    </Modal>
  );
}
