import { useEffect, useRef, useState } from "react";
import type { ChangeEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Modal, call, errMsg, useProfile } from "@agile-suite/core";
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

// Preflight is the automatic dry run that keeps the Import button and its
// row count in step with the current file and mapping.
type Preflight =
  | { kind: "none" }
  | { kind: "needsSummary" }
  | { kind: "running" }
  | { kind: "ready"; r: ImportResult }
  | { kind: "error"; message: string };

// preflightLine words the automatic dry run: how many of the file's rows
// will become drafts and how many will be skipped.
function preflightLine(r: ImportResult): string {
  const skipped = r.errors.length;
  return `${r.rows - skipped} of ${plural(r.rows, "row", "rows")} will become drafts; ${skipped} will be skipped.`;
}

// resultLine words a finished import that created at least one draft.
function resultLine(r: ImportResult): string {
  return `Imported ${plural(r.created.length, "draft", "drafts")}; ${plural(r.errors.length, "row was", "rows were")} skipped.`;
}

// ImportIssuesModal turns a CSV or XLSX into drafts: pick, map, and import.
// Every mapping change runs a debounced dry run in the background so the
// Import button always shows how many rows will actually become drafts. The
// file's bytes go to the backend base64-encoded.
export function ImportIssuesModal({ onClose, onImported }: Props) {
  const { activeId, activeProfile } = useProfile<Profile, Settings>();
  const qc = useQueryClient();
  const { status } = useSync();
  const [picked, setPicked] = useState<Picked | null>(null);
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [mapping, setMapping] = useState<ImportMapping>(EMPTY);
  const [preflight, setPreflight] = useState<Preflight>({ kind: "none" });
  const [result, setResult] = useState<ImportResult | null>(null);
  const [error, setError] = useState("");
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [importing, setImporting] = useState(false);
  const resultRef = useRef<HTMLDivElement>(null);
  const locked = busy || importing || status !== "idle";

  const debouncedMapping = useDebounced(mapping, PREFLIGHT_DELAY_MS, picked?.name ?? "");

  // Run the preflight after a file is picked (right away, since the
  // debounce resets on a new file) and after every mapping change (after
  // the debounce settles).
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
        if (!cancelled) setPreflight({ kind: "error", message: errMsg(err) });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [activeId, picked, debouncedMapping]);

  useEffect(() => {
    if (result) resultRef.current?.scrollIntoView?.({ block: "nearest" });
  }, [result]);

  async function onFile(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setError("");
    setResult(null);
    setPreview(null);
    setNote("");
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
        // rows; the result banner and the mapping display stay as they are.
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
      setNote(path ? `Template saved to ${path}` : "");
    } catch (err) {
      setError(errMsg(err));
    }
  }

  return (
    <Modal onClose={onClose} className="modal import-modal" labelledBy="import-title">
      <div className="pending-head">
        <h2 id="import-title">Import issues</h2>
        <span className="muted">{activeProfile ? `into ${activeProfile.projectKey}` : ""}</span>
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>

      <div className="import-file">
        <label className="edit-row" htmlFor="import-file">
          <span className="muted small">File</span>
          <input id="import-file" type="file" accept=".csv,.xlsx" onChange={(e) => void onFile(e)} disabled={locked} />
        </label>
        {preview && picked && (
          <p className="muted small">
            {picked.name}: <span>{`${plural(preview.headers.length, "column", "columns")}, ${plural(preview.rowCount, "row", "rows")}`}</span>
          </p>
        )}
        <button type="button" className="btn btn-ghost" disabled={locked} onClick={() => void template()}>Download template</button>
        {note && <span className="muted small" role="status">{note}</span>}
      </div>

      {preview && (
        <div className="import-mapping">
          <div className="import-mapping-head"><span className="muted small b">Field</span><span className="muted small b">Column</span></div>
          {IMPORT_FIELDS.map((f) => (
            <label key={f.id} className="edit-row" htmlFor={`map-${f.id}`}>
              <span className="muted small">{f.label}</span>
              <select id={`map-${f.id}`} className="detail-input" value={mapping[f.id]} onChange={(e) => setMapping((m) => ({ ...m, [f.id]: e.target.value }))} disabled={locked}>
                <option value="">(not mapped)</option>
                {preview.headers.map((h, i) => {
                  const label = h || `(column ${i + 1})`;
                  const sample = preview.sample[i]?.trim();
                  return (
                    <option key={`${i}-${h}`} value={h}>{sample ? `${label} (e.g. ${sample})` : label}</option>
                  );
                })}
              </select>
            </label>
          ))}
        </div>
      )}

      {!result && picked && preflight.kind !== "none" && (
        <div
          className={`pending-banner${preflight.kind === "ready" && preflight.r.errors.length ? " pending-banner-warn" : ""}`}
          role={preflight.kind === "error" ? "alert" : "status"}
        >
          {preflight.kind === "needsSummary" && <p className="b">Map a Summary column first.</p>}
          {preflight.kind === "error" && <p className="error-text small">{preflight.message}</p>}
          {preflight.kind === "ready" && (
            <>
              <p className="b">{preflightLine(preflight.r)}</p>
              {preflight.r.errors.length > 0 && (
                <div className="import-errors">
                  {preflight.r.errors.map((e) => (
                    <p key={`${e.row}-${e.message}`}>
                      <span className="danger-text">{`Row ${e.row}`}</span>{" "}
                      <span>{e.message}</span>
                    </p>
                  ))}
                </div>
              )}
            </>
          )}
        </div>
      )}

      {result && (
        <div
          ref={resultRef}
          className={`pending-banner${result.created.length === 0 ? " pending-banner-fail" : result.errors.length ? " pending-banner-warn" : ""}`}
          role={result.created.length === 0 ? "alert" : "status"}
        >
          {result.created.length === 0 ? (
            <>
              <p className="b">Nothing was imported.</p>
              <p>Every row was skipped. Fix the rows below and pick the file again.</p>
            </>
          ) : (
            <p className="b">{resultLine(result)}</p>
          )}
          {result.errors.length > 0 && (
            <div className="import-errors">
              {result.errors.map((e) => (
                <p key={`${e.row}-${e.message}`}>
                  <span className="danger-text">{`Row ${e.row}`}</span>{" "}
                  <span>{e.message}</span>
                </p>
              ))}
            </div>
          )}
          {result.created.length > 0 && (
            <p className="muted small">Types default to Task. Drafts join the Backlog now; Commit creates them in Jira.</p>
          )}
        </div>
      )}

      {error && <p className="error-text small" role="alert">{error}</p>}

      <div className="pending-footer">
        <span className="muted small">Rows that already became drafts are skipped, so the same file can be imported again after fixing the rest.</span>
        <span className="pending-footer-buttons">
          <button type="button" className="btn btn-primary" disabled={importDisabled} onClick={() => void runImport()}>{`Import ${validRows}`}</button>
        </span>
      </div>
    </Modal>
  );
}
