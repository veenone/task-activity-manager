import { useState } from "react";
import type { FormEvent } from "react";
import { ProgressBar, call, errMsg, useNotice } from "@agile-suite/core";
import { CheckWorkDuration } from "../api";
import type { LoggedWork, WorklogRow } from "../api";
import { useDiscardById, useLogWork } from "../queries/pending";
import { workDuration } from "../lib/format";
import { dayLabel, dayState, entryDay, nextStart, seconds } from "../lib/worklogWeek";
import type { WorklogCell } from "./WorklogGrid";
import { PERM_WORK_ON_ISSUES } from "../api/permissions";
import { useDenial } from "../queries/permissions";

interface Props {
  profileId: string;
  // rows is the issues work can be logged on: the ones TAM holds.
  rows: WorklogRow[];
  cell: WorklogCell;
  entries: LoggedWork[];
  target: number;
  today: string;
  // nextGap is the nearest other workday still under the target, or "" when
  // there is none.
  nextGap: string;
  onSelect: (cell: WorklogCell) => void;
  onClose: () => void;
}

// PRESETS are the amounts the form offers in one press, as Jira reads them.
const PRESETS = ["30m", "1h", "2h", "4h"];

// WorklogDayPanel is the Log work form for one issue on one day, with the
// day it is being logged into beside it: how the day stands against the
// target and what else the reader logged on it. It is the same form the
// issue panel carries, so an entry made here is journalled the same way and
// reaches Jira on Commit.
export function WorklogDayPanel({ profileId, rows, cell, entries, target, today, nextGap, onSelect, onClose }: Props) {
  const logWork = useLogWork(profileId);
  const denial = useDenial(profileId);
  const discard = useDiscardById(profileId);
  const { notice } = useNotice();
  const [timeSpent, setTimeSpent] = useState("");
  const [comment, setComment] = useState("");
  const [error, setError] = useState("");
  // A start the reader typed holds only while the offer it replaced stands.
  // Another day, or another entry on this one, makes a new offer, and the
  // field goes back to following it.
  const [typedStart, setTypedStart] = useState<{ offer: string; value: string } | null>(null);

  const row = rows.find((r) => r.issue.key === cell.key) ?? rows[0];
  const dayEntries = entries.filter((e) => entryDay(e) === cell.day);
  const logged = seconds(dayEntries);
  const state = dayState(logged, target, cell.day, today, workDuration);
  const offer = nextStart(dayEntries, cell.day);
  const started = typedStart?.offer === offer ? typedStart.value : offer;
  const missing = target - logged;
  const denied = denial(PERM_WORK_ON_ISSUES, row?.issue.key);

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

  async function log(thenNext: boolean) {
    if (!row || !(await check())) return;
    try {
      await logWork.mutateAsync({ key: row.issue.key, timeSpent, comment, started });
      setTimeSpent("");
      setComment("");
      setTypedStart(null);
      if (thenNext && nextGap) onSelect({ key: row.issue.key, day: nextGap });
    } catch (e) {
      setError(errMsg(e));
    }
  }

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    void log(false);
  }

  return (
    <aside className="detail-panel" aria-label={`Log work on ${dayLabel(cell.day)}`}>
      <div className="detail-head">
        <div className="detail-head-id">
          <h2 className="detail-key">{row ? row.issue.key : dayLabel(cell.day)}</h2>
          {row?.issue.status && <span className="detail-head-status">{row.issue.status}</span>}
        </div>
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close" title="Close">✕</button>
      </div>
      <div className="detail-body">
        {row && <p className="detail-summary">{row.issue.summary}</p>}

        <h3 className="detail-section-title worklog-panel-day">
          {dayLabel(cell.day)}
          <span className={`detail-section-note worklog-state-${state.tone}`}>{state.text}</span>
        </h3>
        <ProgressBar
          value={logged}
          max={target}
          label={`Logged on ${dayLabel(cell.day)}`}
          valueText={state.text}
          tone={state.tone === "complete" ? "complete" : logged > 0 ? "progress" : "muted"}
        />
        <p className="muted small worklog-panel-total">{`${workDuration(logged)} logged of ${workDuration(target)}`}</p>

        {!row ? (
          <p className="muted">There is no issue to log on. Sync, or pick a week with an issue TAM holds.</p>
        ) : (
          <form className="add-worklog" onSubmit={onSubmit} aria-labelledby="worklog-panel-form">
            <h3 id="worklog-panel-form">Log work</h3>
            <label className="edit-row" htmlFor="worklog-panel-issue">
              <span className="muted small">Issue</span>
              <select
                id="worklog-panel-issue"
                className="detail-input"
                value={row.issue.key}
                onChange={(e) => onSelect({ key: e.target.value, day: cell.day })}
              >
                {rows.map((r) => (
                  <option key={r.issue.key} value={r.issue.key}>{`${r.issue.key} ${r.issue.summary}`}</option>
                ))}
              </select>
            </label>
            <label className="edit-row" htmlFor="worklog-panel-time">
              <span className="muted small">Time spent</span>
              <input
                id="worklog-panel-time"
                className="detail-input"
                type="text"
                value={timeSpent}
                placeholder="2h 30m, 90m or 1d"
                onChange={(e) => { setTimeSpent(e.target.value); setError(""); }}
                onBlur={() => { if (timeSpent.trim()) void check(); }}
              />
            </label>
            <div className="worklog-presets" role="group" aria-label="Common amounts">
              {PRESETS.map((p) => (
                <button key={p} type="button" className="btn worklog-preset" onClick={() => { setTimeSpent(p); setError(""); }}>{p}</button>
              ))}
              {missing > 0 && (
                <button
                  type="button"
                  className="btn worklog-preset worklog-preset-fill"
                  onClick={() => { setTimeSpent(workDuration(missing)); setError(""); }}
                >
                  {`Fill the day, ${workDuration(missing)}`}
                </button>
              )}
            </div>
            <label className="edit-row" htmlFor="worklog-panel-started">
              <span className="muted small">Started</span>
              <input
                id="worklog-panel-started"
                className="detail-input"
                type="datetime-local"
                value={started}
                onChange={(e) => { setTypedStart({ offer, value: e.target.value }); setError(""); }}
              />
            </label>
            <label className="edit-row" htmlFor="worklog-panel-comment">
              <span className="muted small">Comment</span>
              <input
                id="worklog-panel-comment"
                className="detail-input"
                type="text"
                value={comment}
                placeholder="Optional"
                onChange={(e) => setComment(e.target.value)}
              />
            </label>
            {error && <p className="error-text small" role="alert">{error}</p>}
            <div className="edit-actions">
              <button type="submit" className="btn btn-primary" disabled={!timeSpent.trim() || logWork.isPending || !!denied} title={denied || undefined}>Log work</button>
              {nextGap && (
                <button type="button" className="btn" disabled={!timeSpent.trim() || logWork.isPending || !!denied} title={denied || undefined} onClick={() => void log(true)}>
                  Log and go to next gap
                </button>
              )}
            </div>
            <span className="muted small">{denied || "Journaled now, pushed on Commit."}</span>
          </form>
        )}

        <h3 className="detail-section-title">
          Your entries that day <span className="detail-section-count">{dayEntries.length}</span>
        </h3>
        {dayEntries.length === 0 ? (
          <p className="muted">Nothing logged on this day.</p>
        ) : (
          <ul className="linked-list">
            {dayEntries.map((e) => (
              <li key={e.log.pendingId ? `pending-${e.log.pendingId}` : `${e.key}-${e.log.id}-${e.log.started}`} className="linked-row">
                <span className="linked-key">{e.log.timeSpent}</span>
                <span className="mono">{e.key}</span>
                <span className="worklog-entry-comment">{e.log.comment}</span>
                <span className="muted small">{e.log.started.slice(11, 16)}</span>
                {e.log.pending && (
                  <>
                    <span className="pending-dot" role="img" aria-label="Pending changes" />
                    <button
                      type="button"
                      className="btn btn-discard btn-discard-row"
                      aria-label={`Discard the ${e.log.timeSpent} entry on ${e.key}`}
                      disabled={discard.isPending}
                      onClick={() =>
                        discard.mutate(
                          { id: e.log.pendingId ?? 0, key: e.key },
                          { onError: (err) => void notice({ title: "Discard failed", message: errMsg(err), tone: "error" }) },
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
        )}
      </div>
    </aside>
  );
}
