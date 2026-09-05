import { useEffect, useState } from "react";
import { Modal } from "@agile-suite/core";
import { GetDiagnostics } from "../api";
import type { Diagnostics } from "../api";

export function AboutModal({ onClose }: { onClose: () => void }) {
  const [d, setD] = useState<Diagnostics | null>(null);
  useEffect(() => {
    GetDiagnostics().then(setD).catch(() => setD(null));
  }, []);
  return (
    <Modal onClose={onClose} labelledBy="about-title">
      <div className="pending-head">
        <h2 id="about-title">About Task Activity Manager</h2>
      </div>
      <div className="bulk-body">
        <p>Agile task management for Jira Data Center. Part of the agile suite with Xray Test Manager.</p>
        {d && (
          <dl className="about-list">
            <dt>Version</dt><dd>{d.version || "dev"}</dd>
            <dt>Local store</dt><dd>{d.dbPath} (schema {d.schemaVersion})</dd>
            <dt>Shared profiles</dt><dd>{d.sharedPath}</dd>
            <dt>Log</dt><dd>{d.logPath}</dd>
            <dt>Runtime</dt><dd>{d.goVersion} on {d.os}/{d.arch}</dd>
          </dl>
        )}
        <div className="form-actions form-actions-end">
          <button className="btn btn-primary" onClick={onClose}>Close</button>
        </div>
      </div>
    </Modal>
  );
}
