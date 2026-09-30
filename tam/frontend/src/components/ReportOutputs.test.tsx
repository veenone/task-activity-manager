import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { SprintReport } from "../api";
import { nothingToPublishLine, publishedLine, publisherStatusWord, savedLine } from "../lib/publishText";
import { reportDocument } from "../lib/reportDocument";
import { DONE_AGREEMENT } from "../lib/ritualText";
import { ReportOutputs } from "./ReportOutputs";

const bindings = vi.hoisted(() => ({
  PublishSprintReport: vi.fn(),
  ExportSprintReportXLSX: vi.fn(),
  ExportSprintReportPPTX: vi.fn(),
  // The two local reads behind the done agreement section, which is
  // collected on the click the way the chart pictures are.
  ListRitualDocuments: vi.fn(),
  DoneAgreementTicks: vi.fn(),
}));

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, ...bindings };
});

function report(over: Partial<SprintReport> = {}): SprintReport {
  return {
    series: {
      sprintId: 11,
      sprintName: "Sprint 11",
      unit: "points",
      unitReason: "",
      committed: 34,
      added: 5,
      removed: 2,
      completed: 29,
      carriedOver: 10,
      days: [{ date: "2026-03-02", scope: 34, completed: 0, remaining: 34, ideal: 34 }],
      truncated: [],
      issues: [],
    },
    velocity: [
      { sprintId: 11, sprintName: "Sprint 11", unit: "points", unitReason: "", committed: 34, completed: 29, truncated: false },
    ],
    builtAt: "2026-03-16T09:00:00Z",
    unavailable: "",
    ...over,
  };
}

// No frame, so no chart is on screen and the document carries no picture. What
// a drawn chart becomes is lib/chartImage's own suite; the canvas that ends it
// does not exist under this runner.
function draw(over: Partial<SprintReport> = {}) {
  const r = report(over);
  render(
    <ReportOutputs
      profileId="p1"
      boardId={1}
      doc={reportDocument(r, false)}
      sprint={{ report: r, live: false, charts: { current: null } }}
    />,
  );
}

beforeEach(() => {
  bindings.PublishSprintReport.mockReset();
  bindings.ExportSprintReportXLSX.mockReset();
  bindings.ExportSprintReportPPTX.mockReset();
  // No agreement document, so no done agreement section, which is the state
  // every test here but the two below it is about.
  bindings.ListRitualDocuments.mockReset().mockResolvedValue([]);
  bindings.DoneAgreementTicks.mockReset().mockResolvedValue({});
});

describe("ReportOutputs", () => {
  it("publishes the report on screen and says which page it wrote", async () => {
    bindings.PublishSprintReport.mockResolvedValue({ title: "Sprint 11 · Report", pageId: "1234" });
    draw();
    await userEvent.click(screen.getByRole("button", { name: "Publish to Confluence" }));
    await waitFor(() => expect(bindings.PublishSprintReport).toHaveBeenCalled());
    const [profileId, boardId, sprintId, doc] = bindings.PublishSprintReport.mock.calls[0];
    expect([profileId, boardId, sprintId]).toEqual(["p1", 1, 11]);
    expect(doc.title).toBe("Sprint 11 · Report");
    expect(doc.sections[0].notes.join(" ")).toContain("Committed is a minimum estimate");
    expect(await screen.findByText(publishedLine("Sprint 11 · Report"))).toBeInTheDocument();
  });

  it("exports a spreadsheet and a deck and says where each one was saved", async () => {
    bindings.ExportSprintReportXLSX.mockResolvedValue("/home/qa/tam-report-sprint-11-1.xlsx");
    bindings.ExportSprintReportPPTX.mockResolvedValue("/home/qa/tam-report-sprint-11-1.pptx");
    draw();
    await userEvent.click(screen.getByRole("button", { name: "Export a spreadsheet" }));
    expect(await screen.findByText(savedLine("/home/qa/tam-report-sprint-11-1.xlsx"))).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Export a deck" }));
    expect(await screen.findByText(savedLine("/home/qa/tam-report-sprint-11-1.pptx"))).toBeInTheDocument();
    expect(bindings.ExportSprintReportXLSX.mock.calls[0][0].sections).toHaveLength(3);
  });

  it("says nothing at all when the save dialog is cancelled", async () => {
    // Go answers a cancelled dialog with an empty path and no error. Nothing
    // was written, so nothing is claimed and nothing is red.
    bindings.ExportSprintReportXLSX.mockResolvedValue("");
    draw();
    await userEvent.click(screen.getByRole("button", { name: "Export a spreadsheet" }));
    await waitFor(() => expect(bindings.ExportSprintReportXLSX).toHaveBeenCalled());
    await waitFor(() => expect(screen.queryByText("Working on it...")).toBeNull());
    expect(screen.queryByText(/Saved to/)).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("offers nothing for a report that is unavailable, and says why", async () => {
    draw({ unavailable: "sprintHasNoDates" });
    expect(screen.getByText(nothingToPublishLine())).toBeInTheDocument();
    for (const name of ["Publish to Confluence", "Export a spreadsheet", "Export a deck"]) {
      expect(screen.getByRole("button", { name })).toBeDisabled();
    }
    expect(bindings.PublishSprintReport).not.toHaveBeenCalled();
    expect(bindings.ExportSprintReportXLSX).not.toHaveBeenCalled();
    expect(bindings.ExportSprintReportPPTX).not.toHaveBeenCalled();
  });

  it("names the page and the reason when Confluence refuses the write", async () => {
    bindings.PublishSprintReport.mockRejectedValue(
      new Error(`the Confluence page "Sprint 11 · Report" could not be created in TEAM: 403 Forbidden`),
    );
    draw();
    await userEvent.click(screen.getByRole("button", { name: "Publish to Confluence" }));
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Sprint 11 · Report");
    expect(alert).toHaveTextContent("403 Forbidden");
  });
});

// Issue #86. The three outputs each take a moment and the view said almost
// nothing while one ran: one shared `running` string, and a `run` that wiped
// the previous outcome whichever button produced it.
describe("the publisher ribbon", () => {
  /** The ribbon cell for one publisher, by its accessible name. */
  const cell = (name: RegExp) => screen.getByRole("listitem", { name });

  // Issue #90 kept an idle cell reading "Ready" so that a blank cell beside
  // two filled ones would not read as a problem. That argues for keeping the
  // idle cells once the ribbon is live; it never argued for mounting three of
  // them before anything has run. Three cells saying "Ready" under three
  // enabled buttons state what the buttons already state, and they cost a
  // whole row of a frame whose velocity panel pays for every row above it.
  it("shows no ribbon at all until a publisher has been used", () => {
    draw();
    expect(screen.queryByRole("list")).toBeNull();
    expect(screen.queryByText(publisherStatusWord("idle"))).toBeNull();
  });

  it("shows all three cells, idle ones included, once any publisher has run", async () => {
    bindings.ExportSprintReportXLSX.mockResolvedValue("/tmp/a.xlsx");
    draw();
    await userEvent.click(screen.getByRole("button", { name: /spreadsheet/i }));
    await waitFor(() => expect(cell(/spreadsheet/i)).toHaveTextContent(publisherStatusWord("done")));
    // The other two keep their word rather than leaving a gap beside it.
    expect(cell(/Confluence/i)).toHaveTextContent(publisherStatusWord("idle"));
    expect(cell(/deck/i)).toHaveTextContent(publisherStatusWord("idle"));
  });

  it("marks only the publisher that is running", async () => {
    let release: (v: string) => void = () => {};
    bindings.ExportSprintReportXLSX.mockReturnValue(new Promise<string>((r) => { release = r; }));
    draw();
    await userEvent.click(screen.getByRole("button", { name: /spreadsheet/i }));
    await waitFor(() => expect(cell(/spreadsheet/i)).toHaveTextContent(publisherStatusWord("running")));
    expect(cell(/Confluence/i)).toHaveTextContent(publisherStatusWord("idle"));
    expect(cell(/deck/i)).toHaveTextContent(publisherStatusWord("idle"));
    release("/tmp/a.xlsx");
    await waitFor(() => expect(cell(/spreadsheet/i)).toHaveTextContent(publisherStatusWord("done")));
  });

  // The bug: run() cleared done and failure for every publisher, so the one
  // confirmation naming a page written to Confluence vanished the moment the
  // user also exported a file.
  it("keeps a publish confirmation when a later export runs", async () => {
    bindings.PublishSprintReport.mockResolvedValue({ title: "Sprint 11 · Report", pageId: "9" });
    bindings.ExportSprintReportPPTX.mockResolvedValue("/tmp/a.pptx");
    draw();
    await userEvent.click(screen.getByRole("button", { name: /Confluence/i }));
    await screen.findByText(publishedLine("Sprint 11 · Report"));
    await userEvent.click(screen.getByRole("button", { name: /deck/i }));
    await screen.findByText(savedLine("/tmp/a.pptx"));
    expect(screen.getByText(publishedLine("Sprint 11 · Report"))).toBeInTheDocument();
  });

  it("returns a cancelled export to idle rather than calling it done or failed", async () => {
    bindings.ExportSprintReportXLSX.mockResolvedValue("");
    draw();
    await userEvent.click(screen.getByRole("button", { name: /spreadsheet/i }));
    // Nothing was written, so all three are idle again and the ribbon has
    // nothing left to report. It goes away rather than leaving three cells
    // saying "Ready" behind it.
    await waitFor(() => expect(screen.queryByRole("list")).toBeNull());
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByText(/Saved to/)).toBeNull();
  });

  it("shows one publisher's failure without hiding the other two", async () => {
    bindings.PublishSprintReport.mockRejectedValue(new Error("Confluence said no"));
    bindings.ExportSprintReportPPTX.mockResolvedValue("/tmp/a.pptx");
    draw();
    await userEvent.click(screen.getByRole("button", { name: /deck/i }));
    await screen.findByText(savedLine("/tmp/a.pptx"));
    await userEvent.click(screen.getByRole("button", { name: /Confluence/i }));
    // Scoped to the cell: the reason is deliberately in the live region too,
    // because a screen reader user needs to hear why it failed, so an
    // unscoped query finds it twice.
    await waitFor(() => expect(cell(/Confluence/i)).toHaveTextContent(/Confluence said no/));
    expect(cell(/Confluence/i)).toHaveTextContent(publisherStatusWord("failed"));
    expect(cell(/deck/i)).toHaveTextContent(publisherStatusWord("done"));
  });

  // Issue #95. The charts go on the page as attachments, and one Confluence
  // refuses must not read as a failed publish: the page is there and its
  // tables are on it.
  it("says a chart did not reach the page without calling the publish failed", async () => {
    bindings.PublishSprintReport.mockResolvedValue({
      title: "Sprint 11 · Report",
      pageId: "9",
      warning: "Not every chart reached the page: chart-1.png: 413 Payload Too Large.",
    });
    draw();
    await userEvent.click(screen.getByRole("button", { name: /Confluence/i }));
    await waitFor(() => expect(cell(/Confluence/i)).toHaveTextContent(publisherStatusWord("warned")));
    expect(cell(/Confluence/i)).toHaveTextContent(/chart-1\.png/);
    expect(cell(/Confluence/i)).toHaveTextContent(/Sprint 11 · Report/);
    expect(cell(/Confluence/i)).not.toHaveTextContent(publisherStatusWord("failed"));
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("announces a state change rather than only colouring it", async () => {
    bindings.ExportSprintReportXLSX.mockResolvedValue("/tmp/a.xlsx");
    draw();
    const live = screen.getByRole("status");
    await userEvent.click(screen.getByRole("button", { name: /spreadsheet/i }));
    await waitFor(() => expect(live).toHaveTextContent(/spreadsheet/i));
  });
});

// The done agreement section is the question a sprint review argues about,
// and it reaches the page, the spreadsheet and the deck out of the one
// document: no renderer in Go knows the section exists.
describe("the done agreement on a published report", () => {
  const body = (...items: string[]) =>
    `<ac:task-list>${items
      .map((i) => `<ac:task><ac:task-status>incomplete</ac:task-status><ac:task-body>${i}</ac:task-body></ac:task>`)
      .join("")}</ac:task-list>`;

  // A board with a standing agreement of two items, one of which one of the
  // sprint's two cards was ticked for.
  function withAgreement(): Partial<SprintReport> {
    bindings.ListRitualDocuments.mockResolvedValue([
      { sprintId: 0, ritualType: DONE_AGREEMENT, body: body("Unit tests pass", "Docs updated") },
    ]);
    bindings.DoneAgreementTicks.mockResolvedValue({ "PLAT-1": ["Unit tests pass"] });
    return { series: { ...report().series, issues: ["PLAT-1", "PLAT-2"] } };
  }

  it("publishes a count per card against each item the agreement states", async () => {
    bindings.PublishSprintReport.mockResolvedValue({ title: "Sprint 11 · Report", pageId: "1234" });
    draw(withAgreement());
    await userEvent.click(screen.getByRole("button", { name: "Publish to Confluence" }));
    await waitFor(() => expect(bindings.PublishSprintReport).toHaveBeenCalled());
    const doc = bindings.PublishSprintReport.mock.calls[0][3];
    expect(doc.sections[doc.sections.length - 1].heading).toBe("Done agreement");
    expect(doc.sections[doc.sections.length - 1].table.rows).toEqual([
      ["Unit tests pass", "1 of 2"],
      ["Docs updated", "0 of 2"],
    ]);
    // The ticks are read once, for the cards the report was built from, and
    // the documents for the sprint the report is about.
    expect(bindings.DoneAgreementTicks).toHaveBeenCalledWith("p1", 1, ["PLAT-1", "PLAT-2"]);
    expect(bindings.ListRitualDocuments).toHaveBeenCalledWith("p1", 1, 11);
  });

  it("carries the same section into the spreadsheet and the deck", async () => {
    bindings.ExportSprintReportXLSX.mockResolvedValue("/tmp/a.xlsx");
    bindings.ExportSprintReportPPTX.mockResolvedValue("/tmp/a.pptx");
    draw(withAgreement());
    await userEvent.click(screen.getByRole("button", { name: /spreadsheet/i }));
    await screen.findByText(savedLine("/tmp/a.xlsx"));
    await userEvent.click(screen.getByRole("button", { name: /deck/i }));
    await screen.findByText(savedLine("/tmp/a.pptx"));
    const sheet = bindings.ExportSprintReportXLSX.mock.calls[0][0];
    const deck = bindings.ExportSprintReportPPTX.mock.calls[0][0];
    expect(sheet.sections[3].table.rows).toEqual([
      ["Unit tests pass", "1 of 2"],
      ["Docs updated", "0 of 2"],
    ]);
    expect(deck.sections[3]).toEqual(sheet.sections[3]);
  });

  it("refuses the publish rather than writing a page the section is missing from", async () => {
    bindings.ListRitualDocuments.mockRejectedValue(new Error("the store is not open"));
    draw({ series: { ...report().series, issues: ["PLAT-1"] } });
    await userEvent.click(screen.getByRole("button", { name: "Publish to Confluence" }));
    await waitFor(() =>
      expect(screen.getByRole("listitem", { name: /Confluence/i })).toHaveTextContent(/the store is not open/),
    );
    expect(bindings.PublishSprintReport).not.toHaveBeenCalled();
  });
});
