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
import { CommitPendingChanges, EventsOn, SyncIssues } from "../api";
import type { CommitResult, Profile, Settings } from "../api";
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

  useEffect(
    () =>
      EventsOn(PROGRESS_EVENT, (p: SyncProgress) =>
        dispatch({ type: "SYNC_PROGRESS", progress: p }),
      ),
    [],
  );

  useEffect(() => {
    setLastCommit(null);
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
        await call(() => SyncIssues(activeId, full));
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

  const runCommit = useCallback(async (): Promise<CommitResult | null> => {
    if (!activeId || statusRef.current !== "idle") return null;
    statusRef.current = "committing";
    dispatch({ type: "COMMIT_START" });
    try {
      const res = await call(() => CommitPendingChanges(activeId));
      setLastCommit(res);
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
      runCommit,
      lastCommit,
      dismissConflict,
    }),
    [state, activeId, runSync, runCommit, lastCommit, dismissConflict],
  );

  return <SyncContext.Provider value={api}>{children}</SyncContext.Provider>;
}
