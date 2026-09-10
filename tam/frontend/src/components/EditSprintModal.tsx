import { useEffect } from "react";
import type { FormEvent } from "react";
import { Modal, announce, errMsg, useConfirm } from "@agile-suite/core";
import type { Sprint } from "../api";
import { useEditSprint } from "../queries/sprints";
import { useSync } from "../contexts/SyncContext";
import { dayInput } from "../lib/format";
import { ImmediateWriteChip } from "./ImmediateWriteChip";
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
// a sprint and creating one, and like both of those it reaches Jira the
// moment it is submitted rather than waiting for Commit.
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
  const originalEnd = dayInput(sprint.endDate);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (edit.isPending) return;
    const values = draft.validate();
    if (!values) return;
    // Moving a running sprint's end date is the one edit here with a
    // consequence outside this window: Jira takes the new date at once, and
    // every burndown and every board reading that sprint moves with it. It
    // is a question rather than a refusal, since moving the date is a real
    // and ordinary thing for a team to decide.
    if (sprint.state === "active" && values.to !== originalEnd) {
      const ok = await confirm({
        title: "Move a running sprint's end date?",
        message: `${sprint.name} is running. Jira takes the new end date immediately, so everyone reading this board sees the sprint end on ${values.to} instead of ${originalEnd}.`,
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
        // The note is the write's own postscript, empty almost always: Jira
        // took the edit and the board's sprint list could not be re-read
        // afterwards, so the list on screen still shows the old name. It
        // rides with the sentence into the view's banner, since this dialog
        // closes on success.
        onSuccess: (note) => {
          const changed = `${values.name} was updated.`;
          const line = note ? `${changed} ${note}` : changed;
          announce(line);
          onEdited(line);
          onClose();
        },
        // The dialog stays open with what the user typed still in it. The
        // refusal is usually Jira's own sentence about a permission or a
        // closed sprint, or the busy guard naming whichever operation is
        // already running: that guard is Go's, and it refuses whether or not
        // anything on screen is showing a banner, so this is the only place
        // the user can read why nothing happened.
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
      className="modal pending-modal"
      labelledBy="edit-sprint-title"
      closeOnOverlayClick={false}
      closeOnEsc={!edit.isPending}
    >
      <div className="pending-head">
        <h2 id="edit-sprint-title">{`Edit ${sprint.name}`}</h2>
        <ImmediateWriteChip />
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>

      <form id="edit-sprint-form" ref={draft.formRef} className="bulk-body edit-form" onSubmit={onSubmit}>
        <SprintDraftFields
          draft={draft}
          suggestion={undefined}
          suggestionError={null}
          suggesting={false}
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
