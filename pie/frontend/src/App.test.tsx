import { beforeEach, describe, expect, it, vi } from "vitest";
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
  vi.mocked(api.StartupError).mockResolvedValue("");
  vi.mocked(api.ListProfiles).mockResolvedValue([quality] as never);
});

describe("running a query", () => {
  it("sends the JQL with the selected profile and shows the rows", async () => {
    vi.mocked(api.RunQuery).mockResolvedValue({ total: 1, truncated: false, issues: [issue] } as never);
    show();
    await runJql("category = X");

    const row = (await screen.findByRole("cell", { name: "ALPHA-1" })).closest("tr")!;
    expect(api.RunQuery).toHaveBeenCalledWith("p1", "category = X");
    expect(within(row).getByRole("cell", { name: "Attach fails" })).toBeVisible();
    expect(within(row).getByRole("cell", { name: "2026-11-05" })).toBeVisible();
    expect(screen.getByText("1 issue")).toBeVisible();
    expect(screen.getByRole("table", { name: "Query results" })).toContainElement(row);
  });

  it("shows Jira's own message when the query is refused, and keeps the last rows", async () => {
    vi.mocked(api.RunQuery).mockResolvedValueOnce({ total: 1, truncated: false, issues: [issue] } as never);
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
    vi.mocked(api.RunQuery).mockResolvedValue({ total: 7000, truncated: true, issues: [issue] } as never);
    show();
    await runJql("x");
    expect(await screen.findByText("Showing the first 1 of 7000 issues")).toBeVisible();
  });

  it("will not run without JQL", async () => {
    show();
    await screen.findByLabelText("JQL");
    expect(await screen.findByRole("button", { name: "Run" })).toBeDisabled();
  });
});

describe("profiles", () => {
  it("asks for a profile first when there are none", async () => {
    vi.mocked(api.ListProfiles).mockResolvedValue([]);
    vi.mocked(api.CreateProfile).mockResolvedValue(quality as never);
    show();
    const user = userEvent.setup();
    await user.click(await screen.findByRole("button", { name: "Add profile" }));
    await user.type(screen.getByLabelText("Name"), "Quality");
    await user.type(screen.getByLabelText("Jira URL"), "https://jira.example.com");
    await user.type(screen.getByLabelText("Personal access token"), "secret");
    vi.mocked(api.ListProfiles).mockResolvedValue([quality] as never);
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(api.CreateProfile).toHaveBeenCalledWith("Quality", "https://jira.example.com", "secret");
    expect(await screen.findByRole("option", { name: "Quality" })).toBeInTheDocument();
    expect(screen.queryByLabelText("Personal access token")).not.toBeInTheDocument();
  });

  it("keeps the form open and shows why when a profile cannot be saved", async () => {
    vi.mocked(api.CreateProfile).mockRejectedValue("a profile needs a name");
    show();
    const user = userEvent.setup();
    await user.click(await screen.findByRole("button", { name: "Add profile" }));
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(await screen.findByText("a profile needs a name")).toBeVisible();
    expect(screen.getByLabelText("Name")).toBeVisible();
  });

  it("deletes a profile only after confirming", async () => {
    vi.mocked(api.DeleteProfile).mockResolvedValue(undefined as never);
    show();
    const user = userEvent.setup();
    await screen.findByRole("option", { name: "Quality" });
    await user.click(screen.getByRole("button", { name: "Delete profile" }));
    expect(api.DeleteProfile).not.toHaveBeenCalled();

    vi.mocked(api.ListProfiles).mockResolvedValue([]);
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Delete" }));
    expect(api.DeleteProfile).toHaveBeenCalledWith("p1");
    expect(await screen.findByText("Add a profile to run a query.")).toBeVisible();
  });
});

it("shows a startup failure instead of a form that cannot work", async () => {
  vi.mocked(api.StartupError).mockResolvedValue("open the database: disk full");
  show();
  expect(await screen.findByRole("alert")).toHaveTextContent("open the database: disk full");
  expect(screen.queryByLabelText("JQL")).not.toBeInTheDocument();
});
