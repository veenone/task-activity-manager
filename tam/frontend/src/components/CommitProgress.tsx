import { useEffect, useState } from "react";
import { ProgressBar } from "@agile-suite/core";
import { COMMIT_PROGRESS_EVENT, EventsOn } from "../api";
import type { CommitProgress as Frame } from "../api";
import { useSync } from "../contexts/SyncContext";
import { plural } from "../lib/format";

// commitStageLine says what a frame is pushing: the phase, and the issue
// when the phase goes one issue at a time.
export function commitStageLine(frame: Frame | null): string {
  if (!frame) return "Starting the commit";
  if (frame.finished) return "Sent, finishing up";
  return frame.key ? `Committing ${frame.stage}, ${frame.key}` : `Committing ${frame.stage}`;
}

// CommitProgress is the Pending changes dialog's bar while a Commit runs.
// It listens for as long as the dialog is open, so the first frame cannot
// arrive before the listener does, and starts each Commit from empty.
export function CommitProgress() {
  const { status } = useSync();
  const committing = status === "committing";
  const [frame, setFrame] = useState<Frame | null>(null);

  useEffect(() => EventsOn(COMMIT_PROGRESS_EVENT, (p: Frame) => setFrame(p)), []);
  useEffect(() => {
    if (committing) setFrame(null);
  }, [committing]);

  if (!committing) return null;
  const done = frame?.done ?? 0;
  const total = frame?.total ?? 0;
  const count = `${done} of ${plural(total, "change", "changes")}`;
  return (
    <div className="commit-progress">
      <ProgressBar value={done} max={total} label="Commit progress" valueText={count} tone="progress" />
      <div className="commit-progress-line muted">
        <span>{commitStageLine(frame)}</span>
        <span>{count}</span>
      </div>
    </div>
  );
}
