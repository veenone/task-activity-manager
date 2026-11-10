import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DialogProvider } from "@agile-suite/core";
import type { query } from "../wailsjs/go/models";
import App from "./App";
import { ResultsTable } from "./ResultsTable";
import * as api from "../wailsjs/go/main/App";

vi.mock("../wailsjs/go/main/App", () => ({
  StartupError: vi.fn(),
  ListProfiles: vi.fn(),
  DefaultProfile: () => Promise.resolve(""),
  CreateProfile: vi.fn(),
  DeleteProfile: vi.fn(),
  RunQuery: vi.fn(),
  ListQueries: vi.fn(),
  SaveQuery: vi.fn(),
  DeleteQuery: vi.fn(),
  LastRun: vi.fn(),
  Fields: vi.fn(),
  SetFields: vi.fn(),
  ListCases: vi.fn(),
  OpenCases: vi.fn(),
}));

function issue(key: string) {
  return {
    key,
    project: "A",
    projectName: "",
    type: "Bug",
    summary: `Summary of ${key}`,
    status: "Open",
    priority: "High",
    updated: "2026-10-02T00:00:00.000+0000",
    matched: true,
  } as unknown as query.Issue;
}

const issues = ["A-1", "A-2", "A-3"].map(issue);
const rowKeys = () =>
  within(screen.getByRole("table", { name: "Query results" }))
    .getAllByRole("row")
    .slice(1)
    .map((r) => (r as HTMLTableRowElement).cells[1].textContent);

describe("ResultsTable cases", () => {
  it("opens the selected rows as cases and clears the selection", async () => {
    const user = userEvent.setup();
    const onOpenCases = vi.fn().mockResolvedValue(true);
    render(<ResultsTable issues={issues} caseKeys={new Set()} onOpenCases={onOpenCases} />);
    expect(screen.queryByRole("button", { name: /as cases/ })).not.toBeInTheDocument();
    await user.click(screen.getByRole("checkbox", { name: "Select A-1" }));
    await user.click(screen.getByRole("checkbox", { name: "Select A-3" }));
    await user.click(screen.getByRole("button", { name: "Open 2 as cases" }));
    expect(onOpenCases).toHaveBeenCalledWith(["A-1", "A-3"]);
    expect(screen.getByRole("checkbox", { name: "Select A-1" })).not.toBeChecked();
  });

  it("keeps the selection when the cases could not be opened", async () => {
    const user = userEvent.setup();
    render(<ResultsTable issues={issues} caseKeys={new Set()} onOpenCases={vi.fn().mockResolvedValue(false)} />);
    await user.click(screen.getByRole("checkbox", { name: "Select A-2" }));
    await user.click(screen.getByRole("button", { name: "Open 1 as cases" }));
    expect(screen.getByRole("checkbox", { name: "Select A-2" })).toBeChecked();
  });

  it("selects every row on the page from the header", async () => {
    const user = userEvent.setup();
    render(<ResultsTable issues={issues} caseKeys={new Set()} onOpenCases={vi.fn()} />);
    await user.click(screen.getByRole("checkbox", { name: "Select all on this page" }));
    expect(screen.getByRole("button", { name: "Open 3 as cases" })).toBeVisible();
  });

  it("marks rows with a case and filters on it", async () => {
    const user = userEvent.setup();
    render(<ResultsTable issues={issues} caseKeys={new Set(["A-2"])} onOpenCases={vi.fn()} />);
    const a2 = screen.getByRole("checkbox", { name: "Select A-2" }).closest("tr")!;
    expect(within(a2).getByText("Case")).toBeVisible();
    await user.selectOptions(screen.getByLabelText("Case"), "with");
    expect(rowKeys()).toEqual(["A-2"]);
    await user.selectOptions(screen.getByLabelText("Case"), "without");
    expect(rowKeys()).toEqual(["A-1", "A-3"]);
  });
});

describe("cases in the app", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(api.StartupError).mockResolvedValue("");
    vi.mocked(api.ListProfiles).mockResolvedValue([{ id: "p1", name: "Q", jiraUrl: "demo" }] as never);
    vi.mocked(api.ListQueries).mockResolvedValue([]);
    vi.mocked(api.Fields).mockResolvedValue({ productBase: "", clientVisibility: "", clientImpact: "" } as never);
    vi.mocked(api.ListCases).mockResolvedValue([{ id: 1, key: "Z-9", state: "open", notes: "", openedAt: "" }] as never);
    vi.mocked(api.RunQuery).mockResolvedValue({
      result: { total: 3, issues, sets: [], truncated: false, linkError: "", linksTruncated: false, needsConfirm: false },
      runAt: "",
      added: null,
      dropped: null,
    } as never);
  });

  it("shows the Cases tab before any run", async () => {
    render(
      <DialogProvider>
        <App />
      </DialogProvider>,
    );
    const user = userEvent.setup();
    await user.click(within(await screen.findByRole("navigation", { name: "Views" })).getByRole("button", { name: "Cases" }));
    expect(await screen.findByRole("button", { name: /^Z-9/ })).toBeVisible();
    expect(api.RunQuery).not.toHaveBeenCalled();
  });

  it("stores the cases and names the ones this run did not match", async () => {
    vi.mocked(api.OpenCases).mockResolvedValue([
      { id: 1, key: "Z-9", state: "open", notes: "", openedAt: "" },
      { id: 2, key: "A-1", state: "open", notes: "", openedAt: "" },
    ] as never);
    render(
      <DialogProvider>
        <App />
      </DialogProvider>,
    );
    const user = userEvent.setup();
    await user.type(await screen.findByLabelText("JQL"), "x");
    await user.click(screen.getByRole("button", { name: "Run" }));
    expect(await screen.findByText("1 case is for an issue this run did not match: Z-9")).toBeVisible();

    await user.click(screen.getByRole("checkbox", { name: "Select A-1" }));
    await user.click(screen.getByRole("button", { name: "Open 1 as cases" }));
    expect(api.OpenCases).toHaveBeenCalledWith("p1", ["A-1"]);
    const a1 = screen.getByRole("checkbox", { name: "Select A-1" }).closest("tr")!;
    expect(await within(a1).findByText("Case")).toBeVisible();
  });
});
