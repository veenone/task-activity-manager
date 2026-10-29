// One iteration's row, and its steps when it is open.
//
// Split out of ParameterizedRunRows when the bulk selection controls (#365)
// took the file past the 400-line limit. It holds the status cells too, since
// nothing else renders them.

import { ResultBulkBar } from "./ResultBulkBar";
import { RUN_STATUSES } from "./constants";
import type { ParameterizedRun } from "../../api";


export function IterationRows({
  iteration,
  index,
  open,
  canWrite,
  selected,
  selectedSteps,
  onToggle,
  onSelect,
  onSelectStep,
  onSelectEveryStep,
  onApplyToSteps,
  onClearSteps,
  onIterationStatus,
  onStepStatus,
}: {
  iteration: ParameterizedRun["iterations"][number];
  index: number;
  open: boolean;
  canWrite: boolean;
  selected: boolean;
  selectedSteps: Set<string>;
  onToggle: () => void;
  onSelect: () => void;
  onSelectStep: (stepId: string) => void;
  onSelectEveryStep: (stepIds: string[]) => void;
  onApplyToSteps: (status: string) => void;
  onClearSteps: () => void;
  onIterationStatus: (status: string) => void;
  onStepStatus: (stepId: string, status: string) => void;
}) {
  const pickedSteps = iteration.steps.filter((step) => selectedSteps.has(stepKey(iteration.id, step.id)));
  return (
    <>
      <tr>
        {canWrite && (
          <td className="parameterized-pick-col">
            <input
              type="checkbox"
              aria-label={`Select iteration ${index + 1}`}
              checked={selected}
              onChange={onSelect}
            />
          </td>
        )}
        <th scope="row">
          <button type="button" className="parameterized-run-toggle" aria-expanded={open} onClick={onToggle}>
            <span aria-hidden="true">{open ? "▾" : "▸"}</span> Iteration {index + 1}
          </button>
        </th>
        <td>
          {canWrite ? <StatusSelect label={`Iteration ${index + 1} status`} value={iteration.status} onChange={onIterationStatus} /> : <StatusBadge status={iteration.status} />}
          {iteration.uncommitted && <UncommittedMark />}
        </td>
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
          <td colSpan={canWrite ? 4 : 3}>
            {iteration.steps.length === 0 ? (
              <span className="muted">No step results.</span>
            ) : (
              <>
                {canWrite && (
                  <ResultBulkBar
                    label={`Selected steps of iteration ${index + 1}`}
                    count={pickedSteps.length}
                    onApply={onApplyToSteps}
                    onClear={onClearSteps}
                  />
                )}
              <table className="parameterized-step-table" aria-label={`Steps for iteration ${index + 1}`}>
                <thead>
                  <tr>
                    {canWrite && (
                      <th scope="col" className="parameterized-pick-col">
                        <input
                          type="checkbox"
                          aria-label={`Select every step of iteration ${index + 1}`}
                          checked={pickedSteps.length === iteration.steps.length}
                          onChange={() => onSelectEveryStep(iteration.steps.map((step) => step.id))}
                        />
                      </th>
                    )}
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
                      {canWrite && (
                        <td className="parameterized-pick-col">
                          <input
                            type="checkbox"
                            aria-label={`Select step ${stepIndex + 1} of iteration ${index + 1}`}
                            checked={selectedSteps.has(stepKey(iteration.id, step.id))}
                            onChange={() => onSelectStep(step.id)}
                          />
                        </td>
                      )}
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
                        {step.uncommitted && <UncommittedMark />}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              </>
            )}
          </td>
        </tr>
      )}
    </>
  );
}

// A result that is journalled and not yet pushed. Results are local-first
// (#366), so without this a local pick is indistinguishable from the value Xray
// already holds.
function UncommittedMark() {
  return (
    <span className="parameterized-run-uncommitted" title="Set locally. Commit to send it to Jira.">
      not committed
    </span>
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

// A step is selected per iteration, so its key carries both. The separator is a
// NUL because an iteration id is Xray's own and a step id is built from it, and
// neither can contain one.
export function stepKey(iterationId: string, stepId: string): string {
  return `${iterationId}\u0000${stepId}`;
}
