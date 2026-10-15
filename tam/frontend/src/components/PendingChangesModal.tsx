import { useMemo } from "react";
import { Modal, errMsg, useConfirm, useNotice, useProfile } from "@agile-suite/core";
import type { Profile, Settings } from "../api";
import { groupPending, useDiscardAll, useDiscardById, useDiscardChange, usePendingChanges, useUnpushableEdits } from "../queries/pending";
import type { PendingGroup } from "../queries/pending";
import { useSync } from "../contexts/SyncContext";
import { plural } from "../lib/format";
import { CommitBanner } from "./CommitBanner";
import { ConflictCard } from "./ConflictCard";
import { PendingGroupRows } from "./PendingGroupRows";

export { sprintChangeLine, sprintDraftLine } from "./pendingWords";

interface Props {
  onClose: () => void;
}

// summaryLine is the dialog's subtitle: "3 changes on 2 issues, 1 of them
// new", with boards and sprints counted apart, since neither is an issue.
export function summaryLine(groups: PendingGroup[], rowCount: number): string {
  const issues = groups.filter((g) => g.kind === "issue");
  const boards = groups.filter((g) => g.kind === "board").length;
  const newCount = groups.filter((g) => g.sprintRow).length;
  const changedCount = groups.filter((g) => g.kind === "sprint" && !g.sprintRow).length;
  const changes = plural(rowCount, "change", "changes");
  const others: string[] = [];
  if (boards > 0) others.push(plural(boards, "new board", "new boards"));
  if (newCount > 0) others.push(plural(newCount, "new sprint", "new sprints"));
  if (changedCount > 0) others.push(plural(changedCount, "sprint change", "sprint changes"));
  if (issues.length === 0) return `${changes}: ${others.join(", ")}`;
  const drafts = issues.filter((g) => g.createRow).length;
  let line = `${changes} on ${plural(issues.length, "issue", "issues")}`;
  if (drafts > 0) line += `, ${drafts} of them new`;
  if (others.length > 0) line += `, and ${others.join(", ")}`;
  return line;
}

// countPushable is the number on the Commit button: the groups Commit will
// actually deliver. Two things take a group out of it.
//
// A conflict holds its issue back, which it always has. And an issue
// carrying a journal row Jira will refuse delivers nothing at all: an
// edit's fields go in one update, so one refused field fails the whole of
// it, and counting that issue would promise something Commit cannot do.
// Nothing here changes what a push does, and nothing hides the work: the
// card and its Discard stay exactly as they are, this is only the number.
//
// refusedKeys comes from the same rows the note on the row is drawn from,
// so the number and the explanation beside it can never disagree. It only
// ever holds issues whose edit screen TAM has actually read, which is what
// keeps the empty-is-unknown rule here too: an issue nothing is known about
// is absent from it and stays in the count, because it may well push.
//
// Only an issue group is matched by issue key. A sprint group's key is its
// numeric id and a draft board's is a negative one, and neither is an issue
// key Jira could refuse a field on.
export function countPushable(
  groups: PendingGroup[],
  conflictKeys: Set<string>,
  refusedKeys: Set<string>,
): number {
  return groups.filter((g) => !conflictKeys.has(g.key) && !(g.kind === "issue" && refusedKeys.has(g.key))).length;
}


export function PendingChangesModal({ onClose }: Props) {
  const { activeId, activeProfile } = useProfile<Profile, Settings>();
  const pending = usePendingChanges(activeId);
  const unpushable = useUnpushableEdits(activeId);
  const discardOne = useDiscardChange(activeId);
  const discardRow = useDiscardById(activeId);
  const discardAll = useDiscardAll(activeId);
  const { confirm } = useConfirm();
  const { notice } = useNotice();
  const { canCommit, runCommit, lastCommit, status } = useSync();

  const rows = pending.data ?? [];
  const groups = useMemo(() => groupPending(rows), [rows]);
  const conflictKeys = new Set((lastCommit?.conflicts ?? []).map((c) => c.key).filter((k) => groups.some((g) => g.key === k)));

  // What the last Commit held, by key: a row is held when something it names
  // was not created, and the card says what that is.
  const heldByKey = useMemo(() => {
    const byKey = new Map<string, string[]>();
    for (const h of lastCommit?.held ?? []) {
      const reasons = byKey.get(h.key) ?? [];
      if (!reasons.includes(h.reason)) reasons.push(h.reason);
      byKey.set(h.key, reasons);
    }
    return byKey;
  }, [lastCommit]);

  // The journal rows Jira's own edit screens say a Commit would refuse,
  // by row id. They are named on their row and kept: the value is what the
  // user typed, and only the user throws it away.
  const unpushableIds = new Set((unpushable.data ?? []).map((e) => e.id));
  // The same rows by issue key, which is what the Commit count is short by.
  const refusedKeys = new Set((unpushable.data ?? []).map((e) => e.key));
  const refusedCount = groups.filter((g) => g.kind === "issue" && refusedKeys.has(g.key)).length;

  const pushable = countPushable(groups, conflictKeys, refusedKeys);

  // A held-back issue sits above the table: it is what blocks a clean
  // commit, so it belongs where the eye lands first. The rest keep
  // groupPending's order.
  const heldGroups = groups.filter((g) => conflictKeys.has(g.key));
  const tableGroups = groups.filter((g) => !conflictKeys.has(g.key));
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
        <h2 id="pending-title">{`Pending changes (${rows.length})`}</h2>
        <span className="muted">{groups.length ? summaryLine(groups, rows.length) : ""}</span>
        <button type="button" className="btn btn-ghost" onClick={onClose} title="Close" aria-label="Close">✕</button>
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
          <>
            {/* XTM shows its conflicts above its table too. */}
            {heldGroups.map((g) => {
              const conflict = lastCommit!.conflicts.find((c) => c.key === g.key)!;
              return <ConflictCard key={g.id} profileId={activeId} conflict={conflict} disabled={busy} />;
            })}
            {tableGroups.length > 0 && (
              <div className="pending-table-wrap">
                <table aria-label="Pending changes" className="pending-table">
                  <thead>
                    <tr>
                      <th scope="col">Issue</th>
                      <th scope="col">Field</th>
                      <th scope="col">Before</th>
                      <th scope="col">After</th>
                      <th scope="col"><span className="sr-only">Actions</span></th>
                    </tr>
                  </thead>
                  {tableGroups.map((g) => (
                    <PendingGroupRows
                      key={g.id}
                      group={g}
                      project={activeProfile?.projectKey ?? ""}
                      heldReasons={heldByKey.get(g.key) ?? []}
                      unpushableIds={unpushableIds}
                      busy={busy}
                      onDiscard={(row) => discardOne.mutate(row, { onError: onDiscardError })}
                    />
                  ))}
                </table>
              </div>
            )}
          </>
        )}
      </div>

      <div className="pending-actions">
        <p className="muted pending-footnote-inline">
          Edits are pushed with Jira's own field update, a move with the transition, sprint, and rank endpoints. A conflict holds only that issue back.
          {refusedCount > 0 && ` ${plural(refusedCount, "issue is", "issues are")} not in that count: a field Jira will not take fails that issue's whole update. Discard the row, or ask a Jira administrator to add the field to the edit screen.`}
        </p>
        <button type="button" className="btn btn-danger" disabled={busy || rows.length === 0} onClick={() => void onDiscardAll()} title="Revert every pending change">
          Discard all
        </button>
        <button type="button" className="btn btn-primary" disabled={!canCommit || busy || pushable === 0} onClick={() => void runCommit()}>
          {`Commit (${pushable})`}
        </button>
      </div>
    </Modal>
  );
}
