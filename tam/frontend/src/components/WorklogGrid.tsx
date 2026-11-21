import type { KeyboardEvent } from "react";
import { ProgressBar, filteredEmpty } from "@agile-suite/core";
import type { LoggedWork, WorklogRow } from "../api";
import { workDuration } from "../lib/format";
import { JiraStatus } from "@agile-suite/core";
import { cellEntries, dayLabel, dayState, dayTotals, isWeekend, seconds } from "../lib/worklogWeek";
import { TypeChip } from "./TypeChip";

export interface WorklogCell {
  key: string;
  day: string;
}

interface Props {
  days: string[];
  today: string;
  rows: WorklogRow[];
  // filtered says the text filter hid every row the week has.
  filtered: boolean;
  // entries is everything the reader logged in the days shown, on every
  // issue. The column totals count all of it, whatever the filter hides.
  entries: LoggedWork[];
  // target is the seconds a workday is measured against.
  target: number;
  selected: WorklogCell | null;
  onSelect: (cell: WorklogCell) => void;
}

const STEP: Record<string, [number, number]> = {
  ArrowUp: [-1, 0],
  ArrowDown: [1, 0],
  ArrowLeft: [0, -1],
  ArrowRight: [0, 1],
};

// WorklogGrid is the week: issues down the side, days across the top, and
// the reader's own time where they meet. Each cell of an issue TAM holds is
// a button that opens the form for that issue and day.
export function WorklogGrid({ days, today, rows, filtered, entries, target, selected, onSelect }: Props) {
  const totals = dayTotals(entries);
  const mine = rows.filter((r) => r.assigned);
  const others = rows.filter((r) => !r.assigned);

  // The arrow keys walk the cells the way they walk a spreadsheet. A row
  // with no buttons (an issue the cache does not hold) is stepped over
  // rather than stopped at.
  function onKeyDown(e: KeyboardEvent<HTMLButtonElement>) {
    const step = STEP[e.key];
    const from = e.currentTarget;
    const table = from.closest("table");
    if (!step || !table) return;
    let row = Number(from.dataset.row);
    const col = Number(from.dataset.col) + step[1];
    for (;;) {
      row += step[0];
      if (row < 0 || row >= rows.length || col < 0 || col >= days.length) return;
      const next = table.querySelector<HTMLElement>(`[data-row="${row}"][data-col="${col}"]`);
      if (next) {
        e.preventDefault();
        next.focus();
        return;
      }
      if (step[0] === 0) return;
    }
  }

  function renderRow(r: WorklogRow, index: number) {
    const { issue } = r;
    const rowEntries = entries.filter((e) => e.key === issue.key);
    const isSelected = selected?.key === issue.key;
    return (
      <tr key={issue.key} className={isSelected ? "worklog-row-selected" : undefined}>
        <th scope="row" className="worklog-issue">
          <span className="worklog-issue-line">
            <span className="mono worklog-key">{issue.key}</span>
            {issue.type && <TypeChip type={issue.type} />}
            {issue.status && (
              <JiraStatus status={issue.status} category={issue.statusCategory} />
            )}
          </span>
          <span className="worklog-summary" title={issue.summary}>{issue.summary}</span>
          {!r.cached && <span className="muted small">TAM does not hold this issue, so it is read only here</span>}
        </th>
        {days.map((day, col) => {
          const cell = cellEntries(rowEntries, issue.key, day);
          const logged = seconds(cell);
          const pending = cell.some((e) => e.log.pending);
          const cls = `worklog-cell${day === today ? " worklog-today" : ""}${isWeekend(day) ? " worklog-weekend" : ""}`;
          if (!r.cached) {
            return <td key={day} className={cls}>{logged > 0 ? workDuration(logged) : ""}</td>;
          }
          return (
            <td key={day} className={cls}>
              <button
                type="button"
                className="worklog-cell-btn"
                data-row={index}
                data-col={col}
                aria-pressed={isSelected && selected?.day === day}
                aria-label={`${issue.key}, ${dayLabel(day)}, ${logged > 0 ? `${workDuration(logged)} logged` : "nothing logged"}`}
                onKeyDown={onKeyDown}
                onClick={() => onSelect({ key: issue.key, day })}
              >
                {logged > 0 ? workDuration(logged) : <span className="worklog-add" aria-hidden="true">+</span>}
                {pending && <span className="pending-dot" role="img" aria-label="Pending changes" />}
              </button>
            </td>
          );
        })}
        <td className="worklog-sum">{workDuration(seconds(rowEntries))}</td>
      </tr>
    );
  }

  return (
    <div className="backlog-grid">
      <div className="worklog-scroll">
        <table className="worklog-table" aria-label="Your logged time by issue and day">
          <thead>
            <tr>
              <th scope="col" className="worklog-issue-head">Issue</th>
              {days.map((day) => {
                const logged = totals.get(day) ?? 0;
                const weekend = isWeekend(day);
                const state = dayState(logged, target, day, today, workDuration);
                return (
                  <th
                    key={day}
                    scope="col"
                    className={`worklog-day${day === today ? " worklog-today" : ""}${weekend ? " worklog-weekend" : ""}`}
                  >
                    <span className="worklog-day-name">{dayLabel(day)}</span>
                    {day === today && <span className="muted small">today</span>}
                    {weekend ? (
                      logged > 0 && <span className="worklog-day-state">{workDuration(logged)}</span>
                    ) : (
                      <>
                        <ProgressBar
                          value={logged}
                          max={target}
                          label={`Logged on ${dayLabel(day)}`}
                          valueText={state.text}
                          tone={state.tone === "complete" ? "complete" : logged > 0 ? "progress" : "muted"}
                        />
                        <span className={`worklog-day-state worklog-state-${state.tone}`}>{state.text}</span>
                      </>
                    )}
                  </th>
                );
              })}
              <th scope="col" className="worklog-sum">Week</th>
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 && (
              <tr>
                <td colSpan={days.length + 2} className="muted worklog-empty">
                  {filtered
                    ? filteredEmpty("issues")
                    : "No open issue is assigned to you, and nothing was logged in these days."}
                </td>
              </tr>
            )}
            {mine.length > 0 && (
              <tr className="worklog-group">
                <th colSpan={days.length + 2} scope="colgroup">
                  Assigned to me <span className="detail-section-count">{mine.length}</span>
                </th>
              </tr>
            )}
            {mine.map((r, i) => renderRow(r, i))}
            {others.length > 0 && (
              <tr className="worklog-group">
                <th colSpan={days.length + 2} scope="colgroup">
                  Also logged in these days <span className="detail-section-count">{others.length}</span>
                </th>
              </tr>
            )}
            {others.map((r, i) => renderRow(r, mine.length + i))}
          </tbody>
        </table>
      </div>
      <p className="worklog-foot muted small">
        Arrow keys move between cells and Enter opens the form. The hours are yours only.
      </p>
    </div>
  );
}
