import { useMemo, useState } from "react";
import type { RefObject } from "react";
import { call, errMsg } from "@agile-suite/core";
import { ExportSprintReportPPTX, ExportSprintReportXLSX, PublishSprintReport } from "../api";
import type { ReportDocument } from "../api";
import { chartImages } from "../lib/chartImage";
import type { ChartImages } from "../lib/chartImage";
import {
  busyLine,
  isBusyRefusal,
  nothingToPublishLine,
  partlyPublishedLine,
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
  // sprintId is only where the page goes: PublishSprintReport resolves the
  // sprint's ritual page as the parent and falls back to the configured root
  // when there is none. A kanban board has no sprint and passes 0, which is
  // that fallback, so its report lands under the root.
  sprintId: number;
  // build is the document, and taking it as a function rather than building it
  // here is what lets one bar serve both report kinds. A sprint report and a
  // kanban report are different figures in the same ReportDocument shape, so
  // the publishers below need to know which document they are sending and
  // nothing about which kind of report made it.
  //
  // It takes the chart pictures because the document is rebuilt with them at
  // the moment a button is pressed. A report with no charts ignores the
  // argument.
  build: (images?: ChartImages) => ReportDocument | null;
  // charts is the report frame the view drew, and the only place a picture is
  // looked for. The charts are in the DOM rather than in the report, so they
  // are collected on the click: rasterising three of them on every render
  // would be work for a button nobody pressed.
  charts: RefObject<HTMLDivElement | null>;
}

// A publisher is idle until it is used. "failed" carries a reason, "done"
// carries where the output went, and "warned" carries both: the page that was
// written and the chart that did not reach it. A refused attachment is not a
// failed publish, because the tables on the page are the report.
type Status = "idle" | "running" | "done" | "warned" | "failed";
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

export function ReportOutputs({ profileId, boardId, sprintId, build, charts }: Props) {
  const doc = useMemo(() => build(), [build]);
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

  function run(id: PublisherID, label: string, action: () => Promise<Outcome>) {
    set(id, label, { status: "running", message: "" });
    void call(action)
      .then((next) => set(id, label, next))
      .catch((e) => set(id, label, { status: "failed", message: errMsg(e) }));
  }

  // One at a time, because two exports would put two save dialogs on screen
  // and publishing takes the profile's lock. Which one is running is what
  // the ribbon now says.
  const running = PUBLISHERS.find((p) => outcomes[p.id].status === "running");
  const busy = running !== undefined || !doc;

  // drawn is the document with the pictures of the charts on screen in it. The
  // document built above is what it falls back to, which is the same document
  // without them.
  async function drawn(built: ReportDocument): Promise<ReportDocument> {
    return build(await chartImages(charts.current)) ?? built;
  }

  function onRun(id: PublisherID, label: string) {
    if (!doc) return;
    if (id === "publish") {
      run(id, label, async () => {
        // A warning means the page is written and a chart is not on it. The
        // page is the outcome either way, so it is named either way.
        const page = await PublishSprintReport(profileId, boardId, sprintId, await drawn(doc));
        return page.warning
          ? { status: "warned", message: partlyPublishedLine(page.title, page.warning) }
          : { status: "done", message: publishedLine(page.title) };
      });
      return;
    }
    const exportIt = id === "xlsx" ? ExportSprintReportXLSX : ExportSprintReportPPTX;
    run(id, label, async () => {
      const path = await exportIt(await drawn(doc));
      return outcomeOf(path ? savedLine(path) : "");
    });
  }

  // Whether the ribbon has anything to report. Issue #90 gave an unused
  // publisher the word "Ready" so that a blank cell beside two filled ones
  // would not read as a problem, and that still holds once the ribbon is
  // live. It was never a reason to mount three cells saying "Ready" under
  // three enabled buttons before anything has run: they state what the
  // buttons state, and this view pays for every row out of the velocity
  // table's height.
  const used = PUBLISHERS.some((p) => outcomes[p.id].status !== "idle");

  return (
    <>
      {/* The buttons and nothing else. This used to be a .report-actions
          nested inside the view's own .report-actions, so one rule carrying
          flex-wrap and a 70ch cap applied twice and the strip wrapped to
          three rows at every width. The row is its own class now and the view
          owns the bar around it. */}
      <div className="report-outputs-row">
        {PUBLISHERS.map((p) => (
          <button key={p.id} type="button" className="btn" disabled={busy} onClick={() => onRun(p.id, p.label)}>
            {p.action}
          </button>
        ))}
      </div>
      {/* One region for all three. Three of them announce over each other,
          and a region mounted with its text already inside announces
          unreliably, so this one is always present and only its text
          changes. */}
      <p className="sr-only" role="status" aria-live="polite">{announcement}</p>
      {/* One slot under the buttons, never empty and never twice filled: the
          reason the controls are dead, or what the outputs will carry before
          anyone has pressed one, or where the three of them got to. The
          guidance used to be a flex item inside the button strip, 233
          characters of it, which is what guaranteed the wrap. */}
      {!doc ? (
        <p className="muted small report-outputs-note">{nothingToPublishLine()}</p>
      ) : !used ? (
        <p className="muted small report-outputs-note">
          The page, the spreadsheet and the deck carry these figures as tables, with the caveats on them, and the
          charts as pictures. On the page the charts are attached files the page references.
        </p>
      ) : (
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
              {(o.status === "done" || o.status === "warned") && (
                <span className="muted small report-ribbon-note">{o.message}</span>
              )}
              {o.status === "failed" && (
                <span className="error-text small report-ribbon-note" role="alert">
                  {isBusyRefusal(o.message) ? busyLine(o.message) : o.message}
                </span>
              )}
            </li>
          );
        })}
      </ul>
      )}
    </>
  );
}
