import { useMemo, useState } from "react";
import { ProgressBar, useProfile } from "@agile-suite/core";
import { DEFAULT_DAY_HOURS } from "../api";
import type { Profile, Settings } from "../api";
import { useDayHours, useSetDayHours, useWorklogWeek } from "../queries/worklogWeek";
import { plural, workDuration } from "../lib/format";
import { addDays, dayLabel, dayOf, gaps, periodLabel, seconds, weekOf } from "../lib/worklogWeek";
import { WorklogDayPanel } from "./WorklogDayPanel";
import { WorklogGrid } from "./WorklogGrid";
import type { WorklogCell } from "./WorklogGrid";

// LOOKBACK_DAYS is how far behind today the view looks for short days.
const LOOKBACK_DAYS = 14;
// WORKDAYS is the days of a week the target is owed on.
const WORKDAYS = 5;

// WorklogView is a week of the reader's own time: their issues down the
// side, the days across the top, and the Log work form beside the grid. A
// workday that came in under the target stays listed above the grid for a
// fortnight, so yesterday can be filled today.
//
// Nothing here is stored. The entries are read from Jira for the days on
// screen, joined with the ones the journal still holds, and an entry made
// here is journalled like any other and reaches Jira on Commit.
export function WorklogView() {
  const { activeId } = useProfile<Profile, Settings>();
  const today = dayOf(new Date());
  const [anchor, setAnchor] = useState(today);
  const [cell, setCell] = useState<WorklogCell | null>(null);
  const [text, setText] = useState("");
  // What is being typed into Day target, until the field is left. The stored
  // value is written once then, not on every keystroke.
  const [typedHours, setTypedHours] = useState<string | null>(null);
  const days = useMemo(() => weekOf(anchor), [anchor]);
  const week = useWorklogWeek(activeId, days[0], days[6]);
  const behind = useWorklogWeek(activeId, addDays(today, -LOOKBACK_DAYS), addDays(today, -1));
  const hours = useDayHours(activeId).data ?? DEFAULT_DAY_HOURS;
  const setHours = useSetDayHours(activeId);
  const target = hours * 3600;

  if (!activeId) {
    return (
      <section className="backlog" aria-label="Worklog" data-tour="worklog-body">
        <p className="muted backlog-empty">Add a profile to see your work log.</p>
      </section>
    );
  }

  const entries = week.data?.entries ?? [];
  const rows = week.data?.rows ?? [];
  const needle = text.trim().toLowerCase();
  const shown = needle
    ? rows.filter((r) => `${r.issue.key} ${r.issue.summary}`.toLowerCase().includes(needle))
    : rows;
  const toFill = behind.data ? gaps(behind.data.entries, today, target, LOOKBACK_DAYS) : [];
  const logged = seconds(entries);
  const weekTarget = target * WORKDAYS;

  // Picking a day keeps the issue the panel was on, so several days can be
  // filled against one issue without choosing it again each time.
  function pickDay(day: string) {
    setAnchor(day);
    setCell({ key: cell?.key ?? "", day });
  }

  return (
    <section className="backlog worklog-view" aria-label="Worklog" data-tour="worklog-body">
      <div className="filter-bar">
        <button type="button" className="btn" aria-label="Previous week" onClick={() => setAnchor(addDays(days[0], -7))}>‹</button>
        <span className="worklog-period">{periodLabel(days)}</span>
        <button type="button" className="btn" aria-label="Next week" onClick={() => setAnchor(addDays(days[0], 7))}>›</button>
        <button type="button" className="btn" disabled={days.includes(today)} onClick={() => setAnchor(today)}>This week</button>
        <input
          className="detail-input worklog-filter"
          type="search"
          aria-label="Filter issues"
          placeholder="Filter by key or summary"
          value={text}
          onChange={(e) => setText(e.target.value)}
        />
        <label className="worklog-target">
          <span className="muted">Day target</span>
          <input
            className="detail-input"
            type="number"
            min={0.5}
            max={24}
            step={0.5}
            value={typedHours ?? String(hours)}
            onChange={(e) => setTypedHours(e.target.value)}
            onBlur={() => {
              const n = Number(typedHours);
              if (typedHours !== null && n > 0 && n <= 24 && n !== hours) setHours.mutate(n);
              setTypedHours(null);
            }}
          />
          <span className="muted">h</span>
        </label>
        <span className="worklog-meter">
          <span className="muted">Week</span>
          <ProgressBar
            value={logged}
            max={weekTarget}
            label="Logged this week"
            valueText={`${workDuration(logged)} of ${workDuration(weekTarget)}`}
            tone={logged >= weekTarget ? "complete" : "progress"}
          />
          <strong>{workDuration(logged)}</strong>
          <span className="muted">{`of ${workDuration(weekTarget)}`}</span>
        </span>
      </div>

      {toFill.length > 0 && (
        <div className="worklog-gaps" role="group" aria-label="Days to fill">
          <span className="worklog-gaps-title">{`${plural(toFill.length, "workday", "workdays")} to fill`}</span>
          <span className="muted small">{`last ${LOOKBACK_DAYS} days`}</span>
          {toFill.map((g) => (
            <button
              key={g.day}
              type="button"
              className="btn worklog-gap"
              aria-pressed={cell?.day === g.day}
              onClick={() => pickDay(g.day)}
            >
              <strong>{dayLabel(g.day)}</strong>
              {g.logged > 0 ? `${workDuration(target - g.logged)} short` : "nothing logged"}
            </button>
          ))}
        </div>
      )}

      {week.isPending ? (
        <p className="muted backlog-empty">Loading your work log</p>
      ) : week.isError ? (
        <p className="error-text" role="alert">
          Could not load your work log: {week.error.message}{" "}
          <button type="button" className="btn btn-ghost" onClick={() => void week.refetch()}>Retry</button>
        </p>
      ) : (
        <div className="backlog-body">
          <WorklogGrid
            days={days}
            today={today}
            rows={shown}
            filtered={rows.length > 0}
            entries={entries}
            target={target}
            selected={cell}
            onSelect={setCell}
          />
          {cell && (
            <WorklogDayPanel
              profileId={activeId}
              rows={rows.filter((r) => r.cached)}
              cell={cell}
              entries={entries}
              target={target}
              today={today}
              nextGap={toFill.find((g) => g.day !== cell.day)?.day ?? ""}
              onSelect={(next) => { setAnchor(next.day); setCell(next); }}
              onClose={() => setCell(null)}
            />
          )}
        </div>
      )}
    </section>
  );
}
