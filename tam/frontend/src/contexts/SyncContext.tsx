import { createContext, useCallback, useContext, useEffect, useMemo, useReducer, useRef, useState } from "react";
import type { ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  syncReducer,
  initialSyncState,
  canSync as canSyncSel,
  canCommit as canCommitSel,
  canSwitchProfile as canSwitchProfileSel,
  useNotice,
  useProfile,
  call,
  errMsg,
} from "@agile-suite/core";
import type { SyncProgress, SyncStatus } from "@agile-suite/core";
import { CommitPendingChanges, EventsOn, SyncBoards, SyncIssues } from "../api";
import type { BoardSummary, CommitResult, Profile, Settings } from "../api";
import { invalidateProfileData, invalidateWrites } from "../queries/invalidate";

// SyncProvider owns the one reducer that keeps sync and commit from
// overlapping. Both actions gate on the selectors before dispatching and the
// reducer refuses a start in any state but idle, so a double click or a
// keyboard repeat cannot start a second run.

const PROGRESS_EVENT = "tam:sync-progress";

interface SyncApi {
  status: SyncStatus;
  progress: SyncProgress | null;
  syncError: string;
  canSync: boolean;
  canCommit: boolean;
  canSwitchProfile: boolean;
  runSync: (full: boolean) => Promise<void>;
  // runBoardsRefresh is the Boards view's own Refresh. It holds the same
  // lock a sync does, because Go holds the same per-profile lock for both:
  // a refresh outside this reducer left the shell offering Sync while the
  // backend was bound to refuse it. It rejects on failure so the caller's
  // mutation still sees the error.
  runBoardsRefresh: () => Promise<BoardSummary>;
  // runSprintCeremony runs a start or a completion under the same lock. Both
  // bound methods take Go's per-profile lock the moment they are called, so
  // a ceremony started while a sync runs would be refused by the backend on a
  // shell that still offered Sync. It rejects rather than swallowing: the
  // dialog is what reports a ceremony's failure, word for word.
  runSprintCeremony: <T>(action: () => Promise<T>) => Promise<T>;
  // runQuietLock is what a fast management write (creating, renaming, or
  // destroying a sprint) takes Go's per-profile lock through without reading
  // as a ceremony. It guards against overlapping a sync, a commit, a boards
  // refresh, or a ceremony the same synchronous way runSprintCeremony does,
  // so two locked calls from this client can never race each other, but it
  // dispatches neither SYNC_START nor SYNC_END: status, canSync, and the
  // progress banner never move for it, because a create is not something
  // every other view needs to announce. Go's own lock still refuses the call
  // outright when another operation already holds it there, and that
  // refusal reaches the caller as an ordinary rejected promise.
  runQuietLock: <T>(action: () => Promise<T>) => Promise<T>;
  // runCommit resolves to the result, or null when nothing ran or the call
  // failed (the failure is shown as a notice).
  runCommit: () => Promise<CommitResult | null>;
  // lastCommit is the most recent result for the active profile, for the
  // Pending changes dialog's banner. It clears when the profile changes.
  lastCommit: CommitResult | null;
  // dismissConflict drops one held issue from lastCommit once it has been
  // resolved, so the dialog shows it as an ordinary group (override) or
  // not at all (keep remote).
  dismissConflict: (key: string) => void;
  // lastBoards is the boards pass of the most recent sync for the active
  // profile, null when that sync had none. The Boards view reports on it:
  // a board the pass had to skip is otherwise never mentioned, since most
  // users never press the board's own Refresh. lastBoardsAt stamps it, so
  // the view can tell which of the two passes ran last.
  lastBoards: BoardSummary | null;
  lastBoardsAt: number;
}

const SyncContext = createContext<SyncApi | null>(null);

export function useSync(): SyncApi {
  const ctx = useContext(SyncContext);
  if (!ctx) {
    throw new Error("useSync must be used within a SyncProvider");
  }
  return ctx;
}

export function SyncProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(syncReducer, initialSyncState);
  const { activeId } = useProfile<Profile, Settings>();
  const qc = useQueryClient();
  const { notice } = useNotice();
  const statusRef = useRef<SyncStatus>("idle");
  const [lastCommit, setLastCommit] = useState<CommitResult | null>(null);
  const [boards, setBoards] = useState<{ summary: BoardSummary | null; at: number }>({ summary: null, at: 0 });

  useEffect(
    () =>
      EventsOn(PROGRESS_EVENT, (p: SyncProgress) =>
        dispatch({ type: "SYNC_PROGRESS", progress: p }),
      ),
    [],
  );

  useEffect(() => {
    setLastCommit(null);
    setBoards({ summary: null, at: 0 });
  }, [activeId]);

  const runSync = useCallback(
    async (full: boolean) => {
      if (!activeId || statusRef.current !== "idle") return;
      statusRef.current = "syncing";
      dispatch({
        type: "SYNC_START",
        clearError: true,
        initialProgress: { phase: "issues", fetched: 0, total: 0, done: false, stage: "Starting" },
      });
      try {
        const sum = await call(() => SyncIssues(activeId, full));
        setBoards({ summary: sum.boards ?? null, at: Date.now() });
      } catch (e) {
        const message = errMsg(e);
        dispatch({ type: "SYNC_ERROR", message });
        void notice({ title: "Sync failed", message, tone: "error" });
      } finally {
        statusRef.current = "idle";
        dispatch({ type: "SYNC_END" });
        invalidateProfileData(qc, activeId);
      }
    },
    [activeId, qc, notice],
  );

  const runBoardsRefresh = useCallback(async (): Promise<BoardSummary> => {
    if (!activeId) throw new Error("no profile selected");
    if (statusRef.current !== "idle") {
      throw new Error("a sync is already running for this profile");
    }
    statusRef.current = "syncing";
    dispatch({
      type: "SYNC_START",
      clearError: true,
      initialProgress: { phase: "boards", fetched: 0, total: 0, done: false, stage: "Refreshing boards" },
    });
    try {
      const sum = await call(() => SyncBoards(activeId));
      setBoards({ summary: sum, at: Date.now() });
      return sum;
    } finally {
      statusRef.current = "idle";
      dispatch({ type: "SYNC_END" });
    }
  }, [activeId]);

  // A ceremony is short, one Jira call or a handful, so it reports a stage
  // rather than a count: there is nothing to page through and no total to
  // fill in. It emits no progress frames of its own, exactly as the boards
  // refresh does not.
  const runSprintCeremony = useCallback(async <T,>(action: () => Promise<T>): Promise<T> => {
    if (!activeId) throw new Error("no profile selected");
    if (statusRef.current !== "idle") {
      throw new Error("a sync is already running for this profile");
    }
    statusRef.current = "syncing";
    dispatch({
      type: "SYNC_START",
      clearError: true,
      initialProgress: { phase: "sprint", fetched: 0, total: 0, done: false, stage: "Talking to Jira" },
    });
    try {
      return await action();
    } finally {
      statusRef.current = "idle";
      dispatch({ type: "SYNC_END" });
    }
  }, [activeId]);

  // A quiet lock is the same statusRef guard every other run function
  // checks, so it still refuses while any of them is in flight, but it never
  // touches the reducer: no SYNC_START, no SYNC_END, no progress frame. The
  // global banner and canSync are what they would be if the call were not
  // running at all, which is the point, since a create is fast and local to
  // its own dialog.
  const runQuietLock = useCallback(async <T,>(action: () => Promise<T>): Promise<T> => {
    if (!activeId) throw new Error("no profile selected");
    if (statusRef.current !== "idle") {
      throw new Error("a sync is already running for this profile");
    }
    statusRef.current = "syncing";
    try {
      return await action();
    } finally {
      statusRef.current = "idle";
    }
  }, [activeId]);

  const runCommit = useCallback(async (): Promise<CommitResult | null> => {
    if (!activeId || statusRef.current !== "idle") return null;
    statusRef.current = "committing";
    dispatch({ type: "COMMIT_START" });
    try {
      const res = await call(() => CommitPendingChanges(activeId));
      setLastCommit(res);
      // A Commit that moved cards ends with a boards sync, because a
      // board's membership is whatever Jira's board endpoint last
      // answered with: without it a card that was just pushed jumps back
      // to where the previous sync saw it. It is best effort, and its
      // summary goes where every other boards pass's does, so a board it
      // had to skip is still reported.
      if ((res.moved ?? []).length > 0) {
        try {
          setBoards({ summary: await call(() => SyncBoards(activeId)), at: Date.now() });
        } catch {
          // The commit itself landed. A failed refresh leaves the board
          // showing the local move, which is what it showed a moment ago.
        }
      }
      return res;
    } catch (e) {
      void notice({ title: "Commit failed", message: errMsg(e), tone: "error" });
      return null;
    } finally {
      statusRef.current = "idle";
      dispatch({ type: "COMMIT_END" });
      invalidateWrites(qc, activeId);
      invalidateProfileData(qc, activeId);
    }
  }, [activeId, qc, notice]);

  const dismissConflict = useCallback((key: string) => {
    setLastCommit((cur) => (cur ? { ...cur, conflicts: cur.conflicts.filter((c) => c.key !== key) } : cur));
  }, []);

  const api = useMemo<SyncApi>(
    () => ({
      status: state.status,
      progress: state.progress,
      syncError: state.syncError,
      canSync: canSyncSel(state) && !!activeId,
      canCommit: canCommitSel(state) && !!activeId,
      canSwitchProfile: canSwitchProfileSel(state),
      runSync,
      runBoardsRefresh,
      runSprintCeremony,
      runQuietLock,
      runCommit,
      lastCommit,
      dismissConflict,
      lastBoards: boards.summary,
      lastBoardsAt: boards.at,
    }),
    [state, activeId, runSync, runBoardsRefresh, runSprintCeremony, runQuietLock, runCommit, lastCommit, dismissConflict, boards],
  );

  return <SyncContext.Provider value={api}>{children}</SyncContext.Provider>;
}
