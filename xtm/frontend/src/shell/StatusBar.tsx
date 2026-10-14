import type { SyncProgress } from "../api";
import type { AppState } from "./useAppState";

// StatusBar shows sync progress while a sync runs, then its error or the
// synced test count and when it last ran.
export function StatusBar({ s }: { s: AppState }) {
  const { progress, syncError, syncState } = s;
  return (
    <footer className="app-statusbar">
      {progress && !progress.done ? (
        <SyncBar progress={progress} />
      ) : syncError ? (
        <span className="error-text">Sync failed: {syncError}</span>
      ) : syncState ? (
        <span className="muted sync-info">
          {syncState.testCount.toLocaleString()} tests
          {syncState.lastSyncedAt
            ? ` · last synced ${new Date(
                syncState.lastSyncedAt,
              ).toLocaleString()}`
            : " · never synced"}
        </span>
      ) : (
        <span className="muted sync-info">&nbsp;</span>
      )}
    </footer>
  );
}

function SyncBar({ progress }: { progress: SyncProgress }) {
  const hasCount = progress.total > 0;
  const pct = hasCount
    ? Math.round((progress.fetched / progress.total) * 100)
    : 0;
  // Prefer the explicit stage label; fall back to a phase-derived label.
  const stage =
    progress.stage || (progress.phase === "folders" ? "Folders" : "Syncing");
  return (
    <div className="syncbar">
      {hasCount && (
        <div className="syncbar-track">
          <div className="syncbar-fill" style={{ width: `${pct}%` }} />
        </div>
      )}
      <span className="muted">
        {stage}
        {hasCount
          ? `: ${progress.fetched.toLocaleString()} / ${progress.total.toLocaleString()}`
          : "…"}
      </span>
    </div>
  );
}
