import { useEffect, useState } from "react";
import type { ChangeEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Modal, call, errMsg, useNotice, useProfile } from "@agile-suite/core";
import { AutoMapImport, IMPORT_FIELDS, ImportIssues, PreviewImport, SaveImportTemplate, readFileAsBase64 } from "../api";
import type { ImportMapping, ImportPreview, ImportResult, Profile, Settings } from "../api";
import { invalidateWrites } from "../queries/invalidate";
import { useSync } from "../contexts/SyncContext";
import { plural } from "../lib/format";
import { useDebounced } from "../lib/useDebounced";

interface Props {
  onClose: () => void;
  onImported: (keys: string[]) => void;
}

interface Picked {
  name: string;
  b64: string;
  isXlsx: boolean;
}

const EMPTY: ImportMapping = { type: "", summary: "", description: "", priority: "", labels: "", assignee: "", storyPoints: "", parentKey: "" };
const PREFLIGHT_DELAY_MS = 250;

// Preflight is the automatic dry run that keeps the Import button and the
// validation line in step with the current file and mapping.
type Preflight =
  | { kind: "none" }
  | { kind: "needsSummary" }
  | { kind: "running" }
  | { kind: "ready"; r: ImportResult };

// validationLine words a dry run the way XTM's import dialog does: how many
// of the file's rows are valid, and how many will be skipped.
function validationLine(r: ImportResult): string {
  const skipped = r.errors.length;
  const valid = r.rows - skipped;
  return `${valid} valid ${valid === 1 ? "row" : "rows"}${skipped > 0 ? `, ${skipped} skipped` : ""}.`;
}

// resultLine words a finished import that created at least one draft.
function resultLine(r: ImportResult): string {
  const skipped = r.errors.length;
  return `✓ Imported ${plural(r.created.length, "draft", "drafts")} as pending creates${skipped > 0 ? ` (${skipped} skipped)` : ""}. Commit them from the Pending changes dialog.`;
}

// ImportIssuesModal turns a CSV or XLSX into drafts: pick, map, and import.
// Every mapping change runs a debounced dry run in the background so the
// Import button always reflects how many rows will actually become drafts.
// The file's bytes go to the backend base64-encoded.
export function ImportIssuesModal({ onClose, onImported }: Props) {
  const { activeId } = useProfile<Profile, Settings>();
  const qc = useQueryClient();
  const { status } = useSync();
  const { notice } = useNotice();
  const [picked, setPicked] = useState<Picked | null>(null);
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [mapping, setMapping] = useState<ImportMapping>(EMPTY);
  const [preflight, setPreflight] = useState<Preflight>({ kind: "none" });
  const [result, setResult] = useState<ImportResult | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [importing, setImporting] = useState(false);
  const locked = busy || importing || status !== "idle";

  const debouncedMapping = useDebounced(mapping, PREFLIGHT_DELAY_MS, picked?.name ?? "");

  // Run the preflight after a file is picked (right away, since the
  // debounce resets on a new file) and after every mapping change (once the
  // debounce settles).
  useEffect(() => {
    if (!picked) {
      setPreflight({ kind: "none" });
      return;
    }
    if (!debouncedMapping.summary) {
      setPreflight({ kind: "needsSummary" });
      return;
    }
    let cancelled = false;
    setPreflight({ kind: "running" });
    void (async () => {
      try {
        const r = await call(() => ImportIssues(activeId, picked.b64, picked.isXlsx, picked.name, debouncedMapping, true));
        if (!cancelled) setPreflight({ kind: "ready", r });
      } catch (err) {
        if (!cancelled) setError(errMsg(err));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [activeId, picked, debouncedMapping]);

  async function revalidate() {
    if (!picked) return;
    if (!mapping.summary) {
      setPreflight({ kind: "needsSummary" });
      return;
    }
    setPreflight({ kind: "running" });
    try {
      const r = await call(() => ImportIssues(activeId, picked.b64, picked.isXlsx, picked.name, mapping, true));
      setPreflight({ kind: "ready", r });
    } catch (err) {
      setError(errMsg(err));
    }
  }

  async function onFile(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setError("");
    setResult(null);
    setPreview(null);
    setBusy(true);
    try {
      const b64 = await readFileAsBase64(file);
      const isXlsx = /\.xlsx$/i.test(file.name);
      const pv = await call(() => PreviewImport(b64, isXlsx));
      setPicked({ name: file.name, b64, isXlsx });
      setPreview(pv);
      setMapping(await call(() => AutoMapImport(pv.headers)));
    } catch (err) {
      setPicked(null);
      setError(errMsg(err));
    } finally {
      setBusy(false);
    }
  }

  const validRows = preflight.kind === "ready" ? preflight.r.rows - preflight.r.errors.length : 0;
  const importDisabled = locked || preflight.kind !== "ready" || validRows === 0;
  const success = result !== null && result.created.length > 0;

  async function runImport() {
    if (!picked || importDisabled) return;
    setError("");
    setImporting(true);
    try {
      const r = await call(() => ImportIssues(activeId, picked.b64, picked.isXlsx, picked.name, mapping, false));
      setResult(r);
      if (r.created.length > 0) {
        invalidateWrites(qc, activeId);
        onImported(r.created);
        // Clear the picked file so a second click cannot re-import the same
        // rows; a zero-draft result leaves it in place so the user can fix
        // the mapping and try again.
        setPicked(null);
      }
    } catch (err) {
      setError(errMsg(err));
    } finally {
      setImporting(false);
    }
  }

  async function template() {
    try {
      const path = await call(() => SaveImportTemplate());
      if (path) await notice({ title: "Template saved", message: path });
    } catch (err) {
      await notice({ title: "Template export failed", message: errMsg(err), tone: "error" });
    }
  }

  return (
    <Modal onClose={onClose} className="modal pending-modal" labelledBy="import-issues-title">
      <div className="pending-head">
        <h2 id="import-issues-title">Import issues (CSV or XLSX)</h2>
        <button className="btn btn-ghost" onClick={onClose} title="Close">
          ✕
        </button>
      </div>

      <div className="bulk-body">
        {!success && (
          <>
            <div className="import-row">
              <input type="file" accept=".csv,.xlsx,text/csv" aria-label="File" onChange={(e) => void onFile(e)} disabled={locked} />
              <button className="link-btn" onClick={() => void template()} disabled={locked}>
                Download template
              </button>
            </div>
            {picked && preview && (
              <p className="muted">
                {picked.name} ({plural(preview.rowCount, "row", "rows")})
              </p>
            )}

            {preview && (
              <div className="import-mapping">
                {IMPORT_FIELDS.map((f) => (
                  <label key={f.id} className="bulk-row">
                    <span>
                      {f.label}
                      {f.id === "summary" ? " *" : ""}
                    </span>
                    <select
                      value={mapping[f.id]}
                      onChange={(e) => setMapping((m) => ({ ...m, [f.id]: e.target.value }))}
                      disabled={locked}
                    >
                      <option value="">(not mapped)</option>
                      {preview.headers.map((h, i) => {
                        const label = h || `(column ${i + 1})`;
                        const sample = preview.sample[i]?.trim();
                        return (
                          <option key={`${i}-${h}`} value={h}>
                            {sample ? `${label} (e.g. ${sample})` : label}
                          </option>
                        );
                      })}
                    </select>
                  </label>
                ))}
              </div>
            )}

            {!result && picked && preflight.kind !== "none" && (
              <div className="import-validation">
                {preflight.kind === "needsSummary" && <p className="muted">Map a Summary column first.</p>}
                {preflight.kind === "ready" && (
                  <>
                    <p className={preflight.r.errors.length ? "warn-text" : "ok-text"}>{validationLine(preflight.r)}</p>
                    {preflight.r.errors.length > 0 && (
                      <ul className="commit-fail-list">
                        {preflight.r.errors.slice(0, 20).map((er, i) => (
                          <li key={i}>row {er.row}: {er.message}</li>
                        ))}
                      </ul>
                    )}
                  </>
                )}
              </div>
            )}

            {result && result.created.length === 0 && (
              <div className="import-validation">
                <p className="warn-text">Nothing was imported.</p>
                {result.errors.length > 0 && (
                  <ul className="commit-fail-list">
                    {result.errors.slice(0, 20).map((er, i) => (
                      <li key={i}>row {er.row}: {er.message}</li>
                    ))}
                  </ul>
                )}
              </div>
            )}

            {error && <div className="error-text">{error}</div>}
          </>
        )}

        {success && result && <p className="ok-text">{resultLine(result)}</p>}
      </div>

      <div className="pending-actions">
        {!success ? (
          <>
            <button className="btn" onClick={onClose} disabled={locked}>
              Cancel
            </button>
            <button className="btn" onClick={() => void revalidate()} disabled={locked || !picked}>
              Validate
            </button>
            <button className="btn btn-primary" onClick={() => void runImport()} disabled={importDisabled}>
              {importing ? "Working…" : "Import"}
            </button>
          </>
        ) : (
          <button className="btn btn-primary" onClick={onClose}>
            Done
          </button>
        )}
      </div>
    </Modal>
  );
}
