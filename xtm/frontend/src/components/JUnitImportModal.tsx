import { useEffect, useMemo, useRef, useState } from "react";
import { useProfile } from "../contexts/ProfileContext";
import { AnalyzeJUnitImport, ApplyJUnitImport, errMsg } from "../api";
import type { JUnitImportPreview, JUnitMapping, JUnitMatch } from "../api";
import { fileToBase64 } from "../files";
import { Modal } from "./Modal";
import "./junit-import.css";

interface Props {
  execKey: string;
  onCancel: () => void;
  onApplied: (succeeded: number, failed: number) => void;
}

interface EditableMapping extends JUnitMapping {
  selectedIterationId: string;
}

function signature(parameters: { name: string; value: string }[]) {
  return parameters.map((parameter) => `${parameter.name}=${parameter.value}`).join(", ");
}

function resolvedMatch(mapping: EditableMapping): JUnitMatch | null {
  if (!mapping.included) return null;
  if (mapping.match?.targetKind === "run") return mapping.match;
  const candidate = mapping.candidateIterations.find(
    (iteration) => iteration.iterationId === mapping.selectedIterationId,
  );
  if (!candidate) return null;
  return {
    testcase: mapping.testcase,
    testKey: mapping.testKey,
    summary: mapping.summary,
    result: mapping.result,
    currentRun: candidate.currentRun,
    targetKind: "iteration",
    iterationId: candidate.iterationId,
    parameters: candidate.parameters,
    datasetSourceKind: mapping.datasetSourceKind,
    datasetSourceKey: mapping.datasetSourceKey,
  };
}

export function JUnitImportModal({ execKey, onCancel, onApplied }: Props) {
  const { activeId: profileId } = useProfile();
  const [preview, setPreview] = useState<JUnitImportPreview | null>(null);
  const [mappings, setMappings] = useState<EditableMapping[]>([]);
  const [analyzeError, setAnalyzeError] = useState("");
  const [applying, setApplying] = useState(false);
  const [applyError, setApplyError] = useState("");
  const filePickedRef = useRef(false);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    inputRef.current?.click();
    function onFocus() {
      setTimeout(() => {
        if (!filePickedRef.current) onCancel();
      }, 300);
    }
    window.addEventListener("focus", onFocus, { once: true });
    return () => window.removeEventListener("focus", onFocus);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  async function onFileChange(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    if (!file) {
      onCancel();
      return;
    }
    filePickedRef.current = true;
    setAnalyzeError("");
    try {
      const { b64 } = await fileToBase64(file);
      const result = await AnalyzeJUnitImport(profileId, execKey, b64);
      setPreview(result);
      setMappings(result.mappings.map((mapping) => ({
        ...mapping,
        selectedIterationId: mapping.match?.iterationId ?? "",
      })));
    } catch (error) {
      setAnalyzeError(errMsg(error));
    }
  }

  const resolved = useMemo(
    () => mappings.map(resolvedMatch).filter((match): match is JUnitMatch => match !== null),
    [mappings],
  );
  const includedCount = mappings.filter((mapping) => mapping.included).length;
  const unresolved = includedCount !== resolved.length;
  const duplicateTargets = useMemo(() => {
    const seen = new Set<string>();
    for (const match of resolved) {
      const key = `${match.targetKind}:${match.testKey}:${match.iterationId}`;
      if (seen.has(key)) return true;
      seen.add(key);
    }
    return false;
  }, [resolved]);

  function update(index: number, patch: Partial<EditableMapping>) {
    setMappings((current) => current.map((mapping, row) => row === index ? { ...mapping, ...patch } : mapping));
  }

  async function applyImport() {
    setApplying(true);
    setApplyError("");
    try {
      const result = await ApplyJUnitImport(profileId, execKey, resolved);
      onApplied(result.succeeded?.length ?? 0, result.failed?.length ?? 0);
    } catch (error) {
      setApplyError(errMsg(error));
      setApplying(false);
    }
  }

  const canApply = includedCount > 0 && !unresolved && !duplicateTargets && !applying;

  return (
    <Modal onClose={onCancel} className="modal bulk-modal" labelledBy="junit-import-title">
      <input ref={inputRef} type="file" accept=".xml" className="junit-file-input" onChange={onFileChange} />
      <div className="pending-head">
        <h2 id="junit-import-title">Import results (JUnit XML)</h2>
        <button className="btn btn-ghost" onClick={onCancel} title="Close" aria-label="Close">✕</button>
      </div>

      <div className="bulk-body junit-import-body">
        {!preview && !analyzeError && <p className="muted">Analyzing…</p>}
        {analyzeError && <div className="error-text">{analyzeError}</div>}
        {preview && (
          <>
            <p className="junit-import-summary">
              Parsed <strong>{preview.total}</strong> testcase{preview.total !== 1 ? "s" : ""}.{" "}
              <strong>{resolved.length}</strong> ready, <strong>{preview.total - includedCount}</strong> excluded.
            </p>
            {duplicateTargets && <div className="error-text" role="alert">Two included testcases resolve to the same target. Exclude one or choose another iteration.</div>}
            {mappings.length > 0 ? (
              <div className="junit-import-table-wrap">
                <table aria-label="JUnit result mappings" className="board-table junit-import-table">
                  <thead><tr><th scope="col">Include</th><th scope="col">Testcase</th><th scope="col">Test</th><th scope="col">Target</th><th scope="col">Current</th><th scope="col">New</th></tr></thead>
                  <tbody>
                    {mappings.map((mapping, index) => {
                      const match = resolvedMatch(mapping);
                      const chosen = mapping.candidateIterations.find((candidate) => candidate.iterationId === mapping.selectedIterationId);
                      return (
                        <tr key={`${mapping.testcase}-${index}`} className={!mapping.included ? "junit-row-excluded" : ""}>
                          <td><input type="checkbox" checked={mapping.included} aria-label={`Include ${mapping.testcase}`} onChange={(event) => update(index, { included: event.target.checked })} /></td>
                          <td className="mono junit-testcase">{mapping.testcase}</td>
                          <td><span className="mono">{mapping.testKey || "-"}</span><span className="junit-test-summary">{mapping.summary}</span></td>
                          <td>
                            {mapping.candidateIterations.length > 0 ? (
                              <>
                                <select aria-label={`Iteration for ${mapping.testcase}`} value={mapping.selectedIterationId} disabled={!mapping.included} onChange={(event) => update(index, { selectedIterationId: event.target.value })}>
                                  <option value="">Needs mapping</option>
                                  {mapping.candidateIterations.map((candidate) => <option key={candidate.iterationId} value={candidate.iterationId}>{signature(candidate.parameters)}</option>)}
                                </select>
                                {chosen && <details className="junit-parameters" open><summary>Parameter values</summary>{chosen.parameters.map((parameter) => <span key={parameter.name}>{parameter.name}={parameter.value}</span>)}</details>}
                                {!chosen && mapping.reason && <span className="junit-mapping-reason">{mapping.reason}</span>}
                              </>
                            ) : match ? "Overall run" : <span className="muted">Not available</span>}
                          </td>
                          <td>{match?.currentRun ? <span className={`run-badge run-${match.currentRun.toLowerCase()}`}>{match.currentRun}</span> : <span className="muted">(none)</span>}</td>
                          <td><span className={`run-badge run-${mapping.result.toLowerCase()}`}>{mapping.result || "-"}</span></td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            ) : <p className="muted">No testcases found in the XML file.</p>}
            {applyError && <div className="error-text junit-apply-error">{applyError}</div>}
          </>
        )}
      </div>

      <div className="pending-actions">
        <button className="btn" onClick={onCancel} disabled={applying}>Cancel</button>
        {preview && <button className="btn btn-primary" onClick={applyImport} disabled={!canApply}>{applying ? "Applying…" : `Apply ${includedCount} result${includedCount !== 1 ? "s" : ""}`}</button>}
      </div>
    </Modal>
  );
}
