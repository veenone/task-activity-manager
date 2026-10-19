import { useMemo, useRef, useState } from "react";
import { AnalyzeJUnitImportNewExec, ApplyJUnitImportNewExec, errMsg } from "../api";
import type { JUnitNewExecPreview, JUnitNewExecResult, JUnitNewExecRow } from "../api";
import { useProfile } from "../contexts/ProfileContext";
import { fileToBase64 } from "../files";
import { Modal } from "./Modal";
import "./junit-import.css";

interface Props {
  onCancel: () => void;
  onApplied: (result: JUnitNewExecResult) => void;
}

interface EditableRow extends JUnitNewExecRow {
  included: boolean;
  selectedIterationId: string;
}

function signature(parameters: { name: string; value: string }[]) {
  return parameters.map((parameter) => `${parameter.name}=${parameter.value}`).join(", ");
}

function initialRow(row: JUnitNewExecRow): EditableRow {
  const selected = row.targetKind === "iteration" && row.parameters.length > 0
    ? row.candidateIterations.find((candidate) => signature(candidate.parameters) === signature(row.parameters))
    : undefined;
  return { ...row, included: true, selectedIterationId: selected?.iterationId ?? "" };
}

function submissionRow(row: EditableRow): JUnitNewExecRow | null {
  if (!row.included) return null;
  if (row.targetKind !== "iteration") return row;
  const candidate = row.candidateIterations.find((item) => item.iterationId === row.selectedIterationId);
  if (!candidate) return null;
  return { ...row, parameters: candidate.parameters, reason: "" };
}

export function JUnitNewExecModal({ onCancel, onApplied }: Props) {
  const { activeId: profileId } = useProfile();
  const focusRef = useRef<HTMLInputElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const [summary, setSummary] = useState("");
  const [createMissing, setCreateMissing] = useState(true);
  const [preview, setPreview] = useState<JUnitNewExecPreview | null>(null);
  const [rows, setRows] = useState<EditableRow[]>([]);
  const [analyzeError, setAnalyzeError] = useState("");
  const [applyError, setApplyError] = useState("");
  const [analyzing, setAnalyzing] = useState(false);
  const [applying, setApplying] = useState(false);

  async function onFileChange(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    if (!file) return;
    event.target.value = "";
    setAnalyzeError("");
    setApplyError("");
    setPreview(null);
    setRows([]);
    setAnalyzing(true);
    try {
      const { b64 } = await fileToBase64(file);
      const result = await AnalyzeJUnitImportNewExec(profileId, b64, createMissing);
      setPreview(result);
      setRows(result.rows.map(initialRow));
    } catch (error) {
      setAnalyzeError(errMsg(error));
    } finally {
      setAnalyzing(false);
    }
  }

  function update(index: number, patch: Partial<EditableRow>) {
    setRows((current) => current.map((row, item) => item === index ? { ...row, ...patch } : row));
  }

  const resolvedRows = useMemo(
    () => rows.map(submissionRow).filter((row): row is JUnitNewExecRow => row !== null),
    [rows],
  );
  const includedCount = rows.filter((row) => row.included).length;
  const unresolved = includedCount !== resolvedRows.length;
  const duplicateTargets = useMemo(() => {
    const targets = new Set<string>();
    for (const row of resolvedRows) {
      const identity = row.create ? `new:${row.testcase}` : row.testKey;
      const target = `${identity}:${row.targetKind}:${signature(row.parameters)}`;
      if (targets.has(target)) return true;
      targets.add(target);
    }
    return false;
  }, [resolvedRows]);

  async function applyImport() {
    if (!summary.trim()) return;
    setApplying(true);
    setApplyError("");
    try {
      onApplied(await ApplyJUnitImportNewExec(profileId, summary.trim(), resolvedRows));
    } catch (error) {
      setApplyError(errMsg(error));
      setApplying(false);
    }
  }

  const canApply = !!preview && includedCount > 0 && !unresolved && !duplicateTargets && !!summary.trim() && !applying;
  const skippedCount = preview?.skipped.length ?? 0;

  return (
    <Modal initialFocus={focusRef} onClose={onCancel} className="modal bulk-modal" labelledBy="junit-newexec-title">
      <input ref={inputRef} type="file" accept=".xml" className="junit-file-input" onChange={onFileChange} />
      <div className="pending-head">
        <h2 id="junit-newexec-title">New execution from JUnit XML</h2>
        <button className="btn btn-ghost" onClick={onCancel} title="Close" aria-label="Close">×</button>
      </div>

      <div className="bulk-body junit-import-body">
        <div className="junit-new-exec-params">
          <label className="junit-new-exec-label">
            <span>Execution summary</span>
            <input ref={focusRef} className="junit-new-exec-input" value={summary} onChange={(event) => setSummary(event.target.value)} disabled={applying} />
          </label>
          <label className="junit-new-exec-checkbox">
            <input type="checkbox" checked={createMissing} onChange={(event) => setCreateMissing(event.target.checked)} disabled={applying} />
            <span>Create missing tests</span>
          </label>
          <button className="btn junit-new-exec-picker" onClick={() => inputRef.current?.click()} disabled={applying || analyzing || !summary.trim()}>
            {analyzing ? "Analyzing…" : preview ? "Choose another JUnit XML file" : "Choose JUnit XML file"}
          </button>
        </div>

        {analyzeError && <div className="error-text junit-apply-error">{analyzeError}</div>}
        {preview && (
          <>
            <p className="junit-import-summary"><strong>{includedCount}</strong> of <strong>{preview.total}</strong> testcase{preview.total !== 1 ? "s" : ""} ready{skippedCount > 0 ? `, ${skippedCount} skipped` : ""}.</p>
            {unresolved && <div className="error-text" role="alert">Choose an iteration for every included parameterized result.</div>}
            {duplicateTargets && <div className="error-text" role="alert">Two included testcases resolve to the same target. Exclude one or choose another iteration.</div>}
            {rows.length > 0 ? (
              <div className="junit-import-table-wrap">
                <table aria-label="Results to import" className="board-table junit-import-table">
                  <thead><tr><th scope="col">Include</th><th scope="col">Testcase</th><th scope="col">Test</th><th scope="col">Target</th><th scope="col">Result</th></tr></thead>
                  <tbody>
                    {rows.map((row, index) => {
                      const chosen = row.candidateIterations.find((candidate) => candidate.iterationId === row.selectedIterationId);
                      return (
                        <tr key={`${row.testcase}-${index}`} className={!row.included ? "junit-row-excluded" : ""}>
                          <td><input type="checkbox" checked={row.included} aria-label={`Include ${row.testcase}`} onChange={(event) => update(index, { included: event.target.checked })} /></td>
                          <td className="mono junit-testcase">{row.testcase}</td>
                          <td>{row.create ? <span className="kind-badge kind-testexec">NEW</span> : <span className="mono">{row.testKey}</span>}<span className="junit-test-summary">{row.summary}</span></td>
                          <td>
                            {row.targetKind === "iteration" ? (
                              <>
                                <select aria-label={`Iteration for ${row.testcase}`} value={row.selectedIterationId} disabled={!row.included} onChange={(event) => update(index, { selectedIterationId: event.target.value })}>
                                  <option value="">Needs mapping</option>
                                  {row.candidateIterations.map((candidate) => <option key={candidate.iterationId} value={candidate.iterationId}>{signature(candidate.parameters)}</option>)}
                                </select>
                                {!chosen && row.reason && <span className="junit-mapping-reason">{row.reason}</span>}
                              </>
                            ) : "Overall run"}
                          </td>
                          <td>{row.result ? <span className={`run-badge run-${row.result.toLowerCase()}`}>{row.result}</span> : <span className="muted">(none)</span>}</td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            ) : <p className="muted">No testcases are ready to import.</p>}
            {skippedCount > 0 && <details className="junit-skipped"><summary>Skipped testcases ({skippedCount})</summary>{preview.skipped.map((item, index) => <p key={`${item.testcase}-${index}`}><span className="mono">{item.testcase}</span>: {item.reason}</p>)}</details>}
            {applyError && <div className="error-text junit-apply-error">{applyError}</div>}
          </>
        )}
      </div>

      <div className="pending-actions">
        <button className="btn" onClick={onCancel} disabled={applying}>Cancel</button>
        {preview && <button className="btn btn-primary" onClick={applyImport} disabled={!canApply}>{applying ? "Creating…" : `Create execution and apply ${includedCount} result${includedCount !== 1 ? "s" : ""}`}</button>}
      </div>
    </Modal>
  );
}
