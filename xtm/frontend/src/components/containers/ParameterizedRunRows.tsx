import { useEffect, useRef, useState } from "react";
import { useQueryClient, type QueryClient } from "@tanstack/react-query";

import { SetIterationStatus, SetIterationStepStatus, errMsg, type ParameterizedRun } from "../../api";
import { keys } from "../../queries/keys";
import { useParameterizedRun } from "../../queries/parameterizedRuns";
import { IterationRows, stepKey } from "./IterationRows";
import { ResultBulkBar } from "./ResultBulkBar";
import "./parameterized-runs.css";

interface Props {
  profileId: string;
  execKey: string;
  testKey: string;
  expanded: boolean;
  canWrite: boolean;
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

// A test with no dataset answers with backend.ErrUnsupported, and every member
// row of every execution asks, so that answer is not a failure to report: the
// row just has no iterations. Anything else is a failure, and used to render as
// nothing at all, which is how #362 stayed invisible for two releases. The text
// is the sentinel's own, which is the only signal the binding carries across.
function isNotParameterized(error: unknown): boolean {
  return errMsg(error).includes("operation not supported");
}

// ParameterizedRunIndicator resolves the run while the member row is visible,
// so parameterized tests advertise their iteration count before expansion.
export function ParameterizedRunIndicator({ profileId, execKey, testKey, expanded, onLoaded, onToggle }: IndicatorProps) {
  const query = useParameterizedRun(profileId, execKey, testKey, true);
  const announcedRun = useRef("");
  const run = query.data;

  useEffect(() => {
    if (!run || run.iterations.length === 0 || announcedRun.current === run.id) return;
    announcedRun.current = run.id;
    onLoaded(run.iterations.length);
  }, [run, onLoaded]);

  // A read that failed says so in the row. It is not a toggle: there is no run
  // to expand, and the detail rows below only mount for a row that reported its
  // iterations, so a button here would open an empty panel.
  if (query.error && !isNotParameterized(query.error)) {
    return (
      <span className="parameterized-run-indicator-error" title={errMsg(query.error)}>
        <span aria-hidden="true">!</span>
        <span>iterations unavailable</span>
      </span>
    );
  }
  if (!run || run.iterations.length === 0) return null;
  const count = run.iterations.length;
  return (
    <button
      type="button"
      className="parameterized-run-toggle parameterized-run-indicator"
      aria-expanded={expanded}
      aria-label={`${expanded ? "Hide" : "Show"} ${count} dataset iterations for ${testKey}`}
      onClick={onToggle}
    >
      <span aria-hidden="true">{expanded ? "▾" : "▸"}</span>
      <span>
        {count} {count === 1 ? "iteration" : "iterations"}
      </span>
    </button>
  );
}

export function ParameterizedRunRows({ profileId, execKey, testKey, expanded, canWrite, onLoaded }: Props) {
  const query = useParameterizedRun(profileId, execKey, testKey, expanded);
  const client = useQueryClient();
  const [openIterations, setOpenIterations] = useState<Set<string>>(new Set());
  const [selectedIterations, setSelectedIterations] = useState<Set<string>>(new Set());
  const [selectedSteps, setSelectedSteps] = useState<Set<string>>(new Set());
  const [writeError, setWriteError] = useState("");
  const announcedRun = useRef("");
  const run = query.data;

  useEffect(() => {
    if (!run || run.iterations.length === 0 || announcedRun.current === run.id) return;
    announcedRun.current = run.id;
    onLoaded?.();
  }, [run, onLoaded]);

  // A selection belongs to the run it was made in. Moving to another execution
  // reuses this component, so without this the ticks would carry over onto
  // rows the user never chose (#365).
  const runID = run?.id ?? "";
  useEffect(() => {
    setSelectedIterations(new Set());
    setSelectedSteps(new Set());
  }, [runID]);

  if (!expanded) return null;
  if (query.isFetching && !run) return <p className="parameterized-run-message">Loading dataset iterations...</p>;
  if (query.error) return <p className="parameterized-run-error">{errMsg(query.error)}</p>;
  if (!run) return <p className="parameterized-run-message">Iteration details are unavailable.</p>;

  const completed = run.iterations.filter((item) => !["TODO", "EXECUTING"].includes(item.status)).length;
  // The appliers below are hoisted declarations, so the narrowing from the
  // guard above does not reach inside them. This carries it in.
  const iterations = run.iterations;

  // Each applier reports rather than throws, so a bulk apply can keep going
  // and name what did not take instead of stopping at the first refusal
  // (#365). The patch marks the result uncommitted because that is what it now
  // is: it is journalled and waits for Commit (#366), and nothing refetches
  // the run after a pick to discover that for us.
  async function applyIterationResult(iterationId: string, status: string): Promise<string> {
    try {
      await SetIterationStatus(profileId, execKey, testKey, iterationId, status);
      patchRun(client, profileId, execKey, testKey, (current) => ({
        ...current,
        iterations: current.iterations.map((item) =>
          item.id === iterationId ? { ...item, status, uncommitted: true } : item,
        ),
      }));
      return "";
    } catch (error) {
      return errMsg(error);
    }
  }

  async function applyStepResult(iterationId: string, stepId: string, status: string): Promise<string> {
    try {
      await SetIterationStepStatus(profileId, execKey, testKey, iterationId, stepId, status);
      patchRun(client, profileId, execKey, testKey, (current) => ({
        ...current,
        iterations: current.iterations.map((item) =>
          item.id === iterationId
            ? {
                ...item,
                steps: item.steps.map((step) =>
                  step.id === stepId ? { ...step, status, uncommitted: true } : step,
                ),
              }
            : item,
        ),
      }));
      return "";
    } catch (error) {
      return errMsg(error);
    }
  }

  async function setIterationStatus(iterationId: string, status: string) {
    setWriteError(await applyIterationResult(iterationId, status));
  }

  async function setStepStatus(iterationId: string, stepId: string, status: string) {
    setWriteError(await applyStepResult(iterationId, stepId, status));
  }

  // Applied one at a time rather than at once: these are local writes now, and
  // a sequential pass keeps the order of the journal rows the same as the order
  // of the rows on screen.
  async function applyToSelectedIterations(status: string) {
    const failures: string[] = [];
    for (const [index, iteration] of iterations.entries()) {
      if (!selectedIterations.has(iteration.id)) continue;
      const failure = await applyIterationResult(iteration.id, status);
      if (failure) failures.push(`iteration ${index + 1} (${failure})`);
    }
    setWriteError(bulkFailureMessage(failures));
    setSelectedIterations(new Set());
  }

  async function applyToSelectedSteps(iterationId: string, status: string) {
    const iteration = iterations.find((item) => item.id === iterationId);
    const failures: string[] = [];
    for (const [index, step] of (iteration?.steps ?? []).entries()) {
      if (!selectedSteps.has(stepKey(iterationId, step.id))) continue;
      const failure = await applyStepResult(iterationId, step.id, status);
      if (failure) failures.push(`step ${index + 1} (${failure})`);
    }
    setWriteError(bulkFailureMessage(failures));
    clearStepSelection(iterationId);
  }

  function clearStepSelection(iterationId: string) {
    setSelectedSteps((current) => {
      const next = new Set(current);
      for (const key of current) {
        if (key.startsWith(`${iterationId}\u0000`)) next.delete(key);
      }
      return next;
    });
  }

  return (
    <section className="parameterized-run" aria-label={`Dataset iterations for ${testKey}`}>
      <header className="parameterized-run-head">
        <strong>
          {completed} of {run.iterations.length} completed
        </strong>
        <span>
          {sourceName(run.datasetSourceKind)} {run.datasetSourceKey}
        </span>
        {!canWrite && <span className="parameterized-run-readonly">Read-only results</span>}
      </header>
      {writeError && (
        <p className="parameterized-run-error" role="alert">
          {writeError}
        </p>
      )}
      {canWrite && (
        <ResultBulkBar
          label={`Selected iterations of ${testKey}`}
          count={selectedIterations.size}
          onApply={(status) => void applyToSelectedIterations(status)}
          onClear={() => setSelectedIterations(new Set())}
        />
      )}
      <div className="parameterized-run-scroll">
        <table className="parameterized-run-table" aria-label={`Iterations for ${testKey}`}>
          <thead>
            <tr>
              {canWrite && (
                <th scope="col" className="parameterized-pick-col">
                  <input
                    type="checkbox"
                    aria-label={`Select every iteration of ${testKey}`}
                    checked={selectedIterations.size === run.iterations.length}
                    onChange={(event) =>
                      setSelectedIterations(
                        event.target.checked ? new Set(run.iterations.map((item) => item.id)) : new Set(),
                      )
                    }
                  />
                </th>
              )}
              <th scope="col">Iteration</th>
              <th scope="col">Result</th>
              <th scope="col">Parameters</th>
            </tr>
          </thead>
          <tbody>
            {run.iterations.map((iteration, index) => (
              <IterationRows
                key={iteration.id}
                iteration={iteration}
                index={index}
                open={openIterations.has(iteration.id)}
                canWrite={canWrite}
                selected={selectedIterations.has(iteration.id)}
                selectedSteps={selectedSteps}
                onToggle={() => setOpenIterations((current) => toggleSet(current, iteration.id))}
                onSelect={() => setSelectedIterations((current) => toggleSet(current, iteration.id))}
                onSelectStep={(stepId) => setSelectedSteps((current) => toggleSet(current, stepKey(iteration.id, stepId)))}
                onSelectEveryStep={(stepIds) =>
                  setSelectedSteps((current) => {
                    const next = new Set(current);
                    const keys = stepIds.map((id) => stepKey(iteration.id, id));
                    const all = keys.every((key) => next.has(key));
                    for (const key of keys) {
                      if (all) next.delete(key);
                      else next.add(key);
                    }
                    return next;
                  })
                }
                onApplyToSteps={(status) => void applyToSelectedSteps(iteration.id, status)}
                onClearSteps={() => clearStepSelection(iteration.id)}
                onIterationStatus={(status) => void setIterationStatus(iteration.id, status)}
                onStepStatus={(stepId, status) => void setStepStatus(iteration.id, stepId, status)}
              />
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function sourceName(kind: ParameterizedRun["datasetSourceKind"]): string {
  return kind === "execution" ? "Execution" : kind === "plan" ? "Plan" : "Test";
}

// Names what did not take and leaves the rest applied, rather than reporting
// one failure as if the whole apply had failed (#365).
function bulkFailureMessage(failures: string[]): string {
  if (failures.length === 0) return "";
  return `Not applied to ${failures.join(", ")}. The rest were applied.`;
}

function toggleSet(current: Set<string>, value: string): Set<string> {
  const next = new Set(current);
  if (next.has(value)) next.delete(value);
  else next.add(value);
  return next;
}

function patchRun(client: QueryClient, profileId: string, execKey: string, testKey: string, update: (run: ParameterizedRun) => ParameterizedRun) {
  client.setQueryData<ParameterizedRun>(keys.parameterizedRun(profileId, execKey, testKey), (current) => (current ? update(current) : current));
}
