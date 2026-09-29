import { useState } from "react";
import type { FormEvent } from "react";
import { call, errMsg, useNotice } from "@agile-suite/core";
import { CheckWorkDuration } from "../api";
import type { Worklog } from "../api";
import { useDiscardById, useLogWork, useWorklogs, worklogTotal } from "../queries/pending";
import { day, plural, workDuration } from "../lib/format";

interface Props {
  profileId: string;
  issueKey: string;
  // open is the section's own state. The read is made when the section is
  // expanded rather than on every sync: Jira's worklogs are Jira's, most rows
  // are never asked about, and the section starts closed.
  open: boolean;
}

// WorkLogSection is the issue's worklog: what Jira holds, what the journal
// holds, the total of both, and the one form that adds an entry. Any issue,
// not only one assigned to the reader, because Jira lets a user log work on
// anything they can see and logging it on a colleague's issue during a
// handover is ordinary.
export function WorkLogSection({ profileId, issueKey, open }: Props) {
  const logs = useWorklogs(profileId, issueKey, open);
  const logWork = useLogWork(profileId);
  const discard = useDiscardById(profileId);
  const { notice } = useNotice();
  const [timeSpent, setTimeSpent] = useState("");
  const [comment, setComment] = useState("");
  const [error, setError] = useState("");

  // The duration is checked by the Go side, which is where the rule lives, so
  // the sentence under the field is the same one a Commit would have brought
  // back from Jira hours later. The check runs when the field is left and
  // again on Add, because a button press can beat a blur.
  async function check(): Promise<boolean> {
    try {
      await call(() => CheckWorkDuration(timeSpent));
      setError("");
      return true;
    } catch (e) {
      setError(errMsg(e));
      return false;
    }
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!(await check())) return;
    try {
      await logWork.mutateAsync({ key: issueKey, timeSpent, comment });
      setTimeSpent("");
      setComment("");
    } catch (err) {
      setError(errMsg(err));
    }
  }

  const entries = logs.data ?? [];
  return (
    <>
      {logs.isPending ? (
        <p className="muted">Loading the work log</p>
      ) : logs.isError ? (
        <p className="error-text" data-testid="worklog-error">
          Could not load the work log: {logs.error.message}{" "}
          <button type="button" className="btn btn-ghost" onClick={() => void logs.refetch()}>Retry</button>
        </p>
      ) : entries.length === 0 ? (
        <p className="muted">No work logged.</p>
      ) : (
        <>
          <p className="worklog-total">
            {`${workDuration(worklogTotal(entries))} logged over ${plural(entries.length, "entry", "entries")}`}
          </p>
          <ul className="linked-list">
            {entries.map((l) => (
              <li key={l.pendingId ? `pending-${l.pendingId}` : `${l.id}-${l.started}`} className="linked-row">
                <span className="linked-key">{l.timeSpent}</span>
                {/* The day comes off the stamp Jira dated the entry by, not
                    from the reader's own clock: the entry belongs to the day
                    the person who logged it was having. */}
                <span className="muted small">{day(l.started)}</span>
                <span>{l.authorName || l.author || "You"}</span>
                <span>{l.comment}</span>
                {l.pending && (
                  <>
                    <span className="pending-dot" role="img" aria-label="Pending changes" />
                    <span className="muted small">pending</span>
                    <button
                      type="button"
                      className="btn btn-discard btn-discard-row"
                      aria-label={`Discard the ${l.timeSpent} entry`}
                      disabled={discard.isPending}
                      onClick={() =>
                        discard.mutate(
                          { id: l.pendingId ?? 0, key: issueKey },
                          { onError: (e) => void notice({ title: "Discard failed", message: errMsg(e), tone: "error" }) },
                        )
                      }
                    >
                      <span className="discard-mark" aria-hidden="true">✕</span>Discard
                    </button>
                  </>
                )}
              </li>
            ))}
          </ul>
        </>
      )}

      <form className="add-worklog" onSubmit={(e) => void onSubmit(e)} aria-labelledby="add-worklog-title">
        <h3 id="add-worklog-title">Log work</h3>
        <label className="edit-row" htmlFor="worklog-time">
          <span className="muted small">Time spent</span>
          <input
            id="worklog-time"
            className="detail-input"
            type="text"
            value={timeSpent}
            placeholder="2h 30m, 90m or 1d"
            onChange={(e) => { setTimeSpent(e.target.value); setError(""); }}
            onBlur={() => { if (timeSpent.trim()) void check(); }}
          />
        </label>
        <label className="edit-row" htmlFor="worklog-comment">
          <span className="muted small">Comment</span>
          <input
            id="worklog-comment"
            className="detail-input"
            type="text"
            value={comment}
            placeholder="Optional"
            onChange={(e) => setComment(e.target.value)}
          />
        </label>
        {error && <p className="error-text small" role="alert">{error}</p>}
        <div className="edit-actions">
          <button type="submit" className="btn btn-primary" disabled={!timeSpent.trim() || logWork.isPending}>Log work</button>
          <span className="muted small">Journaled now, pushed on Commit.</span>
        </div>
      </form>
    </>
  );
}

// worklogEntryLine is the one sentence the Pending changes dialog reads a
// journalled entry by: "2h 30m on 29 Sep, Pairing".
export function worklogEntryLine(log: Worklog): string {
  const when = day(log.started);
  const head = when ? `${log.timeSpent} on ${when}` : log.timeSpent;
  return log.comment ? `${head}, ${log.comment}` : head;
}
