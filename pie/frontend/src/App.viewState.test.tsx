import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DialogProvider } from "@agile-suite/core";
import App from "./App";
import * as api from "../wailsjs/go/main/App";

vi.mock("../wailsjs/go/main/App", () => ({
  StartupError: vi.fn(),
  ListProfiles: vi.fn(),
  DefaultProfile: vi.fn(),
  RunQuery: vi.fn(),
  ListQueries: vi.fn(),
  LastRun: vi.fn(),
  Fields: vi.fn(),
  ListCases: vi.fn(),
  Baselines: vi.fn(),
}));

function issue(key: string, status: string) {
  const project = key.split("-")[0];
  return { key, project, projectName: project, type: "Bug", summary: `Summary of ${key}`, status, priority: "High", updated: "2026-11-05", created: "2026-11-01", matched: true };
}

const run = {
  result: {
    total: 3,
    truncated: false,
    issues: [issue("A-1", "Open"), issue("A-2", "Closed"), issue("B-1", "Open")],
    sets: [{ keys: ["A-1", "B-1"], projects: ["A", "B"], outOfStep: true }],
  },
};

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.StartupError).mockResolvedValue("");
  vi.mocked(api.DefaultProfile).mockResolvedValue("");
  vi.mocked(api.ListProfiles).mockResolvedValue([{ id: "p1", name: "Quality", jiraUrl: "https://jira.example.com" }] as never);
  vi.mocked(api.ListQueries).mockResolvedValue([]);
  vi.mocked(api.Fields).mockResolvedValue({ productBase: "", clientVisibility: "", clientImpact: "" } as never);
  vi.mocked(api.ListCases).mockResolvedValue([]);
  vi.mocked(api.Baselines).mockResolvedValue([]);
  vi.mocked(api.RunQuery).mockResolvedValue(run as never);
});

async function start() {
  render(
    <DialogProvider>
      <App />
    </DialogProvider>,
  );
  const user = userEvent.setup();
  await user.type(await screen.findByLabelText("JQL"), "project = A");
  await user.click(screen.getByRole("button", { name: "Run" }));
  await screen.findByRole("table", { name: "Query results" });
  return user;
}

// Each case drives the whole app through a run and two tab switches, which
// outlasts the default limit when other suites share the machine.
const SLOW = 30_000;
const tab = (name: string) => within(screen.getByRole("navigation", { name: "Views" })).getByRole("button", { name });
const keys = () =>
  within(screen.getByRole("table", { name: "Query results" }))
    .getAllByRole("row")
    .slice(1)
    .map((r) => (r as HTMLTableRowElement).cells[1].textContent);

it("keeps the Results filters, search and selection across a tab switch", async () => {
  const user = await start();
  await user.selectOptions(screen.getByLabelText("Status"), "Open");
  await user.type(screen.getByLabelText("Search"), "Summary");
  await user.click(screen.getByRole("checkbox", { name: "Select B-1" }));
  await user.click(screen.getByRole("button", { name: "A-1" }));
  await user.click(tab("Trace"));
  await user.click(tab("Results"));
  expect(screen.getByLabelText("Status")).toHaveValue("Open");
  expect(screen.getByLabelText("Search")).toHaveValue("Summary");
  expect(keys()).toEqual(["A-1", "B-1"]);
  expect(screen.getByRole("checkbox", { name: "Select B-1" })).toBeChecked();
  expect(screen.getByRole("button", { name: "Open 1 as cases" })).toBeVisible();
  expect(screen.getByRole("row", { name: /A-1/ })).toHaveClass("pie-row-open");
}, SLOW);

it("keeps the Project map's sub-view and filters across a tab switch", async () => {
  const user = await start();
  await user.click(tab("Project map"));
  await user.click(await screen.findByRole("button", { name: "Tree" }));
  await user.type(screen.getByPlaceholderText("Part of a key"), "B-");
  await user.click(tab("Results"));
  await user.click(tab("Project map"));
  expect(await screen.findByRole("button", { name: "Tree" })).toHaveAttribute("aria-pressed", "true");
  expect(screen.getByPlaceholderText("Part of a key")).toHaveValue("B-");
}, SLOW);

it("keeps the Link map's sub-view across a tab switch", async () => {
  const user = await start();
  await user.click(tab("Link map"));
  await user.click(within(await screen.findByRole("group", { name: "Link view" })).getByRole("button", { name: "Flow" }));
  await user.click(tab("Trace"));
  await user.click(tab("Link map"));
  expect(within(screen.getByRole("group", { name: "Link view" })).getByRole("button", { name: "Flow" })).toHaveAttribute("aria-pressed", "true");
}, SLOW);
