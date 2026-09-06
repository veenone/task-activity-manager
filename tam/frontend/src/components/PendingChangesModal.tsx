import { useMemo } from "react";
import { Modal, errMsg, useConfirm, useNotice, useProfile } from "@agile-suite/core";
import { fieldLabel } from "../api";
import type { CommitResult, IssueDraft, Profile, Settings } from "../api";
import { ISSUE_TYPES } from "../api";
import { groupPending, useDiscardAll, useDiscardChange, usePendingChanges } from "../queries/pending";
import type { PendingGroup } from "../queries/pending";
import { useSync } from "../contexts/SyncContext";
import { plural } from "../lib/format";
import { ConflictCard } from "./ConflictCard";

interface Props {
  onClose: () => void;
}

// summaryLine is the dialog's subtitle: "3 changes on 2 issues, 1 of them new".
export function summaryLine(groups: PendingGroup[], rowCount: number): string {
  const drafts = groups.filter((g) => g.createRow).length;
  const base = `${plural(rowCount, "change", "changes")} on ${plural(groups.length, "issue", "issues")}`;
  return drafts > 0 ? `${base}, ${drafts} of them new` : base;
}

// bannerLine renders a commit result as one sentence.
export function bannerLine(r: CommitResult): string {
  const parts: string[] = [];
  if (r.committed.length) parts.push(plural(r.committed.length, "issue pushed", "issues pushed"));
  if (r.created.length) {
    const mapping = r.created.map((c) => `${c.tempKey} is now ${c.key}`).join(", ");
    parts.push(`${r.created.length} created (${mapping})`);
  }
  if (r.linked.length) parts.push(plural(r.linked.length, "link pushed", "links pushed"));
  if (r.conflicts.length) parts.push(`${r.conflicts.length} held back`);
  if (r.failures.length) parts.push(`${r.failures.length} failed`);
  if (parts.length === 0) return "Last commit: nothing to push.";
  if (!r.committed.length && !r.created.length && !r.linked.length) return `Last commit: nothing pushed, ${parts.join(", ")}.`;
  return `Last commit: ${parts.join(", ")}.`;
}

function draftLine(d: IssueDraft, project: string): string {
  const type = ISSUE_TYPES.find((t) => t.id === d.type)?.label ?? d.type;
  const bits = [`New ${type} in ${project}`];
  if (d.priority) bits.push(`priority ${d.priority}`);
  if (d.assignee) bits.push(`assignee ${d.assignee}`);
  if (d.storyPoints !== null && d.storyPoints !== undefined) bits.push(`${d.storyPoints} points`);
  return bits.join(", ");
}

export function PendingChangesModal({ onClose }: Props) {
  const { activeId, activeProfile } = useProfile<Profile, Settings>();
  const pending = usePendingChanges(activeId);
  const discardOne = useDiscardChange(activeId);
  const discardAll = useDiscardAll(activeId);
  const { confirm } = useConfirm();
  const { notice } = useNotice();
  const { canCommit, runCommit, lastCommit, status } = useSync();

  const rows = pending.data ?? [];
  const groups = useMemo(() => groupPending(rows), [rows]);
  const conflictKeys = new Set((lastCommit?.conflicts ?? []).map((c) => c.key).filter((k) => groups.some((g) => g.key === k)));
  const pushable = groups.filter((g) => !conflictKeys.has(g.key)).length;
  const busy = status !== "idle" || discardOne.isPending || discardAll.isPending;

  function onDiscardError(e: unknown) {
    void notice({ title: "Discard failed", message: errMsg(e), tone: "error" });
  }

  async function onDiscardAll() {
    const ok = await confirm({
      title: "Discard all pending changes?",
      message: `${plural(rows.length, "change", "changes")} will be reverted locally. Jira is not touched.`,
      confirmLabel: "Discard all",
      danger: true,
    });
    if (ok) discardAll.mutate(undefined, { onError: onDiscardError });
  }

  return (
    <Modal onClose={onClose} className="modal pending-modal" labelledBy="pending-title">
      <div className="pending-head">
        <h2 id="pending-title">Pending changes</h2>
        <span className="muted">{groups.length ? summaryLine(groups, rows.length) : ""}</span>
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>

      {lastCommit && (
        <div className={`pending-banner${lastCommit.conflicts.length || lastCommit.failures.length ? " pending-banner-warn" : ""}`} role="status">
          <p className="b">{bannerLine(lastCommit)}</p>
          {lastCommit.conflicts.filter((c) => conflictKeys.has(c.key)).map((c) => (
            <p key={c.key} className="small">{c.key} changed in Jira since you edited it. Resolve it below, then commit again.</p>
          ))}
          {lastCommit.failures.map((f) => (
            <p key={f.key} className="small error-text">{f.key}: {f.error}</p>
          ))}
        </div>
      )}

      {pending.isError ? (
        <p className="error-text">Could not load the pending changes: {pending.error.message}</p>
      ) : pending.isPending ? (
        <p className="muted">Loading</p>
      ) : groups.length === 0 ? (
        <p className="muted pending-empty">Nothing pending. Edit an issue or create one and it shows up here.</p>
      ) : (
        <div className="pending-list">
          {groups.map((g) => {
            const conflict = lastCommit?.conflicts.find((c) => c.key === g.key && conflictKeys.has(c.key));
            if (conflict) {
              return <ConflictCard key={g.key} profileId={activeId} conflict={conflict} disabled={busy} />;
            }
            return (
              <section key={g.key} className="pending-card" role="group" aria-label={g.key}>
                <div className="pending-card-head">
                  <span className="b">{g.key}</span>
                  {g.createRow && <span className="chip chip-draft">Draft</span>}
                  {g.draft && <span className="pending-card-summary">{g.draft.summary}</span>}
                  {g.createRow && (
                    <button type="button" className="btn pending-discard" disabled={busy} aria-label={`Discard ${g.key}`} onClick={() => discardOne.mutate(g.createRow!, { onError: onDiscardError })}>
                      Discard
                    </button>
                  )}
                </div>
                {g.draft ? (
                  <>
                    <p className="muted small">{draftLine(g.draft, activeProfile?.projectKey ?? "")}</p>
                    <p className="muted small">Commit creates it in Jira and swaps the temporary key for the real one.</p>
                  </>
                ) : (
                  <ul className="pending-rows">
                    {g.links.map(({ row, link }) => (
                      <li key={row.id} className="pending-row pending-row-link">
                        <span className="muted pending-field">Link</span>{" "}
                        <span className="b">{`${link.type} (${link.direction})`}</span>{" "}
                        <span className="accent-text">{link.toKey}</span>{" "}
                        <span>{link.toSummary}</span>{" "}
                        <button type="button" className="btn btn-ghost" disabled={busy} aria-label={`Discard link to ${link.toKey}`} onClick={() => discardOne.mutate(row, { onError: onDiscardError })}>
                          Discard
                        </button>
                      </li>
                    ))}
                    {g.edits.map((row) => (
                      <li key={row.id} className="pending-row">
                        <span className="muted pending-field">{fieldLabel(row.field)}</span>{" "}
                        <span>{row.beforeVal || "(none)"}</span>{" "}
                        <span className="muted">to</span>{" "}
                        <span className="b">{row.afterVal || "(none)"}</span>{" "}
                        <button type="button" className="btn btn-ghost" disabled={busy} aria-label={`Discard ${row.field} on ${g.key}`} onClick={() => discardOne.mutate(row, { onError: onDiscardError })}>
                          Discard
                        </button>
                      </li>
                    ))}
                  </ul>
                )}
              </section>
            );
          })}
        </div>
      )}

      <div className="pending-footer">
        <span className="muted small">Edits are pushed with Jira's own field update; a conflict holds only that issue back.</span>
        <span className="pending-footer-buttons">
          <button type="button" className="btn" disabled={busy || rows.length === 0} onClick={() => void onDiscardAll()}>Discard all</button>
          <button type="button" className="btn btn-primary" disabled={!canCommit || busy || pushable === 0} onClick={() => void runCommit()}>
            {`Commit (${pushable})`}
          </button>
        </span>
      </div>
    </Modal>
  );
}
