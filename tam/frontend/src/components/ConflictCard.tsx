import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { call, errMsg } from "@agile-suite/core";
import { ResolveConflictKeepRemote, ResolveConflictOverride, fieldLabel } from "../api";
import type { Conflict } from "../api";
import { invalidateWrites } from "../queries/invalidate";
import { useSync } from "../contexts/SyncContext";

interface Props {
  profileId: string;
  conflict: Conflict;
  disabled: boolean;
}

// ConflictCard is a held issue inside the Pending changes dialog: the
// three-way table and the two resolutions. Override rebases the edits so
// the next Commit pushes them; Keep remote drops them and takes Jira's row.
export function ConflictCard({ profileId, conflict, disabled }: Props) {
  const qc = useQueryClient();
  const { dismissConflict } = useSync();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function resolve(how: "override" | "keep") {
    setBusy(true);
    setError("");
    try {
      if (how === "override") {
        await call(() => ResolveConflictOverride(profileId, conflict.key, conflict.remoteVersion));
      } else {
        await call(() => ResolveConflictKeepRemote(profileId, conflict.key));
      }
      dismissConflict(conflict.key);
      invalidateWrites(qc, profileId, conflict.key);
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="pending-card pending-card-conflict" role="group" aria-label={conflict.key}>
      <div className="pending-card-head">
        <span className="b">{conflict.key}</span>
        <span className="pending-card-summary">{conflict.summary}</span>
        <span className="chip chip-conflict">Conflict</span>
      </div>
      <table className="conflict-table">
        <thead>
          <tr><th scope="col">Field</th><th scope="col">Base</th><th scope="col">Mine</th><th scope="col">Remote</th></tr>
        </thead>
        <tbody>
          {conflict.fields.map((f) => (
            <tr key={f.field}>
              <td>{fieldLabel(f.field)}</td>{" "}
              <td>{f.base || "(none)"}</td>{" "}
              <td className="b">{f.mine || "(none)"}</td>{" "}
              <td className={f.remote !== f.base ? "danger-text" : ""}>{f.remote || "(none)"}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <p className="muted small">Base is the value when you edited. Remote is Jira now. Override pushes Mine over Remote; Keep remote drops your edits.</p>
      <div className="edit-actions">
        <button type="button" className="btn btn-primary" disabled={disabled || busy} onClick={() => void resolve("override")}>Override</button>
        <button type="button" className="btn" disabled={disabled || busy} onClick={() => void resolve("keep")}>Keep remote</button>
        {error && <span className="error-text small" role="alert">{error}</span>}
      </div>
    </section>
  );
}
