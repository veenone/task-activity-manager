import { useEffect, useRef, useState } from "react";
import { useQueryClient, type QueryClient } from "@tanstack/react-query";

import { SetIterationStatus, SetIterationStepStatus, errMsg, type ParameterizedRun } from "../../api";
import { keys } from "../../queries/keys";
import { useParameterizedRun } from "../../queries/parameterizedRuns";
import { RUN_STATUSES } from "./constants";
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
  const [writeError, setWriteError] = useState("");
  const announcedRun = useRef("");
  const run = query.data;

  useEffect(() => {
    if (!run || run.iterations.length === 0 || announcedRun.current === run.id) return;
    announcedRun.current = run.id;
    onLoaded?.();
  }, [run, onLoaded]);

  if (!expanded) return null;
  if (query.isFetching && !run) return <p className="parameterized-run-message">Loading dataset iterations...</p>;
  if (query.error) return <p className="parameterized-run-error">{errMsg(query.error)}</p>;
  if (!run) return <p className="parameterized-run-message">Iteration details are unavailable.</p>;

  const completed = run.iterations.filter((item) => !["TODO", "EXECUTING"].includes(item.status)).length;

  async function setIterationStatus(iterationId: string, status: string) {
    setWriteError("");
    try {
      await SetIterationStatus(profileId, execKey, testKey, iterationId, status);
      patchRun(client, profileId, execKey, testKey, (current) => ({
        ...current,
        iterations: current.iterations.map((item) => (item.id === iterationId ? { ...item, status } : item)),
      }));
    } catch (error) {
      setWriteError(errMsg(error));
    }
  }

  async function setStepStatus(iterationId: string, stepId: string, status: string) {
    setWriteError("");
    try {
      await SetIterationStepStatus(profileId, execKey, testKey, iterationId, stepId, status);
      patchRun(client, profileId, execKey, testKey, (current) => ({
        ...current,
        iterations: current.iterations.map((item) =>
          item.id === iterationId
            ? {
                ...item,
                steps: item.steps.map((step) => (step.id === stepId ? { ...step, status } : step)),
              }
            : item,
        ),
      }));
    } catch (error) {
      setWriteError(errMsg(error));
    }
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
      <div className="parameterized-run-scroll">
        <table className="parameterized-run-table" aria-label={`Iterations for ${testKey}`}>
          <thead>
            <tr>
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
                onToggle={() => setOpenIterations((current) => toggleSet(current, iteration.id))}
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

function IterationRows({
  iteration,
  index,
  open,
  canWrite,
  onToggle,
  onIterationStatus,
  onStepStatus,
}: {
  iteration: ParameterizedRun["iterations"][number];
  index: number;
  open: boolean;
  canWrite: boolean;
  onToggle: () => void;
  onIterationStatus: (status: string) => void;
  onStepStatus: (stepId: string, status: string) => void;
}) {
  return (
    <>
      <tr>
        <th scope="row">
          <button type="button" className="parameterized-run-toggle" aria-expanded={open} onClick={onToggle}>
            <span aria-hidden="true">{open ? "▾" : "▸"}</span> Iteration {index + 1}
          </button>
        </th>
        <td>{canWrite ? <StatusSelect label={`Iteration ${index + 1} status`} value={iteration.status} onChange={onIterationStatus} /> : <StatusBadge status={iteration.status} />}</td>
        <td>
          <dl className="parameterized-values">
            {iteration.parameters.map((parameter) => (
              <div key={parameter.name}>
                <dt>{parameter.name}</dt>
                <dd>{parameter.value}</dd>
              </div>
            ))}
          </dl>
        </td>
      </tr>
      {open && (
        <tr className="parameterized-step-row">
          <td colSpan={3}>
            {iteration.steps.length === 0 ? (
              <span className="muted">No step results.</span>
            ) : (
              <table className="parameterized-step-table" aria-label={`Steps for iteration ${index + 1}`}>
                <thead>
                  <tr>
                    <th scope="col">Step</th>
                    <th scope="col">Action</th>
                    <th scope="col">Data</th>
                    <th scope="col">Expected</th>
                    <th scope="col">Result</th>
                  </tr>
                </thead>
                <tbody>
                  {iteration.steps.map((step, stepIndex) => (
                    <tr key={step.id}>
                      <th scope="row">{stepIndex + 1}</th>
                      <td>{step.action || "-"}</td>
                      <td>{step.data || "-"}</td>
                      <td>{step.expected || "-"}</td>
                      <td>
                        {canWrite ? (
                          <StatusSelect label={`Step ${stepIndex + 1} status`} value={step.status} onChange={(status) => onStepStatus(step.id, status)} />
                        ) : (
                          <StatusBadge status={step.status} />
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </td>
        </tr>
      )}
    </>
  );
}

function StatusSelect({ label, value, onChange }: { label: string; value: string; onChange: (status: string) => void }) {
  return (
    <select className={`run-select run-${value.toLowerCase()}`} aria-label={label} value={value} onChange={(event) => onChange(event.target.value)}>
      {RUN_STATUSES.map((status) => (
        <option key={status}>{status}</option>
      ))}
    </select>
  );
}

function StatusBadge({ status }: { status: string }) {
  return <span className={`run-badge run-${status.toLowerCase()}`}>{status}</span>;
}

function sourceName(kind: ParameterizedRun["datasetSourceKind"]): string {
  return kind === "execution" ? "Execution" : kind === "plan" ? "Plan" : "Test";
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
