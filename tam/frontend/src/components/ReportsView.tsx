import { useEffect, useMemo, useState } from "react";
import { call, errMsg, useProfile } from "@agile-suite/core";
import { CancelSprintReport } from "../api";
import type { Profile, Settings, Sprint, SprintReport } from "../api";
import { useBoards, useBoardSprints } from "../queries/boards";
import { useSprintReport } from "../queries/reports";
import { useSync } from "../contexts/SyncContext";
import { busyLine, isBusyRefusal, unavailableLine } from "../lib/reportText";
import { SprintSummary } from "./SprintSummary";
import { VelocityTable } from "./VelocityTable";

// ReportsView is the pickers, the states, and the wiring. The sentence and
// the method line are SprintSummary's, the rows are VelocityTable's, and
// every word either of them prints is in lib/reportText.
//
// It draws no chart. Series.Days carries the day by day line behind these
// totals and drawing it is the next plan, for the reason section 1 of
// docs/superpowers/specs/2026-09-09-tam-reports-design.md gives: a chart
// adds no fact this screen does not already state.
//
// This view is the one place in TAM that takes the per-profile lock for a
// read rather than a write, and it takes it through SyncContext.runReport
// rather than runQuietLock. A report runs for minutes with no dialog over
// it, so the shell has to say so: leaving the reducer idle would leave Sync
// and Commit enabled and inert for the whole of it.
export function ReportsView() {
  const { activeId } = useProfile<Profile, Settings>();
  const { runReport, progress } = useSync();
  const [boardId, setBoardId] = useState(0);
  // A sprint id of 0 asks the backend for the board's most recent closed
  // sprint, which is the report the morning after a sprint closes starts
  // from. It is also what lets this view open before the sprint list has
  // arrived, rather than waiting for a second read to learn a number the
  // backend can work out for itself.
  const [sprintId, setSprintId] = useState(0);

  // Both belong to the profile they were chosen for, so a switch clears
  // them in the render that first sees the new id, the way SprintsView
  // does. An effect would be one render too late, and the read that went
  // out in between would pair the new profile with the old board.
  const [madeFor, setMadeFor] = useState(activeId);
  if (madeFor !== activeId) {
    setMadeFor(activeId);
    setBoardId(0);
    setSprintId(0);
  }

  const boards = useBoards(activeId);
  // Only a scrum board has sprints at all, so a kanban board is not offered
  // here rather than offered and then explained.
  const scrumBoards = (boards.data ?? []).filter((b) => b.type === "scrum");
  const board = scrumBoards.find((b) => b.id === boardId) ?? scrumBoards[0];
  const sprints = useBoardSprints(activeId, board?.id ?? 0);
  const closed = useMemo(() => closedNewestFirst(sprints.data ?? []), [sprints.data]);

  const { report, rebuild } = useSprintReport(activeId, board?.id ?? 0, sprintId, runReport);

  // A report left running in Go after the view has moved on holds the
  // profile against the user's next sync or commit for minutes, for a
  // screen nobody is reading. Cancelling is safe when nothing is running,
  // which is why this fires on every switch as well as on unmount; the
  // read that replaces it survives the moment the cancelled one takes to
  // let go of the lock, which is what the one retry in queries/reports.ts
  // is for.
  useEffect(() => {
    if (!activeId) return;
    return () => {
      void call(() => CancelSprintReport(activeId)).catch(() => {});
    };
  }, [activeId, board?.id, sprintId]);

  const failure = report.isError ? errMsg(report.error) : "";
  const busy = !!failure && isBusyRefusal(failure);

  function switchBoard(id: number) {
    setBoardId(id);
    // A sprint id belongs to the board it was picked on, and the two boards
    // need not share one. 0 asks the new board for its own newest closed
    // sprint.
    setSprintId(0);
  }

  // The sprint the picker points at. The report's own answer wins once
  // there is one, so the control and the heading above the numbers always
  // name the same sprint; before that, 0 means the newest closed sprint,
  // which closedNewestFirst puts first.
  const shownSprintId = report.data?.series.sprintId || sprintId || closed[0]?.id || 0;

  function loading() {
    // While this view's own read is in flight it holds the per-profile
    // lock, so nothing else can be reporting progress and the frame on
    // screen is this report's.
    const stage = progress?.stage || "Building the sprint report";
    const count = progress && progress.total > 0 ? `, ${progress.fetched} of ${progress.total} issues` : "";
    return (
      <div className="report-loading">
        <p className="muted" role="status">{`${stage}${count}`}</p>
        <p className="muted small">
          A report fetches each sprint's issues together with their changelogs, which is the heaviest read TAM
          makes, so a table covering several sprints takes minutes against a real instance.
        </p>
      </div>
    );
  }

  function content(r: SprintReport) {
    if (r.unavailable) {
      return <p className="muted report-unavailable" role="status">{unavailableLine(r.unavailable)}</p>;
    }
    return (
      <>
        <SprintSummary series={r.series} builtAt={r.builtAt} />
        {/* Directly under the summary, because the line that says these
            figures are not exact is the last thing the reader saw, and a
            rebuild is the only thing that can replace a report built while
            Jira was cutting changelogs short. */}
        <div className="report-actions">
          <button type="button" className="btn" disabled={report.isFetching} onClick={() => void rebuild()}>
            Rebuild from Jira
          </button>
          <span className="muted small">
            Reads this sprint and every sprint in the table again instead of serving the stored ones.
          </span>
          {report.isFetching && <span className="muted small">Rebuilding</span>}
        </div>
        <h3 className="report-heading">Velocity</h3>
        <p className="muted small">
          This board's last closed sprints, oldest first. Each row carries its own unit, because a board that
          moved from story points to counting cards holds two quantities rather than one column of numbers.
        </p>
        <VelocityTable rows={r.velocity} />
      </>
    );
  }

  function body() {
    if (boards.isError) {
      return (
        <p className="error-text">
          Could not load the boards: {boards.error.message}{" "}
          <button type="button" className="btn" onClick={() => void boards.refetch()}>Retry</button>
        </p>
      );
    }
    if (boards.isLoading) return <p className="muted">Loading the boards</p>;
    if (!board) {
      return (
        <p className="muted">
          No scrum board has been synced for this project, so there is no sprint to report on. The Boards view
          fetches them.
        </p>
      );
    }
    return (
      <>
        {failure && (
          <div className="pending-banner pending-banner-warn" role="alert">
            <p>{busy ? busyLine(failure) : `This sprint's report could not be read: ${failure}`}</p>
            {/* The previous report stays on screen underneath, so this says
                which report that is rather than letting a stale one pass
                for the one that was just asked for. */}
            {report.data && <p className="muted small">The figures below are the last report that was read in full.</p>}
            <p>
              <button
                type="button"
                className="btn"
                disabled={report.isFetching}
                onClick={() => void report.refetch()}
              >
                Retry
              </button>
            </p>
          </div>
        )}
        {report.isLoading ? loading() : report.data ? content(report.data) : null}
      </>
    );
  }

  return (
    <section className="backlog" aria-label="Reports">
      <div className="board-head">
        {/* One scrum board needs no picker, and a select holding one option
            is a control that cannot be used. The board is still named,
            since the report on screen belongs to it and nothing else says
            so. This is the Sprints view's rule, not the Boards view's. */}
        {scrumBoards.length > 1 ? (
          <label className="board-picker">
            <span>Board</span>
            <select aria-label="Board" value={board?.id ?? ""} onChange={(e) => switchBoard(Number(e.target.value))}>
              {scrumBoards.map((b) => (
                <option key={b.id} value={b.id}>{b.name}</option>
              ))}
            </select>
          </label>
        ) : (
          board && <h2 className="board-head-name">{board.name}</h2>
        )}

        {/* Only closed sprints are offered. A report is the numbers a
            review starts with and the velocity table is closed sprints, so
            a live sprint here would be a fifth thing on screen counting
            something else. */}
        {closed.length > 0 && (
          <label className="board-picker">
            <span>Sprint</span>
            <select
              aria-label="Sprint"
              value={String(shownSprintId)}
              onChange={(e) => setSprintId(Number(e.target.value))}
            >
              {closed.map((s) => (
                <option key={s.id} value={s.id}>{s.name}</option>
              ))}
            </select>
          </label>
        )}
      </div>

      <div className="report-body">{body()}</div>
    </section>
  );
}

// closedNewestFirst is the sprint picker's order, and it mirrors
// reports.VelocitySprints in Go: closed sprints by end date, newest first,
// with the higher id breaking a tie. That is the rule a sprint id of 0
// resolves through, so the first row of this list is the sprint the view
// opens on, and the control never points at one sprint while the numbers
// below it describe another.
//
// A sprint whose end date this frontend cannot read sinks to the bottom
// rather than being dropped: it is still pickable, and picking it is
// answered with the sprintHasNoDates state, which says more than leaving it
// out of the list would.
function closedNewestFirst(sprints: Sprint[]): Sprint[] {
  const ended = (s: Sprint) => {
    const t = new Date(s.endDate).getTime();
    return Number.isNaN(t) ? -Infinity : t;
  };
  return sprints
    .filter((s) => s.state === "closed")
    .sort((a, b) => (ended(b) === ended(a) ? b.id - a.id : ended(b) - ended(a)));
}
