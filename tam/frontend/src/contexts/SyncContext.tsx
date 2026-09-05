import { createContext, useCallback, useContext, useEffect, useMemo, useReducer, useRef } from "react";
import type { ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  syncReducer,
  initialSyncState,
  canSync as canSyncSel,
  canSwitchProfile as canSwitchProfileSel,
  useNotice,
  useProfile,
  call,
  errMsg,
} from "@agile-suite/core";
import type { SyncProgress, SyncStatus } from "@agile-suite/core";
import { EventsOn, SyncIssues } from "../api";
import type { Profile, Settings } from "../api";
import { invalidateProfileData } from "../queries/invalidate";

// SyncContext owns TAM's sync lifecycle on the shared reducer: it subscribes
// to the progress event, runs the bound SyncIssues call, refreshes the
// profile's queries afterwards, and reports failures with a notice. Unlike
// XTM's provider it also owns the orchestration, because TAM has no commit
// path yet and the call is one line.

const PROGRESS_EVENT = "tam:sync-progress";

interface SyncApi {
  status: SyncStatus;
  progress: SyncProgress | null;
  syncError: string;
  canSync: boolean;
  canSwitchProfile: boolean;
  // runSync pulls the active profile's issues; full clears and refetches.
  runSync: (full: boolean) => Promise<void>;
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
  // The reducer refuses a second SYNC_START, but the bound call must not run
  // twice either, and two clicks in one tick both see the pre-dispatch state.
  // So the guard lives in the ref and runSync claims it, rather than the ref
  // trailing the reducer a render behind.
  const statusRef = useRef<SyncStatus>("idle");

  useEffect(
    () =>
      EventsOn(PROGRESS_EVENT, (p: SyncProgress) =>
        dispatch({ type: "SYNC_PROGRESS", progress: p }),
      ),
    [],
  );

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

  const api = useMemo<SyncApi>(
    () => ({
      status: state.status,
      progress: state.progress,
      syncError: state.syncError,
      canSync: canSyncSel(state) && !!activeId,
      canSwitchProfile: canSwitchProfileSel(state),
      runSync,
    }),
    [state, activeId, runSync],
  );

  return <SyncContext.Provider value={api}>{children}</SyncContext.Provider>;
}
