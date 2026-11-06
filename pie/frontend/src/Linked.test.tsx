import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DialogProvider } from "@agile-suite/core";
import App from "./App";
import { ageText } from "./linked";
import * as api from "../wailsjs/go/main/App";

vi.mock("../wailsjs/go/main/App", () => ({
  StartupError: vi.fn(),
  ListProfiles: vi.fn(),
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

function issue(key: string, status: string, statusCategory: string, links: [string, string][] = [], matched = true) {
  return {
    key,
    project: key.split("-")[0],
    type: "Bug",
    summary: `Summary of ${key}`,
    status,
    priority: "High",
    updated: "2026-11-01T10:00:00.000+0700",
    created: "2026-10-01T10:00:00.000+0700",
    resolved: "",
    statusCategory,
    matched,
    links: links.map(([words, k]) => ({ key: k, type: "Cloners", words })),
  };
}

// Two sets: A-1 with its clone B-1 (out of step), and A-2 with C-2 (in step).
// C-2 came only through a link.
const result = {
  total: 3,
  truncated: false,
  linkError: "",
  linksTruncated: false,
  issues: [
    issue("A-1", "Closed", "done", [["is cloned by", "B-1"]]),
    issue("B-1", "Open", "new", [["clones", "A-1"]]),
    issue("A-2", "Closed", "done", [["is cloned by", "C-2"]]),
    issue("C-2", "Rejected", "done", [["clones", "A-2"]], false),
  ],
  sets: [
    { keys: ["A-1", "B-1"], projects: ["A", "B"], outOfStep: true },
    { keys: ["A-2", "C-2"], projects: ["A", "C"], outOfStep: false },
  ],
};

async function runAndOpen(view: "Trace" | "Project map", res: object = result) {
  vi.mocked(api.RunQuery).mockResolvedValue({ result: res } as never);
  render(
    <DialogProvider>
      <App />
    </DialogProvider>,
  );
  const user = userEvent.setup();
  await user.type(await screen.findByLabelText("JQL"), "q");
  await user.click(screen.getByRole("button", { name: "Run" }));
  await user.click(await screen.findByRole("button", { name: view }));
  return user;
}

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.Fields).mockResolvedValue({ productBase: "", clientVisibility: "", clientImpact: "" } as never);
  vi.mocked(api.ListCases).mockResolvedValue([]);
  vi.mocked(api.StartupError).mockResolvedValue("");
  vi.mocked(api.ListQueries).mockResolvedValue([]);
  vi.mocked(api.ListProfiles).mockResolvedValue([{ id: "p1", name: "Q", jiraUrl: "demo" }] as never);
});

it("lists only the issues the query matched in the results table", async () => {
  await runAndOpen("Trace");
  await userEvent.setup().click(screen.getByRole("button", { name: "Results" }));
  const table = screen.getByRole("table", { name: "Query results" });
  expect(within(table).getByRole("cell", { name: "A-2" })).toBeVisible();
  expect(within(table).queryByRole("cell", { name: "C-2" })).not.toBeInTheDocument();
});

describe("project map", () => {
  it("gives each set a row, each project a column, and a dash where a project has none", async () => {
    await runAndOpen("Project map");
    const map = screen.getByRole("table", { name: "Project map" });
    const headers = within(map).getAllByRole("columnheader").map((h) => h.textContent);
    expect(headers).toEqual(["Linked set", "A", "B", "C", "State"]);

    const first = within(map).getByRole("rowheader", { name: "A-1" }).closest("tr")!;
    const cells = within(first).getAllByRole("cell").map((c) => c.textContent);
    expect(cells).toEqual(["A-1 Closed", "B-1 Open", "-", "Out of step"]);
    const second = within(map).getByRole("rowheader", { name: "A-2" }).closest("tr")!;
    expect(within(second).getAllByRole("cell").at(-1)).toHaveTextContent("In step");
  });

  it("narrows to sets out of step", async () => {
    const user = await runAndOpen("Project map");
    await user.click(screen.getByLabelText("Out of step only"));
    const map = screen.getByRole("table", { name: "Project map" });
    expect(within(map).getByRole("rowheader", { name: "A-1" })).toBeVisible();
    expect(within(map).queryByRole("rowheader", { name: "A-2" })).not.toBeInTheDocument();
  });

  it("opens a key's set in Trace", async () => {
    const user = await runAndOpen("Project map");
    await user.click(screen.getByRole("button", { name: "C-2 Rejected" }));
    expect(screen.getByRole("heading", { name: "Linked set of A-2" })).toBeVisible();
  });
});

describe("trace", () => {
  it("shows the selected set by project, with each link worded from its own side", async () => {
    await runAndOpen("Trace");
    const detail = screen.getByRole("region", { name: "Linked set of A-1" });
    const a = within(detail).getByRole("group", { name: "Project A" });
    expect(within(a).getByText("is cloned by B-1")).toBeVisible();
    const b = within(detail).getByRole("group", { name: "Project B" });
    expect(within(b).getByText("clones A-1")).toBeVisible();
  });

  it("says when an issue came only through a link", async () => {
    const user = await runAndOpen("Trace");
    await user.click(screen.getByRole("button", { name: /^A-2/ }));
    const c = within(screen.getByRole("region", { name: "Linked set of A-2" })).getByRole("group", { name: "Project C" });
    expect(within(c).getByText("Not in the query")).toBeVisible();
  });

  it("filters the sets by project and by out of step", async () => {
    const user = await runAndOpen("Trace");
    const list = screen.getByRole("list", { name: "Linked sets" });
    await user.selectOptions(screen.getByLabelText("Project"), "C");
    expect(within(list).queryByRole("button", { name: /^A-1/ })).not.toBeInTheDocument();
    expect(within(list).getByRole("button", { name: /^A-2/ })).toBeVisible();

    await user.selectOptions(screen.getByLabelText("Project"), "");
    await user.click(screen.getByLabelText("Out of step only"));
    expect(within(list).getByRole("button", { name: /^A-1/ })).toBeVisible();
    expect(within(list).queryByRole("button", { name: /^A-2/ })).not.toBeInTheDocument();
  });
});

it("keeps the rows and says why when the links could not all be followed", async () => {
  await runAndOpen("Trace", { ...result, linkError: "An issue with key 'Z-9' does not exist." });
  expect(screen.getByRole("alert")).toHaveTextContent("Linked issues could not all be fetched: An issue with key 'Z-9' does not exist.");
  expect(screen.getByRole("region", { name: "Linked set of A-1" })).toBeVisible();
});

it("counts an issue's age in whole days from Jira's date", () => {
  const now = new Date("2026-11-06T09:00:00Z");
  expect(ageText("2026-11-05T23:30:00.000+0700", now)).toBe("1 day old");
  expect(ageText("2026-10-07T10:00:00.000+0700", now)).toBe("30 days old");
  expect(ageText("", now)).toBe("");
});
