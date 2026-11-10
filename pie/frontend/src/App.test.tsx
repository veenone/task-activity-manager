import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
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
  SaveQuery: vi.fn(),
  DeleteQuery: vi.fn(),
  LastRun: vi.fn(),
  Fields: vi.fn(),
  SetFields: vi.fn(),
  ListCases: vi.fn(),
  OpenCases: vi.fn(),
}));

const quality = { id: "p1", name: "Quality", jiraUrl: "https://jira.example.com" };
const issue = {
  key: "ALPHA-1",
  project: "ALPHA",
  type: "Bug",
  summary: "Attach fails",
  status: "Open",
  priority: "High",
  updated: "2026-11-05T10:00:00.000+0700",
  matched: true,
};

function show() {
  return render(
    <DialogProvider>
      <App />
    </DialogProvider>,
  );
}

async function runJql(jql: string) {
  const user = userEvent.setup();
  const box = await screen.findByLabelText("JQL");
  await user.type(box, jql);
  await user.click(screen.getByRole("button", { name: "Run" }));
}

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.Fields).mockResolvedValue({ productBase: "", clientVisibility: "", clientImpact: "" } as never);
  vi.mocked(api.ListCases).mockResolvedValue([]);
  vi.mocked(api.StartupError).mockResolvedValue("");
  vi.mocked(api.ListQueries).mockResolvedValue([]);
  vi.mocked(api.ListProfiles).mockResolvedValue([quality] as never);
  vi.mocked(api.DefaultProfile).mockResolvedValue("");
});

describe("running a query", () => {
  it("sends the JQL with the selected profile and shows the rows", async () => {
    vi.mocked(api.RunQuery).mockResolvedValue({ result: { total: 1, truncated: false, issues: [issue] } } as never);
    show();
    await runJql("category = X");

    const row = (await screen.findByRole("cell", { name: "ALPHA-1" })).closest("tr")!;
    expect(api.RunQuery).toHaveBeenCalledWith("p1", 0, "category = X", false);
    expect(within(row).getByRole("cell", { name: "Attach fails" })).toBeVisible();
    expect(within(row).getByRole("cell", { name: "2026-11-05" })).toBeVisible();
    expect(screen.getByText("1 issue")).toBeVisible();
    expect(screen.getByRole("table", { name: "Query results" })).toContainElement(row);
  });

  it("shows Jira's own message when the query is refused, and keeps the last rows", async () => {
    vi.mocked(api.RunQuery).mockResolvedValueOnce({ result: { total: 1, truncated: false, issues: [issue] } } as never);
    show();
    await runJql("category = X");
    await screen.findByRole("cell", { name: "ALPHA-1" });

    const refusal = "A value with ID '12345' does not exist for the field 'filter'.";
    vi.mocked(api.RunQuery).mockRejectedValueOnce(refusal);
    await userEvent.setup().click(screen.getByRole("button", { name: "Run" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(refusal);
    expect(screen.getByRole("cell", { name: "ALPHA-1" })).toBeVisible();
  });

  it("says when the result stopped short of the total", async () => {
    vi.mocked(api.RunQuery).mockResolvedValue({ result: { total: 7000, truncated: true, issues: [issue] } } as never);
    show();
    await runJql("x");
    expect(await screen.findByText("Showing the first 1 of 7000 issues")).toBeVisible();
  });

  it("will not run without JQL", async () => {
    show();
    await screen.findByLabelText("JQL");
    expect(await screen.findByRole("button", { name: "Run" })).toBeDisabled();
  });

  // Two accounts see different projects, so one account's rows under another's
  // name would read as a false answer.
  it("clears the rows and the error when the profile changes", async () => {
    vi.mocked(api.ListProfiles).mockResolvedValue([quality, { ...quality, id: "p2", name: "Mine" }] as never);
    vi.mocked(api.RunQuery).mockResolvedValue({ result: { total: 1, truncated: false, issues: [issue] } } as never);
    show();
    await runJql("x");
    await screen.findByRole("cell", { name: "ALPHA-1" });

    await userEvent.setup().selectOptions(screen.getByLabelText("Profile"), "Mine");

    expect(screen.queryByRole("cell", { name: "ALPHA-1" })).not.toBeInTheDocument();
    expect(screen.queryByText("1 issue")).not.toBeInTheDocument();
  });

  it("keeps the profile fixed while a query runs", async () => {
    let finish!: (r: never) => void;
    vi.mocked(api.RunQuery).mockReturnValue(new Promise((resolve) => (finish = resolve)));
    show();
    await runJql("x");

    expect(await screen.findByLabelText("Profile")).toBeDisabled();
    expect(screen.getByRole("button", { name: "Manage profiles" })).toBeDisabled();

    finish({ result: { total: 0, truncated: false, issues: [] } } as never);
    await waitFor(() => expect(screen.getByLabelText("Profile")).toBeEnabled());
  });
});

it("shows a startup failure instead of a form that cannot work", async () => {
  vi.mocked(api.StartupError).mockResolvedValue("open the database: disk full");
  show();
  expect(await screen.findByRole("alert")).toHaveTextContent("open the database: disk full");
  expect(screen.queryByLabelText("JQL")).not.toBeInTheDocument();
});
