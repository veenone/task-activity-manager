import { useId, useState } from "react";
import type { ChangeEvent } from "react";
import { Modal } from "@agile-suite/core";
import { useProfile } from "../../contexts/ProfileContext";
import {
  CheckComponents,
  CreateComponents,
  DownloadComponentTemplate,
  ParseComponentWorkbook,
  errMsg,
} from "../../api";
import type {
  ComponentBatchResult,
  ComponentBatchRow,
  ComponentInput,
} from "../../api";
import { fileToBase64 } from "../../files";
import { assigneeLabel } from "./assigneeTypes";

interface Props {
  // Called once a batch has run, whatever it created, so the list reloads.
  onCreated: () => void;
  onClose: () => void;
}

function plural(n: number, word: string) {
  return `${n} ${word}${n === 1 ? "" : "s"}`;
}

function nameOnly(name: string): ComponentInput {
  return { name, description: "", leadUserName: "", assigneeType: "" };
}

// BulkCreateComponentsModal creates several components in one go, from names
// typed one per line or from an XLSX workbook in the template's columns. Every
// row is checked before anything reaches Jira, and the preview says what is
// wrong with the rows that will be skipped. Jira can still refuse a row the
// check passed (an unknown lead, say); the other rows are created regardless.
export function BulkCreateComponentsModal({ onCreated, onClose }: Props) {
  const { activeId: profileId } = useProfile();
  const titleId = useId();
  const [names, setNames] = useState("");
  const [rows, setRows] = useState<ComponentBatchRow[] | null>(null);
  const [result, setResult] = useState<ComponentBatchResult | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState("");

  const readyRows = (rows ?? []).filter((r) => !r.problem);

  async function run(work: () => Promise<void>) {
    setBusy(true);
    setError("");
    try {
      await work();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  }

  const previewNames = () =>
    run(async () => {
      const inputs = names
        .split("\n")
        .map((n) => n.trim())
        .filter(Boolean)
        .map(nameOnly);
      setRows(await CheckComponents(profileId, inputs));
    });

  async function loadWorkbook(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    await run(async () => {
      const { b64 } = await fileToBase64(file);
      const inputs = await ParseComponentWorkbook(b64);
      setRows(await CheckComponents(profileId, inputs));
    });
  }

  const downloadTemplate = () =>
    run(async () => {
      const path = await DownloadComponentTemplate();
      if (path) setSaved(path);
    });

  const create = () =>
    run(async () => {
      try {
        setResult(
          await CreateComponents(
            profileId,
            readyRows.map((r) => r.input),
          ),
        );
      } finally {
        onCreated();
      }
    });

  return (
    <Modal onClose={onClose} className="modal pending-modal" labelledBy={titleId}>
      <div className="pending-head">
        <h2 id={titleId}>Create components</h2>
        <button className="btn btn-ghost" onClick={onClose} title="Close" aria-label="Close">
          ✕
        </button>
      </div>

      <div className="bulk-body">
        {result ? (
          <>
            <p className="ok-text">
              Created {plural(result.created.length, "component")}.
            </p>
            {result.failed.length > 0 && (
              <>
                <p>Jira refused {plural(result.failed.length, "row")}:</p>
                <ul>
                  {result.failed.map((f) => (
                    <li key={f.row} className="error-text">
                      {`${f.name}: ${f.error}`}
                    </li>
                  ))}
                </ul>
              </>
            )}
          </>
        ) : rows ? (
          <>
            <p className="muted">
              {plural(readyRows.length, "component")} ready
              {rows.length > readyRows.length &&
                `, ${plural(rows.length - readyRows.length, "row")} will be skipped`}
              .
            </p>
            <div className="import-validation">
              <table aria-label="Components to create" className="board-table">
                <thead>
                  <tr>
                    <th scope="col">Name</th>
                    <th scope="col">Description</th>
                    <th scope="col">Lead</th>
                    <th scope="col">Default assignee</th>
                    <th scope="col">Check</th>
                  </tr>
                </thead>
                <tbody>
                  {rows.map((r, i) => (
                    <tr key={i}>
                      <td>{r.input.name || <em>(blank)</em>}</td>
                      <td>{r.input.description}</td>
                      <td>{r.input.leadUserName}</td>
                      <td>{assigneeLabel(r.input.assigneeType)}</td>
                      <td>
                        {r.problem ? (
                          <span className="error-text">{r.problem}</span>
                        ) : (
                          <span className="ok-text">Ready</span>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        ) : (
          <>
            <label className="bulk-row">
              <span>Names, one per line</span>
              <textarea
                rows={8}
                value={names}
                onChange={(e) => setNames(e.target.value)}
              />
            </label>
            <p className="muted">
              Typed names get no description, no lead and the project default
              assignee. To set those, fill in the template and load it instead.
            </p>
            <div className="import-row">
              <input
                type="file"
                accept=".xlsx"
                aria-label="XLSX workbook"
                onChange={loadWorkbook}
                disabled={busy}
              />
              <button
                className="link-btn"
                onClick={downloadTemplate}
                disabled={busy}
              >
                Download template
              </button>
            </div>
            {saved && <p className="muted">Template saved to {saved}</p>}
          </>
        )}
        {error && <div className="error-text">{error}</div>}
      </div>

      <div className="pending-actions">
        {result ? (
          <button className="btn btn-primary" onClick={onClose}>
            Done
          </button>
        ) : rows ? (
          <>
            <button className="btn" onClick={() => setRows(null)} disabled={busy}>
              Back
            </button>
            <button
              className="btn btn-primary"
              onClick={create}
              disabled={busy || readyRows.length === 0}
            >
              {busy
                ? "Creating…"
                : `Create ${plural(readyRows.length, "component")}`}
            </button>
          </>
        ) : (
          <>
            <button className="btn" onClick={onClose} disabled={busy}>
              Cancel
            </button>
            <button
              className="btn btn-primary"
              onClick={previewNames}
              disabled={busy || !names.trim()}
            >
              Preview
            </button>
          </>
        )}
      </div>
    </Modal>
  );
}
