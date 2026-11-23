import { useEffect, useState } from "react";
import { ProgressBar } from "@agile-suite/core";

import { EventsOn } from "../api";
import type { SyncProgress } from "../api";

// useCommitProgress follows the backend's "commit:progress" events while a
// commit runs. fetched counts the pending rows dealt with so far, total the
// rows the commit set out to push, and stage the key being pushed. The final
// done event fills the bar, so it ends full before the result replaces it.
function useCommitProgress(committing: boolean): SyncProgress | null {
  const [progress, setProgress] = useState<SyncProgress | null>(null);
  useEffect(() => {
    if (!committing) return;
    const stop = EventsOn("commit:progress", (p: SyncProgress) => {
      if (p.done) setProgress((prev) => (prev ? { ...prev, fetched: prev.total, stage: "" } : prev));
      else setProgress(p);
    });
    return () => {
      stop();
      setProgress(null);
    };
  }, [committing]);
  return progress;
}

// CommitProgress is the bar the pending changes dialog shows while a commit
// runs: done of total, and the key going out now.
export function CommitProgress({ committing }: { committing: boolean }) {
  const progress = useCommitProgress(committing);
  if (!committing) return null;
  const done = progress?.fetched ?? 0;
  const total = progress?.total ?? 0;
  const count = `${done} of ${total} done`;
  const text = total === 0 ? "Starting the commit" : progress?.stage ? `Pushing ${progress.stage}: ${count}` : count;
  return (
    <div className="commit-progress">
      <ProgressBar value={done} max={total} label="Commit progress" valueText={text} tone="progress" />
      <p className="muted commit-progress-text">{text}</p>
    </div>
  );
}
