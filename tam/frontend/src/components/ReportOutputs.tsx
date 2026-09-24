import { useMemo, useState } from "react";
import { call, errMsg } from "@agile-suite/core";
import { ExportSprintReportPPTX, ExportSprintReportXLSX, PublishSprintReport } from "../api";
import type { SprintReport } from "../api";
import { reportDocument } from "../lib/reportDocument";
import { busyLine, isBusyRefusal, nothingToPublishLine, publishedLine, savedLine } from "../lib/reportText";

// ReportOutputs is the report leaving the screen: a Confluence page, a
// spreadsheet and a deck.
//
// All three render the one document lib/reportDocument builds, so a figure
// reads the same in the app, on the page, in the sheet and on the slide.
// None of them reaches Jira.
//
// Publishing is a write. It happens on this button and never on the view's
// mount, and it says which page it wrote, because a write that answers
// "done" leaves the user to go and find out what it did.
//
// A report that is unavailable has no document at all, which is what leaves
// these three controls disabled with the reason beside them rather than
// letting somebody publish a page of zeroes.

interface Props {
  profileId: string;
  boardId: number;
  report: SprintReport;
  live: boolean;
}

export function ReportOutputs({ profileId, boardId, report, live }: Props) {
  const doc = useMemo(() => reportDocument(report, live), [report, live]);
  const [running, setRunning] = useState("");
  const [done, setDone] = useState("");
  const [failure, setFailure] = useState("");

  function run(what: string, action: () => Promise<string>) {
    setRunning(what);
    setDone("");
    setFailure("");
    void call(action)
      .then(setDone)
      .catch((e) => setFailure(errMsg(e)))
      .finally(() => setRunning(""));
  }

  const busy = running !== "" || !doc;
  return (
    <div className="report-actions">
      <button
        type="button"
        className="btn"
        disabled={busy}
        onClick={() =>
          doc &&
          run("publish", async () => publishedLine((await PublishSprintReport(profileId, boardId, report.series.sprintId, doc)).title))
        }
      >
        Publish to Confluence
      </button>
      <button
        type="button"
        className="btn"
        disabled={busy}
        onClick={() => doc && run("xlsx", async () => savedLine(await ExportSprintReportXLSX(doc)))}
      >
        Export a spreadsheet
      </button>
      <button
        type="button"
        className="btn"
        disabled={busy}
        onClick={() => doc && run("pptx", async () => savedLine(await ExportSprintReportPPTX(doc)))}
      >
        Export a deck
      </button>
      {doc ? (
        <span className="muted small">
          The page, the spreadsheet and the deck carry these figures as tables, with the caveats on them.
        </span>
      ) : (
        <span className="muted small">{nothingToPublishLine()}</span>
      )}
      {running && <span className="muted small" role="status" aria-live="polite">Working on it...</span>}
      {done && <span className="muted small" role="status">{done}</span>}
      {failure && (
        <span className="error-text small" role="alert">
          {isBusyRefusal(failure) ? busyLine(failure) : failure}
        </span>
      )}
    </div>
  );
}
