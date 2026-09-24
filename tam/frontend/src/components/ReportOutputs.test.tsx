import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { SprintReport } from "../api";
import { nothingToPublishLine, publishedLine, savedLine } from "../lib/reportText";
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
