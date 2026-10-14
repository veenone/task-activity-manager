import { useCallback, useEffect, useState } from "react";
import { Modal } from "../Modal";
import { errMsg } from "../../lib/errMsg";
import type { DiagnosticsInfo, InfoRow } from "./diagnostics";

interface Props {
  // The app's short name, used where the export's contents are described.
  appName: string;
  getDiagnostics: () => Promise<DiagnosticsInfo>;
  readLog: () => Promise<string>;
  // Writes the bundle and returns where it went.
  exportDiagnostics: () => Promise<string>;
  // Rows the app adds after the paths, e.g. TAM's shared profile store.
  extraRows?: (d: DiagnosticsInfo) => InfoRow[];
  onClose: () => void;
}

// DiagnosticsDialog shows the environment readout and the tail of the app's
// log, and exports both to a file a user can attach to a ticket.
//
// The log is read apart from the readout: a log the app cannot read says
// nothing about the paths and the build, which are the rest of the dialog.
// Its three states are spelled out (unreadable, still reading, empty), since
// a missing log and an unreadable one mean different things.
export function DiagnosticsDialog({ appName, getDiagnostics, readLog, exportDiagnostics, extraRows, onClose }: Props) {
  const [diag, setDiag] = useState<DiagnosticsInfo | null>(null);
  const [log, setLog] = useState("");
  const [logError, setLogError] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [exported, setExported] = useState("");

  const load = useCallback(() => {
    setLoading(true);
    setError("");
    setLogError("");
    getDiagnostics()
      .then(setDiag)
      .catch((e) => setError(errMsg(e)));
    readLog()
      .then(setLog)
      .catch((e) => {
        setLog("");
        setLogError(errMsg(e));
      })
      .finally(() => setLoading(false));
  }, [getDiagnostics, readLog]);

  useEffect(() => load(), [load]);

  async function exportFile() {
    setError("");
    try {
      setExported(await exportDiagnostics());
    } catch (e) {
      setExported("");
      setError(errMsg(e));
    }
  }

  const rows: InfoRow[] = [
    ["Version", diag ? `v${diag.version}` : "…"],
    ["Schema", diag ? `v${diag.schemaVersion}` : "…"],
    ["Runtime", diag ? `${diag.goVersion} · ${diag.os}/${diag.arch}` : "…"],
    ["Database", diag?.dbPath || "not known", true],
    ["Log file", diag?.logPath || "not known", true],
    ...(diag && extraRows ? extraRows(diag) : []),
  ];
  if (diag?.startupError) rows.push(["Startup error", diag.startupError]);

  const logBody = logError
    ? `The log could not be read: ${logError}`
    : loading
      ? "Reading the log…"
      : log || "The log is empty.";

  return (
    <Modal onClose={onClose} className="modal pending-modal" labelledBy="diagnostics-title">
      <div className="pending-head">
        <h2 id="diagnostics-title">Diagnostics</h2>
        <button type="button" className="btn btn-ghost" onClick={onClose} aria-label="Close">
          ✕
        </button>
      </div>

      <div className="bulk-body">
        {error && <p className="error-text">{error}</p>}

        <dl className="diag-fields" role="group" aria-label="Build and environment">
          {rows.map(([label, value, isPath]) => (
            <div className="diag-row" key={label}>
              <dt>{label}</dt>
              <dd className={isPath ? "mono diag-path" : undefined} title={isPath ? value : undefined}>
                {value}
              </dd>
            </div>
          ))}
        </dl>

        {exported && (
          <p className="muted">
            Saved to <span className="mono">{exported}</span>
          </p>
        )}

        <h3 className="diag-log-head">Recent log</h3>
        <pre className="diag-log">{logBody}</pre>
      </div>

      <div className="pending-actions">
        <span className="muted small">
          The export carries the rows above and the recent log, which names the Jira items and people {appName}{" "}
          has worked on. Read it before attaching it to a ticket.
        </span>
        <span className="pending-footer-buttons">
          <button type="button" className="btn" onClick={load} disabled={loading}>
            Refresh
          </button>
          <button type="button" className="btn btn-primary" onClick={() => void exportFile()}>
            Export
          </button>
        </span>
      </div>
    </Modal>
  );
}
