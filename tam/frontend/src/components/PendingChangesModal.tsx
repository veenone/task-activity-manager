import { useMemo } from "react";
import { Modal, errMsg, useConfirm, useNotice, useProfile } from "@agile-suite/core";
import { fieldLabel } from "../api";
import type { IssueDraft, Profile, Settings } from "../api";
import { ISSUE_TYPES } from "../api";
import { groupPending, useDiscardAll, useDiscardById, useDiscardChange, usePendingChanges } from "../queries/pending";
import type { PendingGroup } from "../queries/pending";
import { useSync } from "../contexts/SyncContext";
import { plural } from "../lib/format";
import { CommitBanner } from "./CommitBanner";
import { ConflictCard } from "./ConflictCard";
import { PendingMoveRow } from "./PendingMoveRow";

interface Props {
  onClose: () => void;
}

// summaryLine is the dialog's subtitle: "3 changes on 2 issues, 1 of them new".
export function summaryLine(groups: PendingGroup[], rowCount: number): string {
  const drafts = groups.filter((g) => g.createRow).length;
  const base = `${plural(rowCount, "change", "changes")} on ${plural(groups.length, "issue", "issues")}`;
  return drafts > 0 ? `${base}, ${drafts} of them new` : base;
}

function draftLine(d: IssueDraft, project: string): string {
  const type = ISSUE_TYPES.find((t) => t.id === d.type)?.label ?? d.type;
  const bits = [`New ${type} in ${project}`];
  if (d.priority) bits.push(`priority ${d.priority}`);
  if (d.assignee) bits.push(`assignee ${d.assignee}`);
  if (d.storyPoints !== null && d.storyPoints !== undefined) bits.push(`${d.storyPoints} points`);
  // A draft dragged into a sprint journals nothing, since Commit sends the
  // draft rather than the move, so the draft's own sprint is the only place
  // that move shows up. Its column cannot be named the same way: a draft
  // carries the status id it was dropped on and no status name, since Jira
  // has never granted it one.
  if (d.sprintName) bits.push(`sprint ${d.sprintName}`);
  return bits.join(", ");
}

export function PendingChangesModal({ onClose }: Props) {
  const { activeId, activeProfile } = useProfile<Profile, Settings>();
  const pending = usePendingChanges(activeId);
  const discardOne = useDiscardChange(activeId);
  const discardRow = useDiscardById(activeId);
  const discardAll = useDiscardAll(activeId);
  const { confirm } = useConfirm();
  const { notice } = useNotice();
  const { canCommit, runCommit, lastCommit, status } = useSync();

  const rows = pending.data ?? [];
  const groups = useMemo(() => groupPending(rows), [rows]);
  const conflictKeys = new Set((lastCommit?.conflicts ?? []).map((c) => c.key).filter((k) => groups.some((g) => g.key === k)));
  const pushable = groups.filter((g) => !conflictKeys.has(g.key)).length;

  // The held-back issue sits above drafts and edits: it is what blocks a
  // clean commit, so it belongs where the eye lands first.
  const orderedGroups = useMemo(
    () => [
      ...groups.filter((g) => conflictKeys.has(g.key)),
      ...groups.filter((g) => !conflictKeys.has(g.key) && g.createRow),
      ...groups.filter((g) => !conflictKeys.has(g.key) && !g.createRow),
    ],
    [groups, conflictKeys],
  );
  const busy = status !== "idle" || discardOne.isPending || discardAll.isPending || discardRow.isPending;

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

      <div className="bulk-body">
        {lastCommit && (
          <CommitBanner
            result={lastCommit}
            heldKeys={conflictKeys}
            pendingRowIds={new Set(rows.map((r) => r.id))}
            busy={busy}
            onUndo={(id, key) => discardRow.mutate({ id, key }, { onError: onDiscardError })}
          />
        )}

        {pending.isError ? (
          <p className="error-text">Could not load the pending changes: {pending.error.message}</p>
        ) : pending.isPending ? (
          <p className="muted">Loading</p>
        ) : groups.length === 0 ? (
          <p className="muted pending-empty">Nothing pending. Edit an issue or create one and it shows up here.</p>
        ) : (
          <div className="pending-list">
            {orderedGroups.map((g) => {
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
                          <span className="muted">Link</span>{" "}
                          <span className="b">{`${link.type} (${link.direction})`}</span>{" "}
                          <span className="accent-text">{link.toKey}</span>{" "}
                          <span>{link.toSummary}</span>{" "}
                          <button type="button" className="btn btn-ghost" disabled={busy} aria-label={`Discard link to ${link.toKey}`} onClick={() => discardOne.mutate(row, { onError: onDiscardError })}>
                            Discard
                          </button>
                        </li>
                      ))}
                      {g.moves.map((row) => (
                        <PendingMoveRow
                          key={row.id}
                          row={row}
                          disabled={busy}
                          onDiscard={() => discardOne.mutate(row, { onError: onDiscardError })}
                        />
                      ))}
                      {g.edits.map((row) => (
                        <li key={row.id} className="pending-row">
                          <span className="muted">{fieldLabel(row.field)}</span>{" "}
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
      </div>

      <div className="pending-actions">
        <span className="muted small">
          Edits are pushed with Jira's own field update, a move with the transition, sprint, and rank endpoints. A conflict holds only that issue back.
        </span>
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
