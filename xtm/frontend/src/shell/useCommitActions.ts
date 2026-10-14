import { ResolveConflictOverride, ResolveConflictKeepRemote, ResolveConflictMerge, RecreateDeletedTest, DiscardPendingChange, DiscardAllPendingChanges, CommitPendingChanges, CommitPendingChangesByIDs, errMsg } from "../api";
import type { CommitResult, ConflictDecision } from "../api";
import type { AppState } from "./useAppState";

// useCommitActions holds what the pending dialog does: discard, commit all
// or some, and the four ways to resolve a conflict.
export function useCommitActions(s: AppState) {
  const { activeId, notice, closeModal, syncStatus, canCommit, beginCommit, endCommit, setSelectedKey, setDetailVersion, setLastCommitResult, reloadPending, refreshProfileData } = s;

  // Called when the user discards a pending change from the modal. The
  // backend reverts test_case to before_val, so the detail panel needs to
  // re-fetch too.
  async function handleDiscard(id: number) {
    if (!activeId) return;
    try {
      await DiscardPendingChange(activeId, id);
      refreshProfileData();
      setDetailVersion((v) => v + 1);
      reloadPending();
    } catch (e) {
      console.error("discard:", errMsg(e));
    }
  }

  // handleDiscardAll reverts every pending change at once (the modal's
  // "Discard all" action). The modal confirms first.
  async function handleDiscardAll() {
    if (!activeId) return;
    try {
      await DiscardAllPendingChanges(activeId);
      refreshProfileData();
      setDetailVersion((v) => v + 1);
      reloadPending();
    } catch (e) {
      console.error("discard all:", errMsg(e));
    }
  }

  // applyCreatedRemap re-points an open detail view from a just-committed
  // "NEW-N" placeholder to the real Jira key the backend assigned (FR-1), so the
  // panel shows the actual ticket instead of a key that no longer exists.
  function applyCreatedRemap(result: CommitResult) {
    const created = result.created ?? [];
    if (created.length === 0) return;
    setSelectedKey((cur) => {
      if (!cur) return cur;
      const match = created.find((c) => c.tempKey === cur);
      return match ? match.key : cur;
    });
  }

  // Shared gate for the two commit entry points. Returns false — with a "sync
  // in progress" notice when a sync is what's blocking — if a commit may not
  // start. A commit already running is a silent no-op.
  async function ensureCanCommit(): Promise<boolean> {
    if (!activeId) return false;
    if (!canCommit) {
      if (syncStatus === "syncing") {
        await notice({
          title: "Sync in progress",
          message:
            "A sync is still running. Please wait for it to finish before committing.",
          tone: "info",
        });
      }
      return false;
    }
    return true;
  }

  // Called when the user clicks "Commit" in the pending modal. Pushes all
  // pending changes to Jira; per-Test results land in lastCommitResult.
  // Committed pending rows are deleted by the backend; failures stay.
  async function handleCommit() {
    if (!(await ensureCanCommit())) return;
    beginCommit();
    setLastCommitResult(null);
    try {
      const result = await CommitPendingChanges(activeId);
      setLastCommitResult(result);
      applyCreatedRemap(result);
      refreshProfileData();
      setDetailVersion((v) => v + 1);
      reloadPending();
    } catch (e) {
      setLastCommitResult({
        succeeded: [],
        conflicted: [],
        failed: [{ testKey: "", error: errMsg(e) }],
      });
    } finally {
      endCommit();
    }
  }

  // handleCommitIds commits a selected subset of pending changes (selective
  // commit) — the per-item Commit button in the modal. Same result handling as
  // a full commit; only the chosen item leaves the list on success.
  async function handleCommitIds(ids: number[]) {
    if (ids.length === 0) return;
    if (!(await ensureCanCommit())) return;
    beginCommit();
    setLastCommitResult(null);
    try {
      const result = await CommitPendingChangesByIDs(activeId, ids);
      setLastCommitResult(result);
      applyCreatedRemap(result);
      refreshProfileData();
      setDetailVersion((v) => v + 1);
      reloadPending();
    } catch (e) {
      setLastCommitResult({
        succeeded: [],
        conflicted: [],
        failed: [{ testKey: "", error: errMsg(e) }],
      });
    } finally {
      endCommit();
    }
  }

  // resolveConflictOverride re-bases a conflicting Test onto the remote version
  // (keep mine) and immediately re-commits so the override takes effect.
  async function resolveConflictOverride(
    testKey: string,
    remoteVersion: string,
  ) {
    if (!activeId) return;
    try {
      await ResolveConflictOverride(activeId, testKey, remoteVersion);
    } catch (e) {
      setLastCommitResult({
        succeeded: [],
        conflicted: [],
        failed: [{ testKey, error: errMsg(e) }],
      });
      return;
    }
    await handleCommit();
  }

  // resolveConflictKeepRemote discards a conflicting Test's local edits (keep
  // remote) and refreshes.
  async function resolveConflictKeepRemote(testKey: string) {
    if (!activeId) return;
    try {
      await ResolveConflictKeepRemote(activeId, testKey);
      setLastCommitResult((prev) =>
        prev
          ? {
              ...prev,
              conflicted: prev.conflicted.filter((c) => c.testKey !== testKey),
            }
          : prev,
      );
      refreshProfileData();
      setDetailVersion((v) => v + 1);
      reloadPending();
    } catch (e) {
      console.error("keep remote:", errMsg(e));
    }
  }

  // resolveConflictMerge applies per-field keep-mine / keep-theirs decisions for
  // a conflicting Test, then re-commits so the merge takes effect.
  async function resolveConflictMerge(
    testKey: string,
    remoteVersion: string,
    decisions: ConflictDecision[],
  ) {
    if (!activeId) return;
    try {
      await ResolveConflictMerge(activeId, testKey, remoteVersion, decisions);
    } catch (e) {
      setLastCommitResult({
        succeeded: [],
        conflicted: [],
        failed: [{ testKey, error: errMsg(e) }],
      });
      return;
    }
    await handleCommit();
  }

  // resolveConflictRecreate turns a remotely-deleted Test's held edits into a
  // brand-new local Test, then drops it from the conflict list.
  async function resolveConflictRecreate(testKey: string) {
    if (!activeId) return;
    try {
      await RecreateDeletedTest(activeId, testKey);
      setLastCommitResult((prev) =>
        prev
          ? {
              ...prev,
              conflicted: prev.conflicted.filter((c) => c.testKey !== testKey),
            }
          : prev,
      );
      refreshProfileData();
      setDetailVersion((v) => v + 1);
      reloadPending();
    } catch (e) {
      console.error("recreate deleted test:", errMsg(e));
    }
  }

  function closePendingModal() {
    closeModal();
    setLastCommitResult(null);
  }

  return { handleDiscard, handleDiscardAll, applyCreatedRemap, ensureCanCommit, handleCommit, handleCommitIds, resolveConflictOverride, resolveConflictKeepRemote, resolveConflictMerge, resolveConflictRecreate, closePendingModal };
}
