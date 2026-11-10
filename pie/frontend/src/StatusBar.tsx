import { createContext, useEffect, useState } from "react";
import { ProgressBar } from "@agile-suite/core";
import { EventsOn } from "../wailsjs/runtime/runtime";

// A Notice is a sentence for the status bar's ribbon, such as where an export
// was saved. error colours the bar, as a failed sync does in TAM and XTM.
export type Notice = { text: string; error?: boolean };

// Progress is Go's query.Progress, sent as an event rather than returned, so the
// generated models do not carry it.
export type Progress = { stage: string; done: number; total: number };

// NoticeContext is how a view puts a Notice in the status bar without App
// passing a callback down to every Export button (#452).
export const NoticeContext = createContext<(n: Notice) => void>(
  () => undefined,
);

// useRunProgress follows Go's run:progress events while running, and forgets
// the last one when the run ends.
export function useRunProgress(running: boolean): Progress | null {
  const [progress, setProgress] = useState<Progress | null>(null);
  useEffect(() => {
    if (!running) {
      setProgress(null);
      return;
    }
    return EventsOn("run:progress", (p: Progress) => setProgress(p));
  }, [running]);
  return progress;
}

// StatusBar is the suite's footer: the profile and the last run on the left,
// the theme on the right. A running query shows its progress in their place,
// and a Notice turns the bar into a ribbon until it is dismissed.
export function StatusBar({
  profile,
  summary,
  runAt,
  progress,
  notice,
  onDismiss,
  theme,
}: {
  profile?: string;
  summary?: string;
  runAt?: string;
  progress: Progress | null;
  notice: Notice | null;
  onDismiss: () => void;
  theme: string;
}) {
  // A run's progress wins over a notice nobody dismissed (#452 review).
  const shown = progress ? null : notice;
  return (
    <footer
      className={
        shown?.error ? "app-statusbar statusbar-failed" : "app-statusbar"
      }
    >
      {shown ? (
        <div className="pie-ribbon" role={shown.error ? "alert" : "status"}>
          <span>{shown.text}</span>
          <button
            type="button"
            className="btn btn-ghost pie-ribbon-close"
            onClick={onDismiss}
          >
            Dismiss
          </button>
        </div>
      ) : (
        <>
          {profile && <span>{profile}</span>}
          {progress ? (
            <span className="pie-run-progress">
              <ProgressBar
                value={progress.done}
                max={progress.total}
                label="Run progress"
                valueText={`${progress.done} of ${progress.total}`}
                tone="progress"
              />
              <span className="muted">{`${progress.stage}: ${progress.done} / ${progress.total}`}</span>
            </span>
          ) : (
            <>
              {summary && <span className="muted">{summary}</span>}
              {runAt && <span className="muted">{runAt}</span>}
            </>
          )}
          <span className="muted statusbar-right">{`Theme: ${theme}`}</span>
        </>
      )}
    </footer>
  );
}
