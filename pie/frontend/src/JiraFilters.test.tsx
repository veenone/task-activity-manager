import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DialogProvider } from "@agile-suite/core";
import App from "./App";
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
  JiraFilters: vi.fn(),
  JiraFilter: vi.fn(),
}));

const weekly = { id: "4200", name: "Client reports, open", jql: "issuetype = Bug", owner: { name: "demo" } };

async function openDialog() {
  render(
    <DialogProvider>
      <App />
    </DialogProvider>,
  );
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: "From a Jira filter" }));
  return { user, dialog: await screen.findByRole("dialog") };
}

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.StartupError).mockResolvedValue("");
  vi.mocked(api.ListProfiles).mockResolvedValue([{ id: "p1", name: "Q", jiraUrl: "demo" }] as never);
  vi.mocked(api.ListQueries).mockResolvedValue([]);
  vi.mocked(api.Fields).mockResolvedValue({ productBase: "", clientVisibility: "", clientImpact: "" } as never);
  vi.mocked(api.ListCases).mockResolvedValue([]);
  vi.mocked(api.JiraFilters).mockResolvedValue([weekly] as never);
});

it("puts a starred filter into the JQL box without running it", async () => {
  const { user, dialog } = await openDialog();
  expect(api.JiraFilters).toHaveBeenCalledWith("p1");
  expect(await within(dialog).findByText("issuetype = Bug")).toBeVisible();
  await user.click(within(dialog).getByRole("button", { name: "Client reports, open" }));
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(screen.getByLabelText("JQL")).toHaveValue("filter = 4200");
  expect(screen.getByLabelText("Saved query")).toHaveValue("0");
  expect(api.RunQuery).not.toHaveBeenCalled();
});

it("reaches a filter nobody starred by its id", async () => {
  vi.mocked(api.JiraFilter).mockResolvedValue({ ...weekly, id: "4300" } as never);
  const { user, dialog } = await openDialog();
  await user.type(within(dialog).getByLabelText("Filter id"), "4300");
  await user.click(within(dialog).getByRole("button", { name: "Open" }));
  expect(api.JiraFilter).toHaveBeenCalledWith("p1", "4300");
  expect(screen.getByLabelText("JQL")).toHaveValue("filter = 4300");
  // The id form sits inside the query form in React's tree; its submit must not run the query.
  expect(api.RunQuery).not.toHaveBeenCalled();
});

it("opens an id on Enter without running the query", async () => {
  vi.mocked(api.JiraFilter).mockResolvedValue({ ...weekly, id: "4300" } as never);
  const { user, dialog } = await openDialog();
  await user.type(within(dialog).getByLabelText("Filter id"), "4300{Enter}");
  expect(screen.getByLabelText("JQL")).toHaveValue("filter = 4300");
  expect(api.RunQuery).not.toHaveBeenCalled();
});

it("keeps the dialog open with Jira's reason for a refused id", async () => {
  vi.mocked(api.JiraFilter).mockRejectedValue("Jira has no filter 9 that this account can see.");
  const { user, dialog } = await openDialog();
  await user.type(within(dialog).getByLabelText("Filter id"), "9");
  await user.click(within(dialog).getByRole("button", { name: "Open" }));
  expect(await within(dialog).findByText("Jira has no filter 9 that this account can see.")).toBeVisible();
  expect(screen.getByRole("dialog")).toBeInTheDocument();
});

it("says so when the account has starred no filter", async () => {
  vi.mocked(api.JiraFilters).mockResolvedValue([]);
  const { dialog } = await openDialog();
  expect(await within(dialog).findByText(/You have starred no filters/)).toBeVisible();
});

it("asks before a filter replaces typed JQL, and keeps it when cancelled", async () => {
  render(
    <DialogProvider>
      <App />
    </DialogProvider>,
  );
  const user = userEvent.setup();
  await user.type(await screen.findByLabelText("JQL"), "project = MINE");
  await user.click(screen.getByRole("button", { name: "From a Jira filter" }));
  await user.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Client reports, open" }));

  const ask = await screen.findByRole("alertdialog");
  expect(ask).toHaveTextContent("The JQL you typed is not saved");
  await user.click(within(ask).getAllByRole("button", { name: "Cancel" }).at(-1)!);
  expect(screen.getByLabelText("JQL")).toHaveValue("project = MINE");
});
