import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { SprintReport } from "../api";
import { nothingToPublishLine, publishedLine, publisherStatusWord, savedLine } from "../lib/reportText";
import { ReportOutputs } from "./ReportOutputs";

const bindings = vi.hoisted(() => ({
  PublishSprintReport: vi.fn(),
  ExportSprintReportXLSX: vi.fn(),
  ExportSprintReportPPTX: vi.fn(),
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
    },
    velocity: [
      { sprintId: 11, sprintName: "Sprint 11", unit: "points", unitReason: "", committed: 34, completed: 29, truncated: false },
    ],
    builtAt: "2026-03-16T09:00:00Z",
    unavailable: "",
    ...over,
  };
}

function draw(over: Partial<SprintReport> = {}) {
  render(<ReportOutputs profileId="p1" boardId={1} report={report(over)} live={false} />);
}

beforeEach(() => {
  bindings.PublishSprintReport.mockReset();
  bindings.ExportSprintReportXLSX.mockReset();
  bindings.ExportSprintReportPPTX.mockReset();
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

  it("starts with all three idle", () => {
    draw();
    for (const name of [/Confluence/i, /spreadsheet/i, /deck/i]) {
      expect(cell(name)).toHaveTextContent(publisherStatusWord("idle"));
    }
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
    await waitFor(() => expect(cell(/spreadsheet/i)).toHaveTextContent(publisherStatusWord("idle")));
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

  it("announces a state change rather than only colouring it", async () => {
    bindings.ExportSprintReportXLSX.mockResolvedValue("/tmp/a.xlsx");
    draw();
    const live = screen.getByRole("status");
    await userEvent.click(screen.getByRole("button", { name: /spreadsheet/i }));
    await waitFor(() => expect(live).toHaveTextContent(/spreadsheet/i));
  });
});
