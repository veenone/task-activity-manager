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
}));

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
  vi.mocked(api.ListQueries).mockResolvedValue([]);
  vi.mocked(api.Fields).mockResolvedValue({ productBase: "Old base", clientVisibility: "", clientImpact: "" } as never);
  vi.mocked(api.ListCases).mockResolvedValue([]);
});

async function fill(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole("button", { name: "Jira fields" }));
  const dialog = await screen.findByRole("dialog");
  expect(within(dialog).getByLabelText("Product base")).toHaveValue("Old base");
  await user.type(within(dialog).getByLabelText("Client visibility"), "Visibility");
  await user.type(within(dialog).getByLabelText("Client impact"), "Impact");
  await user.click(within(dialog).getByRole("button", { name: "Save" }));
  return dialog;
}

it("stores the profile's field names together", async () => {
  vi.mocked(api.SetFields).mockResolvedValue();
  const user = show();
  await fill(user);
  expect(api.Fields).toHaveBeenCalledWith("p1");
  expect(api.SetFields).toHaveBeenCalledWith("p1", {
    productBase: "Old base",
    clientVisibility: "Visibility",
    clientImpact: "Impact",
  });
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
});

it("keeps the dialog open and says why Jira refused a name", async () => {
  vi.mocked(api.SetFields).mockRejectedValue('Jira has no custom field named "Impact". Check the client impact field\'s name');
  const user = show();
  const dialog = await fill(user);
  expect(await within(dialog).findByText(/no custom field named "Impact"/)).toBeVisible();
  expect(within(dialog).getByLabelText("Client impact")).toHaveValue("Impact");
});

// Opening the dialog before the stored names arrive would let Save blank them.
it("waits for the stored names before the dialog can open", async () => {
  vi.mocked(api.Fields).mockReturnValue(new Promise(() => {}));
  show();
  expect(await screen.findByRole("button", { name: "Jira fields" })).toBeDisabled();
});
