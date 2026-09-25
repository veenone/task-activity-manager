import { useMemo, useState } from "react";
import { call, errMsg } from "@agile-suite/core";
import { ExportSprintReportPPTX, ExportSprintReportXLSX, PublishSprintReport } from "../api";
import type { SprintReport } from "../api";
import { reportDocument } from "../lib/reportDocument";
import {
  busyLine,
  isBusyRefusal,
  nothingToPublishLine,
  publishedLine,
  publisherAnnouncement,
  publisherStatusWord,
  savedLine,
} from "../lib/reportText";

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
//
// Each publisher carries its own state, and the ribbon under the buttons
// shows all three. They used to share one "running" string and one outcome,
// which meant a confirmation naming a page written to Confluence was wiped
// the moment the user also exported a file. The three finish independently,
// so they report independently.

interface Props {
  profileId: string;
  boardId: number;
  report: SprintReport;
  live: boolean;
}

// A publisher is idle until it is used. "failed" carries a reason and "done"
// carries where the output went; the other two carry nothing.
type Status = "idle" | "running" | "done" | "failed";
type Outcome = { status: Status; message: string };

const IDLE: Outcome = { status: "idle", message: "" };

// The three, in the order they are offered. The label is what the ribbon and
// the announcement call each one, so it is a noun for the thing produced
// rather than the verb on the button: a ribbon cell reading "Export a deck"
// would be describing the button, not its state.
const PUBLISHERS = [
  { id: "publish", label: "Confluence page", action: "Publish to Confluence" },
  { id: "xlsx", label: "Spreadsheet", action: "Export a spreadsheet" },
  { id: "pptx", label: "Deck", action: "Export a deck" },
] as const;

type PublisherID = (typeof PUBLISHERS)[number]["id"];

export function ReportOutputs({ profileId, boardId, report, live }: Props) {
  const doc = useMemo(() => reportDocument(report, live), [report, live]);
  const [outcomes, setOutcomes] = useState<Record<PublisherID, Outcome>>({
    publish: IDLE,
    xlsx: IDLE,
    pptx: IDLE,
  });
  // The last change, for the one live region. A screen reader is told what
  // changed rather than having three regions announce at once.
  const [announcement, setAnnouncement] = useState("");

  function set(id: PublisherID, label: string, next: Outcome) {
    setOutcomes((prev) => ({ ...prev, [id]: next }));
    setAnnouncement(publisherAnnouncement(label, next.status, next.message));
  }

  // An export answers with the path it wrote, or "" when the user closed the
  // save dialog. Nothing was written then, so the publisher goes back to
  // idle: a cancel is neither a success to announce nor a failure to colour.
  function outcomeOf(message: string): Outcome {
    return message ? { status: "done", message } : IDLE;
  }

  function run(id: PublisherID, label: string, action: () => Promise<string>) {
    set(id, label, { status: "running", message: "" });
    void call(action)
      .then((message) => set(id, label, outcomeOf(message)))
      .catch((e) => set(id, label, { status: "failed", message: errMsg(e) }));
  }

  // One at a time, because two exports would put two save dialogs on screen
  // and publishing takes the profile's lock. Which one is running is what
  // the ribbon now says.
  const running = PUBLISHERS.find((p) => outcomes[p.id].status === "running");
  const busy = running !== undefined || !doc;

  function onRun(id: PublisherID, label: string) {
    if (!doc) return;
    if (id === "publish") {
      run(id, label, async () =>
        publishedLine((await PublishSprintReport(profileId, boardId, report.series.sprintId, doc)).title));
      return;
    }
    const exportIt = id === "xlsx" ? ExportSprintReportXLSX : ExportSprintReportPPTX;
    run(id, label, async () => {
      const path = await exportIt(doc);
      return path ? savedLine(path) : "";
    });
  }

  return (
    <>
      <div className="report-actions">
        {PUBLISHERS.map((p) => (
          <button key={p.id} type="button" className="btn" disabled={busy} onClick={() => onRun(p.id, p.label)}>
            {p.action}
          </button>
        ))}
        {doc ? (
          <span className="muted small">
            The page, the spreadsheet and the deck carry these figures as tables, with the caveats on them.
          </span>
        ) : (
          <span className="muted small">{nothingToPublishLine()}</span>
        )}
      </div>
      {/* One region for all three. Three of them announce over each other,
          and a region mounted with its text already inside announces
          unreliably, so this one is always present and only its text
          changes. */}
      <p className="sr-only" role="status" aria-live="polite">{announcement}</p>
      <ul className="report-ribbon">
        {PUBLISHERS.map((p) => {
          const o = outcomes[p.id];
          return (
            // The accessible name is the label and the state together, so a
            // reader moving through the ribbon hears which publisher each
            // cell is about.
            <li
              key={p.id}
              className={`report-ribbon-item report-ribbon-${o.status}`}
              aria-label={`${p.label}: ${publisherStatusWord(o.status)}`}
            >
              <span className="report-ribbon-label">{p.label}</span>
              <span className="report-ribbon-state">{publisherStatusWord(o.status)}</span>
              {o.status === "done" && <span className="muted small report-ribbon-note">{o.message}</span>}
              {o.status === "failed" && (
                <span className="error-text small report-ribbon-note" role="alert">
                  {isBusyRefusal(o.message) ? busyLine(o.message) : o.message}
                </span>
              )}
            </li>
          );
        })}
      </ul>
    </>
  );
}
