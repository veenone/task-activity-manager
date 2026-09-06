import { useState } from "react";
import type { ChangeEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Modal, call, errMsg, useProfile } from "@agile-suite/core";
import { AutoMapImport, IMPORT_FIELDS, ImportIssues, PreviewImport, SaveImportTemplate, readFileAsBase64 } from "../api";
import type { ImportMapping, ImportPreview, ImportResult, Profile, Settings } from "../api";
import { invalidateWrites } from "../queries/invalidate";
import { useSync } from "../contexts/SyncContext";
import { plural } from "../lib/format";

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

// resultLine words a dry run or an import.
export function resultLine(r: ImportResult, dryRun: boolean): string {
  const skipped = r.errors.length;
  if (dryRun) {
    const ok = r.rows - skipped;
    return `Dry run: ${ok} ${ok === 1 ? "row would become a draft" : "rows would become drafts"}, ${skipped} would be skipped.`;
  }
  return `Imported ${plural(r.created.length, "draft", "drafts")}; ${plural(skipped, "row was", "rows were")} skipped.`;
}

// ImportIssuesModal turns a CSV or XLSX into drafts: pick, preview, map,
// dry run, import. The file's bytes go to the backend base64-encoded.
export function ImportIssuesModal({ onClose, onImported }: Props) {
  const { activeId, activeProfile } = useProfile<Profile, Settings>();
  const qc = useQueryClient();
  const { status } = useSync();
  const [picked, setPicked] = useState<Picked | null>(null);
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [mapping, setMapping] = useState<ImportMapping>(EMPTY);
  const [result, setResult] = useState<{ r: ImportResult; dryRun: boolean } | null>(null);
  const [error, setError] = useState("");
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const locked = busy || status !== "idle";

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

  async function run(dryRun: boolean) {
    if (!picked) return;
    if (!mapping.summary) {
      setError("Map a Summary column first.");
      return;
    }
    setError("");
    setBusy(true);
    try {
      const r = await call(() => ImportIssues(activeId, picked.b64, picked.isXlsx, picked.name, mapping, dryRun));
      setResult({ r, dryRun });
      if (!dryRun && r.created.length > 0) {
        invalidateWrites(qc, activeId);
        onImported(r.created);
        // Clear the picked file so a second click cannot re-import the same
        // rows; the result banner and the mapping display stay as they are.
        setPicked(null);
      }
    } catch (err) {
      setError(errMsg(err));
    } finally {
      setBusy(false);
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
                {preview.headers.map((h, i) => (
                  <option key={`${i}-${h}`} value={h}>{h || `(column ${i + 1})`}</option>
                ))}
              </select>
            </label>
          ))}
        </div>
      )}

      {result && (
        <div className={`pending-banner${result.r.errors.length ? " pending-banner-warn" : ""}`} role="status">
          <p className="b">{resultLine(result.r, result.dryRun)}</p>
          {result.r.errors.map((e) => (
            <p key={`${e.row}-${e.message}`} className="small">
              <span className="danger-text">{`Row ${e.row}`}</span>{" "}
              <span>{e.message}</span>
            </p>
          ))}
          <p className="muted small">Types default to Task. Drafts join the Backlog now; Commit creates them in Jira.</p>
        </div>
      )}

      {error && <p className="error-text small" role="alert">{error}</p>}

      <div className="pending-footer">
        <span className="muted small">Import runs in one transaction; skipped rows stay in the file for a second pass.</span>
        <span className="pending-footer-buttons">
          <button type="button" className="btn" disabled={!picked || locked} onClick={() => void run(true)}>Dry run</button>
          <button type="button" className="btn btn-primary" disabled={!picked || locked} onClick={() => void run(false)}>
            {result?.dryRun ? `Import ${result.r.rows - result.r.errors.length}` : "Import"}
          </button>
        </span>
      </div>
    </Modal>
  );
}
