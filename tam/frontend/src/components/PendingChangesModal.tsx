import { useMemo } from "react";
import { Modal, errMsg, useConfirm, useNotice, useProfile } from "@agile-suite/core";
import { ENTITY_SPRINT_COMPLETE, ENTITY_SPRINT_EDIT, ENTITY_SPRINT_START, fieldLabel } from "../api";
import type { DraftBoard, DraftSprint, IssueDraft, PendingChange, Profile, Settings, SprintComplete, SprintEdit, SprintStart } from "../api";
import { ISSUE_TYPES } from "../api";
import { groupPending, useDiscardAll, useDiscardById, useDiscardChange, usePendingChanges } from "../queries/pending";
import type { PendingGroup } from "../queries/pending";
import { useSync } from "../contexts/SyncContext";
import { dayInput, plural } from "../lib/format";
import { CommitBanner } from "./CommitBanner";
import { ConflictCard } from "./ConflictCard";
import { PendingMoveRow } from "./PendingMoveRow";

interface Props {
  onClose: () => void;
}

// isSprintGroup says a group is a sprint's rather than an issue's.
function isSprintGroup(g: PendingGroup): boolean {
  return !!g.sprintRow || g.sprintChanges.length > 0;
}

// isIssueGroup says a group is an issue's: not a board's, not a sprint's.
function isIssueGroup(g: PendingGroup): boolean {
  return !g.boardRow && !isSprintGroup(g);
}

// summaryLine is the dialog's subtitle: "3 changes on 2 issues, 1 of them
// new", with boards and sprints counted apart, since neither is an issue.
export function summaryLine(groups: PendingGroup[], rowCount: number): string {
  const issues = groups.filter(isIssueGroup);
  const boards = groups.filter((g) => g.boardRow).length;
  const newCount = groups.filter((g) => g.sprintRow).length;
  const changedCount = groups.filter((g) => !g.sprintRow && g.sprintChanges.length > 0).length;
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

// boardDraftLine says what kind of board a draft is and what it collects.
export function boardDraftLine(b: DraftBoard | null): string {
  if (!b) return "A draft board that could not be read. Discard it and draft it again.";
  const type = b.type.charAt(0).toUpperCase() + b.type.slice(1);
  return `${type} board on the filter ${b.filterName}: ${b.jql}`;
}

// sprintChangeName is the sprint's name: an edit's before value, the name
// every other change carries.
function sprintChangeName(row: PendingChange): string {
  try {
    const name = row.entityType === ENTITY_SPRINT_EDIT
      ? (JSON.parse(row.beforeVal) as { name?: string }).name
      : (JSON.parse(row.afterVal) as { name?: string }).name;
    return name || `sprint ${row.entityKey}`;
  } catch {
    return `sprint ${row.entityKey}`;
  }
}

// sprintChangeNote says what Commit does with a sprint change.
function sprintChangeNote(row: PendingChange): string {
  switch (row.entityType) {
    case ENTITY_SPRINT_EDIT: return "Saved locally. Commit sends the change to Jira.";
    case ENTITY_SPRINT_START: return "Commit starts it in Jira.";
    case ENTITY_SPRINT_COMPLETE: return "Commit works out the unfinished cards from Jira again, moves them, and closes the sprint.";
    default: return "Commit deletes it in Jira, and Jira moves its cards to the backlog.";
  }
}

// sprintChangeLine reads a sprint change in words: "Edit sprint Sprint 12:
// name to Sprint 12b, goal removed", "Start sprint Sprint 13, 2026-09-14 to
// 2026-09-28", "Complete sprint Sprint 12, unfinished cards to the backlog".
export function sprintChangeLine(row: PendingChange): string {
  const name = sprintChangeName(row);
  if (row.entityType === ENTITY_SPRINT_START) {
    try {
      const s = JSON.parse(row.afterVal) as SprintStart;
      return `Start sprint ${name}, ${dayInput(s.startDate)} to ${dayInput(s.endDate)}`;
    } catch {
      return `Start sprint ${name}`;
    }
  }
  if (row.entityType === ENTITY_SPRINT_COMPLETE) {
    try {
      const c = JSON.parse(row.afterVal) as SprintComplete;
      return `Complete sprint ${name}, unfinished cards to ${c.moveToName || "the backlog"}`;
    } catch {
      return `Complete sprint ${name}`;
    }
  }
  if (row.entityType !== ENTITY_SPRINT_EDIT) return `Delete sprint ${name}`;
  try {
    const was = JSON.parse(row.beforeVal) as DraftSprint;
    const now = JSON.parse(row.afterVal) as SprintEdit;
    const changed: string[] = [];
    if (now.name !== was.name) changed.push(`name to ${now.name}`);
    if (now.clearGoal) changed.push("goal removed");
    else if (now.goal && now.goal !== was.goal) changed.push(`goal to ${now.goal}`);
    const from = dayInput(now.startDate);
    const to = dayInput(now.endDate);
    if (from !== dayInput(was.startDate) || to !== dayInput(was.endDate)) changed.push(`dates to ${from} to ${to}`);
    return `Edit sprint ${name}: ${changed.length ? changed.join(", ") : "nothing changed"}`;
  } catch {
    return `Edit sprint ${name}: the change could not be read. Discard it and edit the sprint again.`;
  }
}

// sprintDraftLine says where and when a draft sprint runs.
export function sprintDraftLine(s: DraftSprint | null): string {
  if (!s) return "A draft sprint that could not be read. Discard it and draft it again.";
  const board = s.boardName || `board ${s.boardId}`;
  return `New sprint on ${board}, ${dayInput(s.startDate)} to ${dayInput(s.endDate)}`;
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

  const pushable = groups.filter((g) => !conflictKeys.has(g.key)).length;

  // The held-back issue sits above everything: it is what blocks a clean
  // commit, so it belongs where the eye lands first. Draft sprints follow,
  // since Commit creates them before anything that moves into them.
  const orderedGroups = useMemo(
    () => [
      ...groups.filter((g) => isIssueGroup(g) && conflictKeys.has(g.key)),
      ...groups.filter((g) => !isIssueGroup(g)),
      ...groups.filter((g) => !conflictKeys.has(g.key) && isIssueGroup(g) && g.createRow),
      ...groups.filter((g) => !conflictKeys.has(g.key) && isIssueGroup(g) && !g.createRow),
    ],
    [groups, conflictKeys],
  );
  const busy = status !== "idle" || discardOne.isPending || discardAll.isPending || discardRow.isPending;

  function onDiscardError(e: unknown) {
    void notice({ title: "Discard failed", message: errMsg(e), tone: "error" });
  }

  // sprintChangeRows is a sprint's pending edit, start, completion or delete,
  // each in words with its own Discard, on a real sprint's card or a draft's.
  function sprintChangeRows(changes: PendingChange[], name: string) {
    return changes.map((row) => (
      <div key={row.id}>
        <div className="pending-card-head">
          <span className="b">{sprintChangeLine(row)}</span>
          <button type="button" className="btn btn-discard pending-discard" disabled={busy} aria-label={`Discard the change to ${name}`} onClick={() => discardOne.mutate(row, { onError: onDiscardError })}><span className="discard-mark" aria-hidden="true">✕</span>Discard
          </button>
        </div>
        <p className="muted small">{sprintChangeNote(row)}</p>
      </div>
    ));
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
              const conflict = !isIssueGroup(g) ? undefined : lastCommit?.conflicts.find((c) => c.key === g.key && conflictKeys.has(c.key));
              if (conflict) {
                return <ConflictCard key={g.id} profileId={activeId} conflict={conflict} disabled={busy} />;
              }
              if (g.boardRow) {
                const name = g.board?.name ?? "Draft board";
                return (
                  <section key={g.id} className="pending-card" role="group" aria-label={name}>
                    <div className="pending-card-head">
                      <span className="b">{`New board ${name}`}</span>
                      <span className="chip chip-draft">Draft board</span>
                      <button type="button" className="btn btn-discard pending-discard" disabled={busy} aria-label={`Discard ${name}`} onClick={() => discardOne.mutate(g.boardRow!, { onError: onDiscardError })}><span className="discard-mark" aria-hidden="true">✕</span>Discard
                      </button>
                    </div>
                    <p className="muted small">{boardDraftLine(g.board)}</p>
                    <p className="muted small">Commit creates its filter and the board in Jira before anything else.</p>
                  </section>
                );
              }
              if (g.sprintRow) {
                const name = g.sprint?.name ?? "Draft sprint";
                return (
                  <section key={g.id} className="pending-card" role="group" aria-label={name}>
                    <div className="pending-card-head">
                      <span className="b">{name}</span>
                      <span className="chip chip-draft">Draft sprint</span>
                      <button type="button" className="btn btn-discard pending-discard" disabled={busy} aria-label={`Discard ${name}`} onClick={() => discardOne.mutate(g.sprintRow!, { onError: onDiscardError })}><span className="discard-mark" aria-hidden="true">✕</span>Discard
                      </button>
                    </div>
                    <p className="muted small">{sprintDraftLine(g.sprint)}</p>
                    <p className="muted small">Commit creates it in Jira first, then moves its cards into it. Discarding it puts those cards back.</p>
                    {sprintChangeRows(g.sprintChanges, name)}
                  </section>
                );
              }
              if (g.sprintChanges.length > 0) {
                const name = sprintChangeName(g.sprintChanges[0]);
                return (
                  <section key={g.id} className="pending-card" role="group" aria-label={name}>
                    {sprintChangeRows(g.sprintChanges, name)}
                  </section>
                );
              }
              const heldReasons = heldByKey.get(g.key) ?? [];
              return (
                <section key={g.id} className="pending-card" role="group" aria-label={g.key}>
                  <div className="pending-card-head">
                    <span className="b">{g.key}</span>
                    {g.createRow && <span className="chip chip-draft">Draft</span>}
                    {heldReasons.length > 0 && <span className="chip chip-held">Waiting</span>}
                    {g.draft && <span className="pending-card-summary">{g.draft.summary}</span>}
                    {g.createRow && (
                      <button type="button" className="btn btn-discard pending-discard" disabled={busy} aria-label={`Discard ${g.key}`} onClick={() => discardOne.mutate(g.createRow!, { onError: onDiscardError })}><span className="discard-mark" aria-hidden="true">✕</span>Discard
                      </button>
                    )}
                  </div>
                  {heldReasons.map((reason) => (
                    <p key={reason} className="small pending-held">{`${reason.charAt(0).toUpperCase()}${reason.slice(1)}.`}</p>
                  ))}
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
                          <button type="button" className="btn btn-discard btn-discard-row" disabled={busy} aria-label={`Discard link to ${link.toKey}`} onClick={() => discardOne.mutate(row, { onError: onDiscardError })}><span className="discard-mark" aria-hidden="true">✕</span>Discard
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
                          <button type="button" className="btn btn-discard btn-discard-row" disabled={busy} aria-label={`Discard ${row.field} on ${g.key}`} onClick={() => discardOne.mutate(row, { onError: onDiscardError })}><span className="discard-mark" aria-hidden="true">✕</span>Discard
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
          <button type="button" className="btn btn-discard" disabled={busy || rows.length === 0} onClick={() => void onDiscardAll()}>
            <span className="discard-mark" aria-hidden="true">✕</span>Discard all
          </button>
          <button type="button" className="btn btn-primary" disabled={!canCommit || busy || pushable === 0} onClick={() => void runCommit()}>
            {`Commit (${pushable})`}
          </button>
        </span>
      </div>
    </Modal>
  );
}
