// One test's step results inside an execution, when the backend addresses
// step results without a dataset (#371 Phase 6).
//
// This is the flat sibling of ParameterizedRunRows. There is no iteration and
// there are no parameters: a step result belongs straight to the execution, so
// the panel is one table rather than a tree.
//
// Two things it deliberately does differently from the iteration rows:
//
// - The results offered come from the backend's own vocabulary rather than
//   Xray's six. A backend with addressable step results configures its
//   statuses per instance, so a hardcoded list would offer names it has no id
//   for (#371 Phase 5).
// - The status cell sets no status-derived class. Those classes exist for
//   Xray's six names, and colouring by a name this instance invented would
//   silently render unstyled.
//
// It reuses the parameterized stylesheet's classes, which are step-table
// styling rather than anything about datasets. Renaming them would touch
// twenty call sites for no change on screen.

import { useEffect, useRef, useState } from "react";
import { useQueryClient, type QueryClient } from "@tanstack/react-query";

import { SetRunStepStatus, errMsg, type RunStepResult } from "../../api";
import { keys } from "../../queries/keys";
import { useRunStepResults } from "../../queries/runSteps";
import { ResultBulkBar } from "./ResultBulkBar";
import "./parameterized-runs.css";

interface Props {
  profileId: string;
  execKey: string;
  testKey: string;
  expanded: boolean;
  runStatuses: string[];
  onLoaded?: () => void;
}

interface IndicatorProps {
  profileId: string;
  execKey: string;
  testKey: string;
  expanded: boolean;
  onLoaded: (count: number) => void;
  onToggle: () => void;
}

// A test with no steps answers with an empty list rather than an error, and a
// backend that cannot address step results answers ErrUnsupported. Every
// member row of every execution asks, so neither is worth reporting as a
// failure: the row simply has no steps. Anything else is a real failure, and
// rendering it as nothing is how #362 stayed invisible for two releases.
function hasNoStepResults(error: unknown): boolean {
  return errMsg(error).includes("operation not supported");
}

// RunStepIndicator resolves the step results while the member row is visible,
// so a test advertises how many steps it has before expansion.
export function RunStepIndicator({ profileId, execKey, testKey, expanded, onLoaded, onToggle }: IndicatorProps) {
  const query = useRunStepResults(profileId, execKey, testKey, true);
  const announced = useRef(false);
  const steps = query.data;

  useEffect(() => {
    if (!steps || steps.length === 0 || announced.current) return;
    announced.current = true;
    onLoaded(steps.length);
  }, [steps, onLoaded]);

  if (query.error && !hasNoStepResults(query.error)) {
    return (
      <span className="parameterized-run-indicator-error" title={errMsg(query.error)}>
        <span aria-hidden="true">!</span>
        <span>steps unavailable</span>
      </span>
    );
  }
  if (!steps || steps.length === 0) return null;
  return (
    <button
      type="button"
      className="parameterized-run-toggle"
      aria-expanded={expanded}
      onClick={onToggle}
      title={`${steps.length} step result${steps.length === 1 ? "" : "s"}`}
    >
      <span aria-hidden="true">{expanded ? "▾" : "▸"}</span> {steps.length} steps
    </button>
  );
}

export function RunStepRows({ profileId, execKey, testKey, expanded, runStatuses, onLoaded }: Props) {
  const client = useQueryClient();
  const query = useRunStepResults(profileId, execKey, testKey, expanded);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [writeError, setWriteError] = useState("");
  const steps = query.data;
  const announced = useRef(false);

  useEffect(() => {
    if (!steps || steps.length === 0 || announced.current) return;
    announced.current = true;
    onLoaded?.();
  }, [steps, onLoaded]);

  // The selection is per execution and test, so moving to another row drops
  // it rather than applying somebody else's picks.
  useEffect(() => {
    setSelected(new Set());
  }, [profileId, execKey, testKey]);

  if (!expanded) return null;
  if (query.isFetching && !steps) return <p className="parameterized-run-message">Loading step results...</p>;
  if (query.error && !hasNoStepResults(query.error)) {
    return <p className="parameterized-run-error">{errMsg(query.error)}</p>;
  }
  if (!steps || steps.length === 0) {
    return <p className="parameterized-run-message">This test has no recorded step results in this execution.</p>;
  }

  const rows = steps;
  const picked = rows.filter((step) => selected.has(step.id));

  // Each apply reports rather than throws, so a bulk apply keeps going and
  // names what did not take instead of stopping at the first refusal (#365).
  // The patch marks the result uncommitted because that is what it now is: it
  // is journalled and waits for Commit (#366), and nothing refetches after a
  // pick to discover that for us.
  async function applyResult(stepResultId: string, status: string): Promise<string> {
    try {
      await SetRunStepStatus(profileId, execKey, testKey, stepResultId, status);
      patchSteps(client, profileId, execKey, testKey, (current) =>
        current.map((step) => (step.id === stepResultId ? { ...step, status, uncommitted: true } : step)),
      );
      return "";
    } catch (error) {
      return errMsg(error);
    }
  }

  async function setStatus(stepResultId: string, status: string) {
    setWriteError(await applyResult(stepResultId, status));
  }

  // Applied one at a time: these are local writes, and a sequential pass keeps
  // the journal rows in the order the rows appear on screen.
  async function applyToSelected(status: string) {
    const failures: string[] = [];
    for (const [index, step] of rows.entries()) {
      if (!selected.has(step.id)) continue;
      const failure = await applyResult(step.id, status);
      if (failure) failures.push(`step ${index + 1} (${failure})`);
    }
    setWriteError(failures.length === 0 ? "" : `Some results did not apply: ${failures.join(", ")}.`);
    setSelected(new Set());
  }

  function toggle(stepResultId: string) {
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(stepResultId)) next.delete(stepResultId);
      else next.add(stepResultId);
      return next;
    });
  }

  function toggleEvery() {
    setSelected((current) => (current.size === rows.length ? new Set() : new Set(rows.map((step) => step.id))));
  }

  return (
    <section className="parameterized-run" aria-label={`Step results for ${testKey}`}>
      {writeError && (
        <p className="parameterized-run-error" role="alert">
          {writeError}
        </p>
      )}
      <ResultBulkBar
        label={`Selected steps of ${testKey}`}
        count={picked.length}
        statuses={runStatuses}
        onApply={applyToSelected}
        onClear={() => setSelected(new Set())}
      />
      <div className="parameterized-run-scroll">
      <table className="parameterized-step-table" aria-label={`Step results for ${testKey}`}>
        <thead>
          <tr>
            <th scope="col" className="parameterized-pick-col">
              <input
                type="checkbox"
                aria-label={`Select every step of ${testKey}`}
                checked={picked.length === rows.length}
                onChange={toggleEvery}
              />
            </th>
            <th scope="col">Step</th>
            <th scope="col">Action</th>
            <th scope="col">Data</th>
            <th scope="col">Expected</th>
            <th scope="col">Result</th>
            <th scope="col">Comment</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((step, index) => (
            <tr key={step.id}>
              <td className="parameterized-pick-col">
                <input
                  type="checkbox"
                  aria-label={`Select step ${index + 1} of ${testKey}`}
                  checked={selected.has(step.id)}
                  onChange={() => toggle(step.id)}
                />
              </td>
              <th scope="row">{index + 1}</th>
              <td>{step.action || "-"}</td>
              <td>{step.data || "-"}</td>
              <td>{step.expected || "-"}</td>
              <td>
                <select
                  className="run-select"
                  aria-label={`Step ${index + 1} result`}
                  value={step.status}
                  onChange={(event) => setStatus(step.id, event.target.value)}
                >
                  {optionsFor(step.status, runStatuses).map((status) => (
                    <option key={status}>{status}</option>
                  ))}
                </select>
                {step.uncommitted && (
                  <span className="parameterized-run-uncommitted" title="Set locally. Commit to send it to Jira.">
                    not committed
                  </span>
                )}
              </td>
              <td>{step.comment || "-"}</td>
            </tr>
          ))}
        </tbody>
      </table>
      </div>
    </section>
  );
}

// optionsFor keeps the value the backend reported selectable even when it is
// not in the offered list. A status the instance has since renamed or removed
// would otherwise make the select show a different step's result as this
// step's, which reads as a result nobody recorded.
function optionsFor(current: string, offered: string[]): string[] {
  return offered.includes(current) ? offered : [current, ...offered];
}

function patchSteps(
  client: QueryClient,
  profileId: string,
  execKey: string,
  testKey: string,
  patch: (current: RunStepResult[]) => RunStepResult[],
) {
  client.setQueryData<RunStepResult[]>(keys.runStepResults(profileId, execKey, testKey), (current) =>
    current ? patch(current) : current,
  );
}
