import { SyncRibbon } from "@agile-suite/core";

import type { SyncProgress } from "../api";
import type { AppState } from "./useAppState";

// StatusBar shows whether the local store came up, sync progress while a sync
// runs, then its error or the synced test count and when it last ran, and the
// theme in force. The health glyph and the theme label are TAM's, carried over
// in #330: both apps read the same bar and both held the state already.
export function StatusBar({ s }: { s: AppState }) {
  const { progress, syncError, syncState, health, theme } = s;
  // A running sync supersedes the last one's failure, so the bar reports what
  // is happening now. This mirrors the branch below because the ribbon colours
  // the whole bar, which only the footer can do (#313).
  const failed = !!syncError && !(progress && !progress.done);
  return (
    <footer className={failed ? "app-statusbar statusbar-failed" : "app-statusbar"}>
      {failed ? (
        <SyncRibbon message={syncError} />
      ) : (
        <>
          {/* The dot is colour and colour alone is not a reading, so the words
              beside it carry the state and the dot is not announced (I3). */}
          <span className={`dot ${health?.ok ? "dot-ok" : "dot-warn"}`} aria-hidden="true" />
          <span>{health?.ok ? "Local store ready" : "Starting up"}</span>
          {progress && !progress.done ? (
            <SyncBar progress={progress} />
          ) : syncState ? (
            <span className="muted sync-info">
              {syncState.testCount.toLocaleString()} tests
              {syncState.lastSyncedAt
                ? ` · last synced ${new Date(
                    syncState.lastSyncedAt,
                  ).toLocaleString()}`
                : " · never synced"}
            </span>
          ) : null}
          <span className="muted statusbar-right">Theme: {theme}</span>
        </>
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
