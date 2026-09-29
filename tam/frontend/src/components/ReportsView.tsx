import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { call, errMsg, useProfile } from "@agile-suite/core";
import { CancelSprintReport } from "../api";
import type { Profile, Settings, SprintReport } from "../api";
import { useBoards, useBoardSprints } from "../queries/boards";
import { useBoardCapacity } from "../queries/capacity";
import { closedNewestFirst } from "../lib/closedSprints";
import { useSprintReport } from "../queries/reports";
import { useSync } from "../contexts/SyncContext";
import { busyLine, isBusyRefusal, unavailableLine } from "../lib/reportText";
import { flowDocument } from "../lib/flowDocument";
import type { ChartImages } from "../lib/chartImage";
import { reportDocument } from "../lib/reportDocument";
import { FlowReport } from "./FlowReport";
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
  // The charts are in the DOM and the document is built in TypeScript, so
  // something has to collect their pictures at the moment the user asks for an
  // output. This frame is that scope: ReportOutputs queries the charts inside
  // it and nothing outside it, so a chart drawn elsewhere on the page can
  // never end up in a report.
  const reportFrame = useRef<HTMLDivElement>(null);
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
  // Every board, kanban ones included. They used to be filtered out, so a
  // kanban team opened this view and found none of its own boards in it.
  // Having no sprint is a reason to report on a board differently, not a
  // reason to hide it.
  const offeredBoards = boards.data ?? [];
  const board = offeredBoards.find((b) => b.id === boardId) ?? offeredBoards[0];
  // Which report this board gets. It is the board's own type and not a guess
  // from whether sprints came back: a scrum board whose sprints have not
  // synced yet is still a scrum board, and reading it as kanban would answer
  // a sprint question with a column count.
  const kanban = board?.type === "kanban";
  // A kanban board has no sprints to ask for, and asking anyway is a call Go
  // answers with ErrNoSprints for every kanban board in the list.
  const sprints = useBoardSprints(activeId, kanban ? 0 : board?.id ?? 0);
  const closed = useMemo(() => closedNewestFirst(sprints.data ?? []), [sprints.data]);
  const active = (sprints.data ?? []).filter((s) => s.state === "active");
  const offered = [...closed, ...active];
  const requestedSprintId = sprintId || (closed.length ? 0 : active[0]?.id ?? 0);
  const live = active.some((s) => s.id === requestedSprintId);
  const reportBoardId = !kanban && sprints.isSuccess ? board?.id ?? 0 : 0;

  // The kanban read. Column capacity needs no sprint and no changelog: Go
  // counts the board's own cards out of the synced cache against the limits
  // the boards sync stored, which is why this one takes no lock and has no
  // cancel path. #105 built the count and publishes it; what was missing was
  // a way for a kanban board to reach it.
  const capacity = useBoardCapacity(activeId, kanban ? board?.id ?? 0 : 0);

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

  // The report on screen describes a sprint that is still running, which is
  // what makes its figures provisional and its rebuild a refresh. The head
  // and the body both word themselves from it, so it is resolved once here
  // rather than twice from two different sprint ids.
  const shown = report.data;
  const inProgress = active.some((s) => s.id === shown?.series.sprintId);
  // A report with nothing in it cannot be rebuilt into something, and it is
  // the state that says why in the body instead. A kanban board's capacity is
  // a cache read with nothing to rebuild from Jira, so it offers no control:
  // the figure moves when the project syncs, not when this view asks again.
  const canRebuild = !kanban && !!shown && !shown.unavailable;

  // What the outputs bar sends. Both kinds build the one ReportDocument shape,
  // which is why there is one bar and not two: ReportOutputs, the Confluence
  // publisher, the spreadsheet and the deck all take a document and know
  // nothing about which report made it.
  // useMemo, or `?? []` hands back a new array on every render and the
  // builder below it is rebuilt every time, which would rebuild the document
  // on every render of the view.
  const columns = useMemo(() => capacity.data ?? [], [capacity.data]);
  const sprintDoc = useCallback(
    (images?: ChartImages) => (shown ? reportDocument(shown, inProgress, images) : null),
    [shown, inProgress],
  );
  // No images: a capacity table has no chart to rasterise yet.
  const flowDoc = useCallback(
    () => flowDocument(board?.name ?? "", columns),
    [board?.name, columns],
  );
  // Which one, and whether there is anything to publish at all.
  const outputs = kanban
    ? { show: columns.length > 0, build: flowDoc, sprintId: 0 }
    : { show: !!shown, build: sprintDoc, sprintId: shown?.series.sprintId ?? 0 };

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
      return <p className="muted report-unavailable" role="status">{unavailableLine(r.unavailable)}</p>;
    }
    return (
      <>
        <SprintSummary series={r.series} builtAt={r.builtAt} live={inProgress} />
        {/* Nothing between the figures and the charts. Rebuild sits with the
            pickers, because it re-reads the report they name, and the three
            outputs are the bar at the foot of the frame, because publishing
            is what a reader does after the evidence rather than before it.
            They were one strip here, and that strip was two nested copies of
            .report-actions: one rule carrying flex-wrap and a 70ch cap,
            applied twice, wrapped it to three rows at every window width. */}
        {/* The burndown is the wide one: it carries a point per sprint day,
            while the outcome chart carries five bars. */}
        <div className="report-charts">
          <BurndownChart days={r.series.days} unit={r.series.unit} busy={report.isFetching} />
          <OutcomeChart series={r.series} live={inProgress} busy={report.isFetching} />
        </div>
        {/* The panel: a head that does not scroll, holding the chart and the
            notes about it, over the table that does. The paragraph that used
            to sit here said each row shows its own unit, which every cell
            already prints; the half that was not redundant, that the rows run
            oldest first, is in the column header now, where it cannot scroll
            away from the rows it describes. */}
        <div className="report-velocity">
          <div className="report-velocity-head">
            <h3 className="report-heading" id="report-velocity-heading">Velocity</h3>
            <VelocityChart rows={r.velocity} busy={report.isFetching} tabled />
          </div>
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
    // A kanban board's report is capacity, and it needs no sprint list, so
    // this comes before the two sprint states below it.
    if (kanban) {
      return <FlowReport capacity={capacity} onOpenBoards={onOpenBoards} />;
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
      <div className="report-frame" ref={reportFrame}>
        <div className="board-head">
        {/* One scrum board needs no picker, and a select holding one option
            is a control that cannot be used. The board is still named,
            since the report on screen belongs to it and nothing else says
            so. This is the Sprints view's rule, not the Boards view's. */}
        {offeredBoards.length > 1 ? (
          <label className="board-picker">
            <span>Board</span>
            <select aria-label="Board" value={board?.id ?? ""} onChange={(e) => switchBoard(Number(e.target.value))}>
              {offeredBoards.map((b) => (
                <option key={b.id} value={b.id}>{b.name}</option>
              ))}
            </select>
          </label>
        ) : (
          board && <h2 className="board-head-name">{board.name}</h2>
        )}

        {/* Active sprints are provisional reports; velocity stays closed-only.
            A kanban board has none, so the slot is simply empty rather than
            holding a control that cannot be used. */}
        {!kanban && offered.length > 0 && (
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

        {/* Rebuild re-reads the report the two pickers name, so it is the
            third control of the same group rather than a row in the reading
            column below them. .board-head-actions is the class every other
            head in the app puts its controls in, and its margin-left: auto
            is what sets this one at the far end. */}
        {canRebuild && (
          <div className="board-head-actions">
            <button type="button" className="btn" disabled={report.isFetching} onClick={() => void rebuild()}>
              {inProgress ? "Refresh report" : "Rebuild from Jira"}
            </button>
            {report.isFetching && (
              <span className="muted small" role="status" aria-live="polite">Rebuilding the report…</span>
            )}
          </div>
        )}
        </div>

        <div className="report-body">{body()}</div>

        {/* The foot of the frame, outside the body. Publishing is what a
            reader does once the figures, the charts and the trend are read,
            so the controls for it come after all three; and being a sibling
            of the body rather than a row inside it means three publishers
            failing at once, each with its own unbounded reason, scroll inside
            this bar instead of collapsing the velocity panel that was the
            body's only shrinkable child. */}
        {outputs.show && (
          <div className="report-outputs">
            <ReportOutputs
              profileId={activeId}
              boardId={board?.id ?? 0}
              sprintId={outputs.sprintId}
              build={outputs.build}
              charts={reportFrame}
            />
          </div>
        )}
      </div>
    </section>
  );
}
