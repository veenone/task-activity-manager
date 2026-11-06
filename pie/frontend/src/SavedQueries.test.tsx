import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DialogProvider } from "@agile-suite/core";
import App from "./App";
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
}));

const weekly = { id: 7, name: "Weekly", jql: "category = X", runAt: "2026-11-06T12:00:00Z" };

function rows(...keys: string[]) {
  return {
    total: keys.length,
    truncated: false,
    linkError: "",
    linksTruncated: false,
    needsConfirm: false,
    sets: [],
    issues: keys.map((key) => ({ key, project: "A", summary: `Summary of ${key}`, updated: "", matched: true, links: [] })),
  };
}

function show() {
  render(
    <DialogProvider>
      <App />
    </DialogProvider>,
  );
  return userEvent.setup();
}

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.StartupError).mockResolvedValue("");
  vi.mocked(api.ListProfiles).mockResolvedValue([{ id: "p1", name: "Q", jiraUrl: "demo" }] as never);
  vi.mocked(api.ListQueries).mockResolvedValue([weekly] as never);
});

it("opens a saved query's last run without asking Jira", async () => {
  vi.mocked(api.LastRun).mockResolvedValue({ result: rows("A-1"), runAt: weekly.runAt, added: null, dropped: null } as never);
  const user = show();
  await screen.findByRole("option", { name: "Weekly" });
  await user.selectOptions(screen.getByLabelText("Saved query"), "Weekly");

  expect(api.LastRun).toHaveBeenCalledWith("p1", 7);
  expect(await screen.findByRole("cell", { name: "A-1" })).toBeVisible();
  expect(screen.getByLabelText("JQL")).toHaveValue("category = X");
  expect(screen.getByText(/^Last run 2026-11-06 \d\d:\d\d$/)).toBeVisible();
  expect(api.RunQuery).not.toHaveBeenCalled();
});

it("drops the rows on screen when a saved query's run cannot be read", async () => {
  vi.mocked(api.RunQuery).mockResolvedValue({ result: rows("A-1"), runAt: "", added: null, dropped: null } as never);
  vi.mocked(api.LastRun).mockRejectedValue("read the stored run: unexpected end of JSON input");
  const user = show();
  await user.type(await screen.findByLabelText("JQL"), "x");
  await user.click(screen.getByRole("button", { name: "Run" }));
  await screen.findByRole("cell", { name: "A-1" });

  await user.selectOptions(screen.getByLabelText("Saved query"), "Weekly");
  expect(await screen.findByRole("alert")).toHaveTextContent("read the stored run");
  expect(screen.queryByRole("cell", { name: "A-1" })).not.toBeInTheDocument();
});

it("saves the JQL under a name and selects it", async () => {
  vi.mocked(api.ListQueries).mockResolvedValueOnce([]);
  vi.mocked(api.SaveQuery).mockResolvedValue({ ...weekly, runAt: "" } as never);
  const user = show();
  await user.type(await screen.findByLabelText("JQL"), "category = X");
  await user.click(screen.getByRole("button", { name: "Save as..." }));
  const dialog = await screen.findByRole("dialog");
  await user.type(within(dialog).getByRole("textbox"), "Weekly");
  await user.click(within(dialog).getByRole("button", { name: "Save" }));

  expect(api.SaveQuery).toHaveBeenCalledWith("p1", "Weekly", "category = X");
  expect(await screen.findByRole("option", { name: "Weekly", selected: true })).toBeInTheDocument();
});

it("stores a saved query's run and says what changed since the previous one", async () => {
  vi.mocked(api.LastRun).mockResolvedValue({ result: rows("A-1"), runAt: weekly.runAt, added: null, dropped: null } as never);
  vi.mocked(api.RunQuery).mockResolvedValue({
    result: rows("A-1", "B-1"),
    runAt: "2026-11-06T13:00:00Z",
    added: ["B-1"],
    dropped: ["C-1", "D-1"],
  } as never);
  const user = show();
  await screen.findByRole("option", { name: "Weekly" });
  await user.selectOptions(screen.getByLabelText("Saved query"), "Weekly");
  await screen.findByRole("cell", { name: "A-1" });
  await user.click(screen.getByRole("button", { name: "Run" }));

  expect(api.RunQuery).toHaveBeenCalledWith("p1", 7, "category = X", false);
  expect(await screen.findByText("Since the previous run: 1 added, 2 dropped")).toBeVisible();
  expect(screen.getByText("Added: B-1")).toBeInTheDocument();
  expect(screen.getByText("Dropped: C-1, D-1")).toBeInTheDocument();
});

it("runs unsaved JQL without storing it", async () => {
  vi.mocked(api.RunQuery).mockResolvedValue({ result: rows("A-1"), runAt: "", added: null, dropped: null } as never);
  const user = show();
  await user.type(await screen.findByLabelText("JQL"), "x");
  await user.click(screen.getByRole("button", { name: "Run" }));
  expect(api.RunQuery).toHaveBeenCalledWith("p1", 0, "x", false);
  expect(await screen.findByRole("cell", { name: "A-1" })).toBeVisible();
  expect(screen.queryByText(/^Since the previous run/)).not.toBeInTheDocument();
});

it("asks before a large fetch and fetches nothing more when declined", async () => {
  const large = { ...rows(), total: 2500, needsConfirm: true };
  vi.mocked(api.RunQuery).mockResolvedValue({ result: large, runAt: "", added: null, dropped: null } as never);
  const user = show();
  await user.type(await screen.findByLabelText("JQL"), "x");
  await user.click(screen.getByRole("button", { name: "Run" }));

  const ask = await screen.findByRole("alertdialog");
  expect(ask).toHaveTextContent("This query matches 2500 issues.");
  await user.click(within(ask).getAllByRole("button", { name: "Cancel" }).at(-1)!);
  expect(api.RunQuery).toHaveBeenCalledTimes(1);
  expect(screen.queryByRole("table")).not.toBeInTheDocument();
});

it("fetches a large query once the analyst agrees", async () => {
  const large = { ...rows(), total: 2500, needsConfirm: true };
  vi.mocked(api.RunQuery)
    .mockResolvedValueOnce({ result: large, runAt: "", added: null, dropped: null } as never)
    .mockResolvedValueOnce({ result: rows("A-1"), runAt: "", added: null, dropped: null } as never);
  const user = show();
  await user.type(await screen.findByLabelText("JQL"), "x");
  await user.click(screen.getByRole("button", { name: "Run" }));
  await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Fetch" }));

  expect(api.RunQuery).toHaveBeenLastCalledWith("p1", 0, "x", true);
  expect(await screen.findByRole("cell", { name: "A-1" })).toBeVisible();
});

it("deletes a saved query only after confirming", async () => {
  vi.mocked(api.LastRun).mockResolvedValue({ result: rows(), runAt: "", added: null, dropped: null } as never);
  const user = show();
  await screen.findByRole("option", { name: "Weekly" });
  await user.selectOptions(screen.getByLabelText("Saved query"), "Weekly");
  await user.click(screen.getByRole("button", { name: "Delete query" }));
  expect(api.DeleteQuery).not.toHaveBeenCalled();

  vi.mocked(api.ListQueries).mockResolvedValue([]);
  await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Delete" }));
  expect(api.DeleteQuery).toHaveBeenCalledWith("p1", 7);
  expect(await screen.findByRole("option", { name: "Unsaved query", selected: true })).toBeInTheDocument();
  expect(screen.queryByRole("option", { name: "Weekly" })).not.toBeInTheDocument();
});
