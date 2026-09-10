import { useEffect, useRef, useState } from "react";
import type { RefObject } from "react";
import { plural } from "../lib/format";
import type { SprintSuggestion } from "../api";

// A sprint's four editable fields, and the rules about them, in one place.
//
// Three dialogs collect exactly these: starting a sprint Jira already has,
// creating one, and editing one. They differ in their title, their button,
// the call they make and the sentence they report, and in nothing else. The
// fields, the validation, the suggestion seeding and the caption explaining
// where a suggested length came from were copied once already; this is what
// they were copied from.
//
// The dialog keeps its own submit and its own mutation. This owns only what
// all three agree on, which is why validate returns the trimmed values rather
// than calling anything itself.

export interface SprintDraftValues {
  name: string;
  goal: string;
  from: string;
  to: string;
}

interface Options {
  // idPrefix namespaces every field id, because two dialogs can be mounted in
  // the same document during a transition and a duplicate id would make a
  // label point at the wrong input.
  idPrefix: string;
  // initialName is the name a sprint already has. Empty when creating, since
  // there is no sprint yet and the board's numbering fills the gap instead.
  initialName?: string;
  initialGoal?: string;
  suggestion: SprintSuggestion | undefined;
}

export interface SprintDraft {
  values: SprintDraftValues;
  set: {
    name: (v: string) => void;
    goal: (v: string) => void;
    from: (v: string) => void;
    to: (v: string) => void;
  };
  error: string;
  invalidField: string;
  formRef: RefObject<HTMLFormElement | null>;
  nameRef: RefObject<HTMLInputElement | null>;
  endRef: RefObject<HTMLInputElement | null>;
  idPrefix: string;
  // validate answers with the trimmed values when the dialog can answer every
  // question itself, and null when it has already reported which field is
  // wrong. Jira is never asked something the form knows the answer to.
  validate: () => SprintDraftValues | null;
  // fail reports a message no field owns, which is what a refusal off the
  // wire is: Jira's sentence about a permission or a second active sprint,
  // with nothing on the form to point at.
  fail: (message: string) => void;
  clearError: () => void;
}

export function useSprintDraft({ idPrefix, initialName = "", initialGoal = "", suggestion }: Options): SprintDraft {
  const [name, setName] = useState(initialName);
  const [goal, setGoal] = useState(initialGoal);
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

  useEffect(() => {
    if (!suggestion || seeded) return;
    setSeeded(true);
    setFrom(suggestion.start);
    setTo(suggestion.end);
    // A name the sprint already has is what Jira calls it, so it wins; the
    // board's numbering only fills a gap.
    if (!initialName && suggestion.name) setName(suggestion.name);
    // A length nobody measured is a plausible wrong date nobody checks, so
    // the field the user has to look at is the one that takes focus.
    (suggestion.fromHistory ? nameRef : endRef).current?.focus();
  }, [suggestion, seeded, initialName]);

  function fail(message: string, field: string): null {
    setError(message);
    setInvalidField(field);
    formRef.current?.querySelector<HTMLElement>(`#${field}`)?.focus();
    return null;
  }

  function validate(): SprintDraftValues | null {
    if (name.trim() === "") return fail("The sprint needs a name.", `${idPrefix}-name`);
    // Both dates are required rather than optional, and that is TAM's rule
    // rather than Jira's: the service converts them through sprintdate, which
    // refuses an empty value, so a dateless sprint would fail at a round trip
    // instead of here where the message can name the field.
    if (from === "") return fail("The start date cannot be empty.", `${idPrefix}-from`);
    if (to === "") return fail("The end date cannot be empty.", `${idPrefix}-to`);
    if (to < from) return fail("The sprint ends before it starts.", `${idPrefix}-to`);
    setError("");
    setInvalidField("");
    return { name: name.trim(), goal: goal.trim(), from, to };
  }

  return {
    values: { name, goal, from, to },
    set: { name: setName, goal: setGoal, from: setFrom, to: setTo },
    error,
    invalidField,
    formRef,
    nameRef,
    endRef,
    idPrefix,
    validate,
    fail: (message: string) => {
      setError(message);
      setInvalidField("");
    },
    clearError: () => {
      setError("");
      setInvalidField("");
    },
  };
}

interface FieldsProps {
  draft: SprintDraft;
  suggestion: SprintSuggestion | undefined;
  suggestionError: Error | null;
}

// SprintDraftFields renders the four fields and the caption under the dates.
// It takes the suggestion's error separately from the suggestion itself,
// because a dialog whose dates could not be suggested is still usable and has
// to say so rather than silently offering two empty boxes.
export function SprintDraftFields({ draft, suggestion, suggestionError }: FieldsProps) {
  const { values, set, invalidField, idPrefix } = draft;
  const days = suggestion ? plural(suggestion.length, "day", "days") : "";

  return (
    <>
      <label className="edit-row" htmlFor={`${idPrefix}-name`}>
        <span className="muted small">Name</span>
        <input
          id={`${idPrefix}-name`}
          ref={draft.nameRef}
          className="detail-input"
          type="text"
          aria-invalid={invalidField === `${idPrefix}-name` || undefined}
          value={values.name}
          onChange={(e) => set.name(e.target.value)}
        />
      </label>
      <label className="edit-row" htmlFor={`${idPrefix}-goal`}>
        <span className="muted small">Goal</span>
        <textarea
          id={`${idPrefix}-goal`}
          className="detail-input"
          rows={3}
          value={values.goal}
          onChange={(e) => set.goal(e.target.value)}
        />
      </label>
      <div className="edit-row">
        <span className="muted small">Dates</span>
        <span className="edit-cell">
          <span className="date-field">
            <label className="muted small" htmlFor={`${idPrefix}-from`}>Start</label>
            <input
              id={`${idPrefix}-from`}
              className="detail-input detail-input-inline"
              type="date"
              aria-invalid={invalidField === `${idPrefix}-from` || undefined}
              value={values.from}
              onChange={(e) => set.from(e.target.value)}
            />
            <label className="muted small" htmlFor={`${idPrefix}-to`}>End</label>
            <input
              id={`${idPrefix}-to`}
              ref={draft.endRef}
              className="detail-input detail-input-inline"
              type="date"
              aria-invalid={invalidField === `${idPrefix}-to` || undefined}
              value={values.to}
              onChange={(e) => set.to(e.target.value)}
            />
          </span>
          {/* Where the length came from, every time. Jira does not expose a
              board's cadence, so the only honest sources are this board's own
              closed sprints or a flat default, and a team that changed cadence
              would otherwise never know which they were reading. */}
          {suggestionError ? (
            <span className="muted small">The suggested dates could not be read ({suggestionError.message}). Fill them in.</span>
          ) : suggestion ? (
            <span className="muted small">
              {suggestion.fromHistory
                ? `Suggested ${days} from this board's last sprints. Change either date.`
                : `No closed sprint on this board to measure, so this is the ${days} default. Check the end date.`}
            </span>
          ) : (
            <span className="muted small">Reading this board's sprint length.</span>
          )}
        </span>
      </div>
    </>
  );
}
