import { useState } from "react";
import { usePanelFocus } from "@agile-suite/core";
import type { DependencyLink, LinkType } from "../api";

interface Props {
  link: DependencyLink;
  types: LinkType[];
  busy: boolean;
  // denied is why the token may not link these issues, empty when it may.
  denied: string;
  onRemove: () => void;
  onRetype: (type: string) => void;
  onDiscard: () => void;
  onClose: () => void;
}

// LinkPanel is the side panel for a selected link in the Dependencies view
// (#612), in the detail panel's frame. A cached link can be removed or given
// another type; a pending one can only be discarded, from here or from
// Pending changes. Every edit is journaled and reaches Jira on Commit.
export function LinkPanel({ link, types, busy, denied, onRemove, onRetype, onDiscard, onClose }: Props) {
  const panelRef = usePanelFocus<HTMLElement>(onClose, `${link.outward}>${link.inward}`);
  const wording = types.find((t) => t.name === link.type)?.outward ?? link.type.toLowerCase();
  const [type, setType] = useState("");
  const others = types.filter((t) => t.name !== link.type);
  const chosen = type || others[0]?.name || "";
  const blocked = busy || !!denied;

  return (
    <aside ref={panelRef} className="detail-panel" aria-labelledby="link-panel-title">
      <div className="detail-head">
        <div className="detail-head-id">
          <h2 id="link-panel-title" className="detail-key">{link.type}</h2>
          {link.pending && <span className="detail-head-status">{link.removal ? "Removal pending" : "Pending"}</span>}
        </div>
        <div className="detail-head-actions">
          <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close" title="Close">✕</button>
        </div>
      </div>
      <div className="detail-body link-panel-body">
        <p>{`${link.outward} ${wording} ${link.inward}`}</p>
        {link.pending ? (
          <>
            <p className="muted small">
              {link.removal ? "Commit deletes this link in Jira." : "Commit creates this link in Jira."}
            </p>
            <div className="edit-actions">
              <button type="button" className="btn btn-discard" disabled={busy} onClick={onDiscard}>
                {link.removal ? "Keep the link" : "Discard the link"}
              </button>
            </div>
          </>
        ) : !link.id ? (
          <p className="muted small">This link was cached before TAM stored link ids. Sync once, then it can be removed or changed.</p>
        ) : (
          <>
            <div className="edit-actions">
              <button type="button" className="btn btn-danger" disabled={blocked} title={denied || undefined} onClick={onRemove}>
                Remove link
              </button>
            </div>
            <label className="edit-row" htmlFor="link-retype">
              <span className="muted small">Change type to</span>
              <select id="link-retype" className="detail-input" value={chosen} onChange={(e) => setType(e.target.value)} disabled={others.length === 0}>
                {others.map((t) => (
                  <option key={t.name} value={t.name}>{t.name}</option>
                ))}
              </select>
            </label>
            <div className="edit-actions">
              <button type="button" className="btn" disabled={blocked || !chosen} title={denied || undefined} onClick={() => onRetype(chosen)}>
                Change type
              </button>
              <span className="muted small">{denied || "Journaled now, pushed on Commit."}</span>
            </div>
          </>
        )}
      </div>
    </aside>
  );
}
