import { useState } from "react";
import type { ChangeEvent } from "react";
import { useProfile } from "../contexts/ProfileContext";
import {
  ExportPreconditionImportTemplate,
  GetPreconditionCreateFields,
  ImportPreconditions,
  PreviewImport,
  errMsg,
} from "../api";
import type { PreconditionImportMapping, PreconditionImportResult } from "../api";
import { fileToBase64 } from "../files";
import { CreateFieldInputs, useCreateScreenFields } from "./CreateFieldInputs";
import { Modal } from "./Modal";
import { useNotice } from "./useNotice";

interface Props {
  onComplete: () => void;
  onCancel: () => void;
}

const FIELDS: Array<{ key: keyof PreconditionImportMapping; label: string; required?: boolean }> = [
  { key: "summary", label: "Summary", required: true },
  { key: "type", label: "Type" },
  { key: "condition", label: "Condition" },
  { key: "description", label: "Description" },
  { key: "labels", label: "Labels" },
];

// guessMapping pre-selects each field's column by its header name.
function guessMapping(headers: string[]): PreconditionImportMapping {
  const find = (name: string) => headers.find((h) => h.trim().toLowerCase() === name) ?? "";
  return {
    summary: find("summary"),
    type: find("type"),
    condition: find("condition"),
    description: find("description"),
    labels: find("labels"),
  };
}

function plural(n: number, word: string) {
  return `${n} ${word}${n === 1 ? "" : "s"}`;
}

// ImportPreconditionsModal imports preconditions from a CSV or XLSX file: pick
// a file, map its columns, answer the create screen's required fields once for
// every row, validate, then import the new rows as pending creates. A row
// whose summary already exists is skipped. Any row error stops the import
// with nothing queued, whether or not Validate was pressed first.
export function ImportPreconditionsModal({ onComplete, onCancel }: Props) {
  const { activeId: profileId } = useProfile();
  const { notice } = useNotice();
  const extra = useCreateScreenFields(() => GetPreconditionCreateFields(profileId), profileId);
  const [content, setContent] = useState("");
  const [isXlsx, setIsXlsx] = useState(false);
  const [fileName, setFileName] = useState("");
  const [headers, setHeaders] = useState<string[]>([]);
  const [rowCount, setRowCount] = useState(0);
  const [mapping, setMapping] = useState<PreconditionImportMapping>(guessMapping([]));
  const [validation, setValidation] = useState<PreconditionImportResult | null>(null);
  const [result, setResult] = useState<PreconditionImportResult | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function pickFile(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setFileName(file.name);
    setValidation(null);
    setError("");
    try {
      const { b64, xlsx } = await fileToBase64(file);
      setContent(b64);
      setIsXlsx(xlsx);
      const pv = await PreviewImport(b64, xlsx);
      setHeaders(pv.headers ?? []);
      setRowCount(pv.rowCount ?? 0);
      setMapping(guessMapping(pv.headers ?? []));
    } catch (err) {
      setError(errMsg(err));
    }
  }

  async function run(dryRun: boolean) {
    setBusy(true);
    setError("");
    try {
      const r = await ImportPreconditions(profileId, content, isXlsx, mapping, dryRun, extra.payload());
      // The backend queues nothing while any row has an error, so a real run
      // that found one is shown as a validation, not as a result.
      if (dryRun || r.errors.length > 0) setValidation(r);
      else setResult(r);
    } catch (err) {
      setError(errMsg(err));
    } finally {
      setBusy(false);
    }
  }

  async function downloadTemplate() {
    try {
      const path = await ExportPreconditionImportTemplate();
      if (path) await notice({ title: "Template saved", message: path });
    } catch (err) {
      await notice({ title: "Template export failed", message: errMsg(err), tone: "error" });
    }
  }

  const canRun = headers.length > 0 && mapping.summary !== "" && extra.valid && !busy;

  // Once something was imported, closing the dialog any way at all is Done:
  // the caller refreshes the pending list on onComplete, and the ✕ or Escape
  // skipping it left the imported rows out of that list (#263).
  const close = result ? onComplete : onCancel;

  return (
    <Modal onClose={close} className="modal pending-modal" labelledBy="import-preconditions-title">
      <div className="pending-head">
        <h2 id="import-preconditions-title">Import preconditions (CSV or XLSX)</h2>
        <button className="btn btn-ghost" onClick={close} title="Close" aria-label="Close">
          ✕
        </button>
      </div>

      <div className="bulk-body">
        {result ? (
          <p className="ok-text">
            Imported {plural(result.created, "precondition")} as{" "}
            {result.created === 1 ? "a pending create" : "pending creates"}
            {result.existingCount > 0 && ` (${result.existingCount} already existed, skipped)`}.
            Commit them from the Pending list.
          </p>
        ) : (
          <>
            <div className="import-row">
              <input
                type="file"
                accept=".csv,.xlsx,text/csv"
                aria-label="CSV or XLSX file"
                onChange={pickFile}
              />
              <button className="link-btn" onClick={downloadTemplate}>
                Download template
              </button>
            </div>
            {fileName && (
              <p className="muted">
                {fileName} ({plural(rowCount, "row")})
              </p>
            )}

            {headers.length > 0 && (
              <div className="import-mapping">
                {FIELDS.map((f) => (
                  <label key={f.key} className="bulk-row">
                    <span>
                      {f.label}
                      {f.required ? " *" : ""}
                    </span>
                    <select
                      value={mapping[f.key]}
                      onChange={(e) => setMapping((m) => ({ ...m, [f.key]: e.target.value }))}
                    >
                      <option value="">(not mapped)</option>
                      {headers.map((h) => (
                        <option key={h} value={h}>
                          {h}
                        </option>
                      ))}
                    </select>
                  </label>
                ))}
              </div>
            )}

            {extra.fields.length > 0 && (
              <>
                <p className="muted">These fields are required by Jira and apply to every row.</p>
                <CreateFieldInputs fields={extra.fields} values={extra.values} onChange={extra.setValue} />
              </>
            )}

            {validation && (
              <div className="import-validation">
                <p className={validation.errors.length ? "warn-text" : "ok-text"}>
                  {validation.newCount} new, {validation.existingCount} already{" "}
                  {validation.existingCount === 1 ? "exists" : "exist"}.
                  {validation.errors.length > 0 &&
                    ` ${plural(validation.errors.length, "row")} need fixing. Nothing was imported.`}
                </p>
                {validation.errors.length > 0 && (
                  <ul className="commit-fail-list">
                    {validation.errors.slice(0, 20).map((er) => (
                      <li key={er.row}>{`row ${er.row}: ${er.message}`}</li>
                    ))}
                  </ul>
                )}
              </div>
            )}

            {error && <div className="error-text">{error}</div>}
          </>
        )}
      </div>

      <div className="pending-actions">
        {result ? (
          <button className="btn btn-primary" onClick={onComplete}>
            Done
          </button>
        ) : (
          <>
            <button className="btn" onClick={onCancel} disabled={busy}>
              Cancel
            </button>
            <button className="btn" onClick={() => run(true)} disabled={!canRun}>
              Validate
            </button>
            <button className="btn btn-primary" onClick={() => run(false)} disabled={!canRun}>
              {busy ? "Working…" : "Import"}
            </button>
          </>
        )}
      </div>
    </Modal>
  );
}
