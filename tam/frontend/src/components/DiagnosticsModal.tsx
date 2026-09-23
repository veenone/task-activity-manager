import { useCallback, useEffect, useState } from "react";
import { Modal, errMsg } from "@agile-suite/core";
import { ExportDiagnostics, GetDiagnostics, ReadLog } from "../api";
import type { Diagnostics } from "../api";

// DiagnosticsModal is XTM's dialog of the same name, in TAM's chrome: the
// environment readout, the tail of tam.log, and an export a user can attach
// to a ticket. The status bar used to carry two of these file names, which is
// not something a user acts on, and this is where they belong instead (#69).
//
// It differs from XTM's in three places. TAM has two databases, so the shared
// profile store is a row of its own. The log is read through a binding that
// takes no arguments, so the dialog has no say in which file it gets. And the
// log's three failure states are spelled out rather than shown as "(empty)",
// because a missing log and an unreadable one mean different things.
export function DiagnosticsModal({ onClose }: { onClose: () => void }) {
  const [diag, setDiag] = useState<Diagnostics | null>(null);
  const [log, setLog] = useState("");
  const [logError, setLogError] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [exported, setExported] = useState("");

  // The log is read apart from the summary: a log TAM cannot read says
  // nothing about the paths and the build, which are the rest of the dialog.
  const load = useCallback(() => {
    setLoading(true);
    setError("");
    setLogError("");
    GetDiagnostics()
      .then(setDiag)
      .catch((e) => setError(errMsg(e)));
    ReadLog()
      .then(setLog)
      .catch((e) => {
        setLog("");
        setLogError(errMsg(e));
      })
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => load(), [load]);

  async function exportFile() {
    setError("");
    try {
      setExported(await ExportDiagnostics());
    } catch (e) {
      setExported("");
      setError(errMsg(e));
    }
  }

  const rows: Array<[string, string, boolean?]> = [
    ["Version", diag ? `v${diag.version}` : "…"],
    ["Schema", diag ? `v${diag.schemaVersion}` : "…"],
    ["Runtime", diag ? `${diag.goVersion} · ${diag.os}/${diag.arch}` : "…"],
    ["Database", diag?.dbPath || "—", true],
    ["Shared profiles", diag?.sharedPath || "—", true],
    ["Log file", diag?.logPath || "—", true],
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
          The export carries the rows above and the recent log, which names the issues, boards and people TAM
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
