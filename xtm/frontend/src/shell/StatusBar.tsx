import type { SyncProgress } from "../api";
import type { AppState } from "./useAppState";

// StatusBar shows sync progress while a sync runs, then its error or the
// synced test count and when it last ran.
export function StatusBar({ s }: { s: AppState }) {
  const { progress, syncError, syncState } = s;
  // A running sync supersedes the last one's failure, so the bar reports what
  // is happening now. This mirrors the branch below because the ribbon colours
  // the whole bar, which only the footer can do (#313).
  const failed = !!syncError && !(progress && !progress.done);
  return (
    <footer className={failed ? "app-statusbar statusbar-failed" : "app-statusbar"}>
      {progress && !progress.done ? (
        <SyncBar progress={progress} />
      ) : failed ? (
        <div className="sync-ribbon" role="alert">
          <span className="sync-ribbon-mark" aria-hidden="true">
            !
          </span>
          <span>
            <strong>Sync failed.</strong> {syncError}
          </span>
        </div>
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
