import type { ReportProgress } from "../api";

// publishText is every sentence the act of publishing and exporting a sprint
// report puts on screen: a button's state, the page it wrote, the file it
// saved, the lock that refused and the frame of progress under it.
//
// It split out of lib/reportText, which words the figures. Nothing here says
// anything about a figure, and the two groups share no caller: the outputs
// ribbon and the sync context read this file, while the summary, the tables,
// the charts and the document read that one.

// nothingToPublishLine is what the three outputs say when there is no
// report to render. It does not repeat the reason: unavailableLine is
// already on the surface these controls sit on, and the same paragraph
// twice reads as two different problems.
export function nothingToPublishLine(): string {
  return "There is no report to publish or export yet.";
}

// publishedLine is what a publish that worked says. It names the page,
// because a write the user asked for that answers "done" leaves them to go
// and look for what it did.
export function publishedLine(title: string): string {
  return `Published to the Confluence page "${title}".`;
}

// partlyPublishedLine is what a publish that wrote the page but could not put
// every chart on it says. The page comes first, because it is there and the
// user can open it, and what is missing from it follows. Go's warning names
// each chart and what Confluence said about it.
export function partlyPublishedLine(title: string, warning: string): string {
  return `${publishedLine(title)} ${warning}`;
}

// savedLine is what an export says: the path, so the file can be found and
// attached to something without hunting for it.
export function savedLine(path: string): string {
  return `Saved to ${path}.`;
}

// The publisher ribbon. Publishing to Confluence, exporting a spreadsheet
// and exporting a deck are three operations that each take a moment, and
// they finish independently, so each one carries its own state rather than
// the three sharing one.

// publisherStatusWord is the state a publisher is in. "Ready" rather than
// "Idle" or nothing at all: a control that has not been used yet has not
// failed, and a blank cell beside two filled ones reads as a problem.
//
// "Partly done" is the publish that wrote its page and could not put every
// chart on it. Neither "Done" nor "Failed" would be true: the report is on the
// page and a picture of it is not.
export function publisherStatusWord(status: string): string {
  switch (status) {
    case "running":
      return "Working";
    case "done":
      return "Done";
    case "warned":
      return "Partly done";
    case "failed":
      return "Failed";
    default:
      return "Ready";
  }
}

// publisherAnnouncement is what a screen reader hears when one publisher
// changes state. It names the publisher because three of them change
// independently and a bare "Done" would not say which finished, and it
// carries the outcome because where the page or the file went is the part
// worth hearing.
export function publisherAnnouncement(label: string, status: string, message: string): string {
  const state = `${label}: ${publisherStatusWord(status)}`;
  return message ? `${state}. ${message}` : `${state}.`;
}

// isBusyRefusal recognises the refusal App.acquire makes in Go and the one
// SyncContext makes in front of it. Both end "is already running for this
// profile" and both name the operation holding the lock in front of it, Go
// from its own busy map and SyncContext from the operation name it carries
// beside the lock, so the phrase they share is what tells a refusal from a
// failed read. The one refusal that names nothing says "another operation",
// which this matches too.
export function isBusyRefusal(message: string): boolean {
  return / is already running for this profile/.test(message);
}

// busyLine is what a refused read says. The lock refuses rather than
// queues, so the only thing to do is wait for the other operation and ask
// again, and quoting the refusal is what says which operation that is.
export function busyLine(message: string): string {
  // The message is Go's, or SyncContext's in front of it, and both are
  // worded to sit inside an error rather than to open a paragraph.
  const opening = message.charAt(0).toUpperCase() + message.slice(1);
  return (
    `${opening}. A report takes the same per-profile lock a sync and a commit do, ` +
    "and that lock refuses rather than waits, so ask again once the other one has finished."
  );
}

// progressStage is the label for one frame of a report's progress. It
// carries no count: the status bar and the view each put the frame's own
// fetched and total beside it.
export function progressStage(p: ReportProgress): string {
  const name = p.sprintName || `sprint ${p.sprintId}`;
  if (p.phase === "sprint") return `Reading ${name}`;
  if (p.phase === "velocity") return `Reading ${name} for the velocity table`;
  return "Building the sprint report";
}
