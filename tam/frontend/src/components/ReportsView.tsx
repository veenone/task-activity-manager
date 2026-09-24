import { useEffect, useMemo, useState } from "react";
import { call, errMsg, useProfile } from "@agile-suite/core";
import { CancelSprintReport } from "../api";
import type { Profile, Settings, Sprint, SprintReport } from "../api";
import { useBoards, useBoardSprints } from "../queries/boards";
import { useSprintReport } from "../queries/reports";
import { useSync } from "../contexts/SyncContext";
import { busyLine, isBusyRefusal, unavailableLine } from "../lib/reportText";
import { ReportOutputs } from "./ReportOutputs";
import { SprintSummary } from "./SprintSummary";
import { VelocityTable } from "./VelocityTable";
import { BurndownChart } from "./charts/BurndownChart";
import { OutcomeChart } from "./charts/OutcomeChart";
import { VelocityChart } from "./charts/VelocityChart";

// ReportsView is the pickers, the states, and the wiring. The sentence and
// the method line are SprintSummary's, the rows are VelocityTable's, and
// every word either of them prints is in lib/reportText.
//
// The charts are in components/charts, drawn from Series.Days and the
// velocity rows this view already receives. Each states its numbers in text
// beside the marks, so none of them is the only way to read a figure.
//
// This view is the one place in TAM that takes the per-profile lock for a
// read rather than a write, and it takes it through SyncContext.runReport
// rather than runQuietLock. A report runs for minutes with no dialog over
// it, so the shell has to say so: leaving the reducer idle would leave Sync
// and Commit enabled and inert for the whole of it.
export function ReportsView({ onOpenBoards }: { onOpenBoards?: () => void } = {}) {
  const { activeId } = useProfile<Profile, Settings>();
  const { runReport, progress, running } = useSync();
  const [boardId, setBoardId] = useState(0);
  // A sprint id of 0 asks the backend for the board's most recent closed
  // sprint, which is the report the morning after a sprint closes starts
  // from. When none has closed yet, the first active sprint is the default.
  const [sprintId, setSprintId] = useState(0);
  const [canceling, setCanceling] = useState(false);

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
  const active = (sprints.data ?? []).filter((s) => s.state === "active");
  const offered = [...closed, ...active];
  const requestedSprintId = sprintId || (closed.length ? 0 : active[0]?.id ?? 0);
  const live = active.some((s) => s.id === requestedSprintId);
  const reportBoardId = sprints.isSuccess ? board?.id ?? 0 : 0;

  // Wait for the list so a board running its first sprint never asks for
  // a nonexistent closed report before resolving its active default.
  const { report, rebuild } = useSprintReport(
    activeId, reportBoardId, requestedSprintId, runReport, live,
  );

  // A report left running in Go after the view has moved on holds the
  // profile against the user's next sync or commit for minutes, for a
  // screen nobody is reading. Cancelling is safe when nothing is running,
  // which is why this fires on every switch as well as on unmount; the
  // read that replaces it survives the moment the cancelled one takes to
  // let go of the lock, which is what the one retry in queries/reports.ts
  // is for.
  useEffect(() => {
    if (!activeId || !reportBoardId) return;
    return () => {
      void call(() => CancelSprintReport(activeId)).catch(() => {});
    };
  }, [activeId, reportBoardId, requestedSprintId]);

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
  const shownSprintId = report.data?.series.sprintId || requestedSprintId || closed[0]?.id || 0;

  function loading() {
    // The frame in the shell belongs to whichever operation holds the
    // per-profile lock, and this read being in flight does not mean that
    // is this one: a report refused because a sync holds the lock waits
    // out its one retry with the sync's own frames still arriving. So a
    // frame is read only while the operation running is this view's
    // report, and otherwise this says the one thing it knows.
    const frame = running === "report" ? progress : null;
    const stage = frame?.stage || "Building the sprint report";
    const count = frame && frame.total > 0 ? `, ${frame.fetched} of ${frame.total} issues` : "";
    return (
      <div className="report-loading">
        <p className="muted" role="status">{`${stage}${count}`}</p>
        <p className="muted small">
          This can take a little longer because TAM reads each issue's history. A sprint already in the store
          comes back at once; a fresh read from Jira may take a few minutes.
        </p>
        <div className="report-actions">
          <button
            type="button"
            className="btn"
            disabled={canceling}
            onClick={() => {
              setCanceling(true);
              void call(() => CancelSprintReport(activeId)).finally(() => setCanceling(false));
            }}
          >
            {canceling ? "Canceling report" : "Cancel report"}
          </button>
          {canceling && <span className="muted small" role="status">Canceling report…</span>}
        </div>
      </div>
    );
  }

  function content(r: SprintReport) {
    if (r.unavailable) {
      // The three outputs are still drawn, disabled, with their own reason
      // beside them: a report that cannot be built is a report that cannot
      // be published either, and saying so where the controls are is what
      // keeps somebody from looking for them.
      return (
        <>
          <p className="muted report-unavailable" role="status">{unavailableLine(r.unavailable)}</p>
          <ReportOutputs profileId={activeId} boardId={reportBoardId} report={r} live={false} />
        </>
      );
    }
    const inProgress = active.some((s) => s.id === r.series.sprintId);
    return (
      <>
        <SprintSummary series={r.series} builtAt={r.builtAt} live={inProgress} />
        {/* Directly under the summary, because the line that says these
            figures are not exact is the last thing the reader saw, and a
            rebuild is the only thing that can replace a report built while
            Jira was cutting changelogs short. */}
        <div className="report-actions">
          <button type="button" className="btn" disabled={report.isFetching} onClick={() => void rebuild()}>
            {inProgress ? "Refresh report" : "Rebuild from Jira"}
          </button>
          <span className="muted small">
            Reads this sprint and the comparison rows again instead of using saved results.
          </span>
        {report.isFetching && <span className="muted small" role="status" aria-live="polite">Rebuilding the report...</span>}
        </div>
        <ReportOutputs profileId={activeId} boardId={reportBoardId} report={r} live={inProgress} />
        {/* The burndown is the wide one: it carries a point per sprint day,
            while the outcome chart carries five bars. */}
        <div className="report-charts">
          <BurndownChart days={r.series.days} unit={r.series.unit} busy={report.isFetching} />
          <OutcomeChart series={r.series} live={inProgress} busy={report.isFetching} />
        </div>
        <h3 className="report-heading">Velocity</h3>
        <p className="muted small">
          Recent closed sprints on this board, oldest first. Each row shows its own unit so points and cards stay
          distinct when a board changes how it estimates work.
        </p>
        <div className="report-velocity">
          <VelocityChart rows={r.velocity} busy={report.isFetching} tabled />
          <VelocityTable rows={r.velocity} />
        </div>
      </>
    );
  }

  function body() {
    if (boards.isError) {
      return (
        <p className="error-text" role="alert">
          Could not load the boards. Retry the request or open Boards to sync them.{" "}
          <button type="button" className="btn" onClick={() => void boards.refetch()}>Retry</button>
          {onOpenBoards && <button type="button" className="btn" onClick={onOpenBoards}>Open Boards</button>}
        </p>
      );
    }
    if (boards.isLoading) return <p className="muted" role="status">Loading the boards</p>;
    if (!board) {
      return (
        <p className="muted" role="status">
          No scrum board has been synced for this project, so there is no sprint to report on. Open Boards to
          sync one.
          {onOpenBoards && <>{" "}<button type="button" className="btn" onClick={onOpenBoards}>Open Boards</button></>}
        </p>
      );
    }
    if (sprints.isError) {
      return <p className="error-text" role="alert">
        Could not load the sprints: {sprints.error.message}{" "}
        <button type="button" className="btn" onClick={() => void sprints.refetch()}>Retry</button>
      </p>;
    }
    if (sprints.isLoading) return <p className="muted" role="status">Loading the sprints</p>;
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
      <h2 className="sr-only">Reports</h2>
      <div className="report-frame">
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

        {/* Active sprints are provisional reports; velocity stays closed-only. */}
        {offered.length > 0 && (
          <label className="board-picker">
            <span>Sprint</span>
            <select
              aria-label="Sprint"
              value={String(shownSprintId)}
              onChange={(e) => setSprintId(Number(e.target.value))}
            >
              {offered.map((s) => (
                <option key={s.id} value={s.id}>{s.name}{s.state === "active" ? " (in progress)" : ""}</option>
              ))}
            </select>
          </label>
        )}
        </div>

        <div className="report-body">{body()}</div>
      </div>
    </section>
  );
}

// closedNewestFirst is the sprint picker's order, and it mirrors
// reports.VelocitySprints in Go: closed sprints by actual completion date
// (planned end when completion is absent), newest first,
// with the higher id breaking a tie. That is the rule a sprint id of 0
// resolves through, so the first row of this list is the sprint the view
// opens on, and the control never points at one sprint while the numbers
// below it describe another.
//
// A sprint whose start date will not parse is dropped, because Go drops it
// too: it parses both dates and skips a sprint that fails either. Sorting
// on the end date alone let a sprint with a readable end and an unreadable
// start sit at the top of this list while a sprint id of 0 resolved past
// it, so the picker and the numbers disagreed about which sprint was the
// newest one. Nothing was ever mis-titled, since the heading follows the
// response, but the two should agree without that backstop.
//
// A sprint whose end date will not parse sinks to the bottom instead of
// being dropped. It cannot be the first row from there, so it costs the
// agreement above nothing, and picking it is answered with the
// sprintHasNoDates state, which says more than leaving it out would.
function closedNewestFirst(sprints: Sprint[]): Sprint[] {
  const readable = (d: string) => !Number.isNaN(new Date(d).getTime());
  const ended = (s: Sprint) => {
    const t = new Date(s.completeDate?.trim() ? s.completeDate : s.endDate).getTime();
    return Number.isNaN(t) ? -Infinity : t;
  };
  return sprints
    .filter((s) => s.state === "closed" && readable(s.startDate))
    .sort((a, b) => (ended(b) === ended(a) ? b.id - a.id : ended(b) - ended(a)));
}
