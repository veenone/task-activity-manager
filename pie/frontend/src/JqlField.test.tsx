import { beforeEach, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DialogProvider } from "@agile-suite/core";
import App from "./App";
import * as api from "../wailsjs/go/main/App";

vi.mock("../wailsjs/go/main/App", () => ({
  StartupError: vi.fn(),
  ListProfiles: vi.fn(),
  RunQuery: vi.fn(),
  ListQueries: vi.fn(),
  LastRun: vi.fn(),
  Fields: vi.fn(),
  ListCases: vi.fn(),
}));

const weekly = { id: 7, name: "Weekly", jql: "category = X", runAt: "" };
const empty = { total: 0, truncated: false, linkError: "", linksTruncated: false, needsConfirm: false, sets: [], issues: [] };

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.StartupError).mockResolvedValue("");
  vi.mocked(api.Fields).mockResolvedValue({ productBase: "", clientVisibility: "", clientImpact: "" } as never);
  vi.mocked(api.ListCases).mockResolvedValue([]);
  vi.mocked(api.ListProfiles).mockResolvedValue([{ id: "p1", name: "Q", jiraUrl: "demo" }] as never);
  vi.mocked(api.ListQueries).mockResolvedValue([weekly] as never);
  vi.mocked(api.LastRun).mockResolvedValue({ result: null, runAt: "" } as never);
  vi.mocked(api.RunQuery).mockResolvedValue({ result: empty, runAt: "", added: null, dropped: null } as never);
});

function show() {
  render(
    <DialogProvider>
      <App />
    </DialogProvider>,
  );
  return userEvent.setup();
}

const section = () => screen.getByRole("group", { name: /^JQL/ });

it("starts open while there is no JQL, then folds to one line when a saved query opens", async () => {
  const user = show();
  await screen.findByRole("option", { name: "Weekly" });
  expect(section()).toHaveAttribute("open");

  await user.selectOptions(screen.getByLabelText("Saved query"), "Weekly");
  expect(section()).not.toHaveAttribute("open");
  expect(screen.getByText("category = X", { selector: "summary *" })).toBeVisible();
});

it("unfolds to edit and runs while folded", async () => {
  const user = show();
  await screen.findByRole("option", { name: "Weekly" });
  await user.selectOptions(screen.getByLabelText("Saved query"), "Weekly");

  await user.click(screen.getByRole("button", { name: "Run" }));
  expect(api.RunQuery).toHaveBeenCalledWith("p1", 7, "category = X", false);

  await user.click(screen.getByText("category = X", { selector: "summary *" }));
  expect(section()).toHaveAttribute("open");
  expect(screen.getByLabelText("JQL")).toHaveValue("category = X");
});
