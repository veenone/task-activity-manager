import { useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import { Modal, announce, errMsg } from "@agile-suite/core";
import { useCreateSprint, useSprintSuggestion } from "../queries/boards";
import { useSync } from "../contexts/SyncContext";
import { plural } from "../lib/format";

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
  const [name, setName] = useState("");
  const [goal, setGoal] = useState("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [error, setError] = useState("");
  const [invalidField, setInvalidField] = useState("");
  // seeded keeps the suggestion from overwriting a date the user has already
  // corrected: the query settles after the dialog has opened, and a
  // background refetch must not undo an edit made in between.
  const [seeded, setSeeded] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const nameRef = useRef<HTMLInputElement>(null);
  const endRef = useRef<HTMLInputElement>(null);

  const suggested = suggestion.data;
  useEffect(() => {
    if (!suggested || seeded) return;
    setSeeded(true);
    setName(suggested.name);
    setFrom(suggested.start);
    setTo(suggested.end);
    // A length nobody measured is a plausible wrong date nobody checks, so
    // the field the user has to look at is the one that takes focus.
    (suggested.fromHistory ? nameRef : endRef).current?.focus();
  }, [suggested, seeded]);

  function fail(message: string, field: string) {
    setError(message);
    setInvalidField(field);
    formRef.current?.querySelector<HTMLElement>(`#${field}`)?.focus();
  }

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (create.isPending) return;
    if (name.trim() === "") {
      fail("The sprint needs a name.", "create-sprint-name");
      return;
    }
    // Jira is never asked a question the dialog can answer: a sprint with a
    // name and no dates cannot be created, since the service converts both
    // through sprintdate.Parse and an empty value fails that conversion, so
    // an empty date or an end before a start is refused here, where the
    // message can name the field it is about.
    if (from === "") {
      fail("The start date cannot be empty.", "create-sprint-from");
      return;
    }
    if (to === "") {
      fail("The end date cannot be empty.", "create-sprint-to");
      return;
    }
    if (to < from) {
      fail("The sprint ends before it starts.", "create-sprint-to");
      return;
    }
    setError("");
    setInvalidField("");
    create.mutate(
      { boardId, name: name.trim(), goal: goal.trim(), start: from, end: to },
      {
        // The note is the write's own postscript, empty almost always: Jira
        // made the sprint and the board's sprint list could not be re-read
        // afterwards, so the picker does not offer it yet. It rides with the
        // sentence into the board's banner, since this dialog closes on
        // success.
        onSuccess: (created) => {
          const made = `${created.sprint.name || name.trim()} was created, ${from} to ${to}.`;
          const line = created.note ? `${made} ${created.note}` : made;
          announce(line);
          onCreated(String(created.sprint.id), line);
          onClose();
        },
        // The dialog stays open with what the user typed still in it: the
        // failure is usually Jira's own sentence about a permission, or the
        // busy guard naming whichever operation is already running, and
        // pressing the button again is the retry.
        onError: (err) => {
          setError(errMsg(err));
          setInvalidField("");
        },
      },
    );
  }

  const days = suggested ? plural(suggested.length, "day", "days") : "";

  return (
    <Modal onClose={onClose} className="modal pending-modal" labelledBy="create-sprint-title" closeOnOverlayClick={false}>
      <div className="pending-head">
        <h2 id="create-sprint-title">New sprint</h2>
        <span className="muted">Jira makes it now. This does not wait for Commit.</span>
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>

      <form id="create-sprint-form" ref={formRef} className="bulk-body edit-form" onSubmit={onSubmit}>
        <label className="edit-row" htmlFor="create-sprint-name">
          <span className="muted small">Name</span>
          <input
            id="create-sprint-name"
            ref={nameRef}
            className="detail-input"
            type="text"
            aria-invalid={invalidField === "create-sprint-name" || undefined}
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </label>
        <label className="edit-row" htmlFor="create-sprint-goal">
          <span className="muted small">Goal</span>
          <textarea
            id="create-sprint-goal"
            className="detail-input"
            rows={3}
            value={goal}
            onChange={(e) => setGoal(e.target.value)}
          />
        </label>
        <div className="edit-row">
          <span className="muted small">Dates</span>
          <span className="edit-cell">
            <span className="date-field">
              <label className="muted small" htmlFor="create-sprint-from">Start</label>
              <input
                id="create-sprint-from"
                className="detail-input detail-input-inline"
                type="date"
                aria-invalid={invalidField === "create-sprint-from" || undefined}
                value={from}
                onChange={(e) => setFrom(e.target.value)}
              />
              <label className="muted small" htmlFor="create-sprint-to">End</label>
              <input
                id="create-sprint-to"
                ref={endRef}
                className="detail-input detail-input-inline"
                type="date"
                aria-invalid={invalidField === "create-sprint-to" || undefined}
                value={to}
                onChange={(e) => setTo(e.target.value)}
              />
            </span>
            {suggestion.isError ? (
              <span className="muted small">The suggested dates could not be read ({suggestion.error.message}). Fill them in.</span>
            ) : suggested ? (
              <span className="muted small">
                {suggested.fromHistory
                  ? `Suggested ${days} from this board's last sprints. Change either date.`
                  : `No closed sprint on this board to measure, so this is the ${days} default. Check the end date.`}
              </span>
            ) : (
              <span className="muted small">Reading this board's sprint length.</span>
            )}
          </span>
        </div>
      </form>

      <div className="pending-actions">
        <span className="new-issue-status">
          {error && <span className="error-text small" role="alert">{error}</span>}
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
