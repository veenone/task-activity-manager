import { useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import { Modal, announce, errMsg } from "@agile-suite/core";
import type { Sprint } from "../api";
import { useSprintSuggestion, useStartSprint } from "../queries/boards";
import { useSync } from "../contexts/SyncContext";
import { plural } from "../lib/format";

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
  const [name, setName] = useState(sprint.name);
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
    setFrom(suggested.start);
    setTo(suggested.end);
    // The sprint's own name is what Jira already calls it, so it wins; the
    // board's numbering only fills a gap.
    if (!sprint.name && suggested.name) setName(suggested.name);
    // A length nobody measured is a plausible wrong date nobody checks, so
    // the field the user has to look at is the one that takes focus.
    (suggested.fromHistory ? nameRef : endRef).current?.focus();
  }, [suggested, seeded, sprint.name]);

  function fail(message: string, field: string) {
    setError(message);
    setInvalidField(field);
    formRef.current?.querySelector<HTMLElement>(`#${field}`)?.focus();
  }

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (start.isPending) return;
    if (name.trim() === "") {
      fail("The sprint needs a name.", "start-sprint-name");
      return;
    }
    // Jira is never asked a question the dialog can answer: an empty date or
    // an end before a start is refused here, where the message can name the
    // field it is about.
    if (from === "") {
      fail("The start date cannot be empty.", "start-sprint-from");
      return;
    }
    if (to === "") {
      fail("The end date cannot be empty.", "start-sprint-to");
      return;
    }
    if (to < from) {
      fail("The sprint ends before it starts.", "start-sprint-to");
      return;
    }
    setError("");
    setInvalidField("");
    start.mutate(
      { boardId, sprintId: sprint.id, name: name.trim(), goal: goal.trim(), start: from, end: to },
      {
        onSuccess: () => {
          const line = `${name.trim()} is running, ${from} to ${to}.`;
          announce(line);
          onStarted(String(sprint.id), line);
          onClose();
        },
        // The dialog stays open with what the user typed still in it: the
        // failure is usually Jira's own sentence about a second active
        // sprint or a permission, and pressing the button again is the
        // retry.
        onError: (err) => {
          setError(errMsg(err));
          setInvalidField("");
        },
      },
    );
  }

  const days = suggested ? plural(suggested.length, "day", "days") : "";

  return (
    <Modal onClose={onClose} className="modal pending-modal" labelledBy="start-sprint-title" closeOnOverlayClick={false}>
      <div className="pending-head">
        <h2 id="start-sprint-title">{`Start ${sprint.name}`}</h2>
        <span className="muted">Jira starts it now. This does not wait for Commit.</span>
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>

      <form id="start-sprint-form" ref={formRef} className="bulk-body edit-form" onSubmit={onSubmit}>
        {active && (
          <p className="muted small">
            {`${active.name} is already active on this board, and Jira may refuse a second one.`}
          </p>
        )}
        <label className="edit-row" htmlFor="start-sprint-name">
          <span className="muted small">Name</span>
          <input
            id="start-sprint-name"
            ref={nameRef}
            className="detail-input"
            type="text"
            aria-invalid={invalidField === "start-sprint-name" || undefined}
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </label>
        <label className="edit-row" htmlFor="start-sprint-goal">
          <span className="muted small">Goal</span>
          <textarea
            id="start-sprint-goal"
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
              <label className="muted small" htmlFor="start-sprint-from">Start</label>
              <input
                id="start-sprint-from"
                className="detail-input detail-input-inline"
                type="date"
                aria-invalid={invalidField === "start-sprint-from" || undefined}
                value={from}
                onChange={(e) => setFrom(e.target.value)}
              />
              <label className="muted small" htmlFor="start-sprint-to">End</label>
              <input
                id="start-sprint-to"
                ref={endRef}
                className="detail-input detail-input-inline"
                type="date"
                aria-invalid={invalidField === "start-sprint-to" || undefined}
                value={to}
                onChange={(e) => setTo(e.target.value)}
              />
            </span>
            {/* Where the length came from, every time. Jira does not expose a
                board's cadence, so the only honest sources are this board's
                own closed sprints or a flat default, and a team that changed
                cadence would otherwise never know which they were reading. */}
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
          <button type="button" className="btn" onClick={onClose} disabled={start.isPending}>Cancel</button>
          <button type="submit" form="start-sprint-form" className="btn btn-primary" disabled={start.isPending}>
            {start.isPending ? "Starting" : "Start sprint"}
          </button>
        </span>
      </div>
    </Modal>
  );
}
