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
  UpdateProfile: vi.fn(),
  DeleteProfile: vi.fn(),
  DefaultProfile: vi.fn(),
  SetDefaultProfile: vi.fn(),
  ExportProfile: vi.fn(),
  ImportProfile: vi.fn(),
  ConfluenceSettings: vi.fn(),
  SetConfluenceSettings: vi.fn(),
  ListQueries: vi.fn(),
  Fields: vi.fn(),
  ListCases: vi.fn(),
}));

const quality = { id: "p1", name: "Quality", jiraUrl: "https://jira.example.com", projectKey: "" };
const mine = { id: "p2", name: "Mine", jiraUrl: "https://jira2.example.com", projectKey: "" };
const noConfluence = { baseURL: "", spaceKey: "", rootPageID: "", reportsSpaceKey: "", reportsRootPageID: "" };

function show() {
  render(
    <DialogProvider>
      <App />
    </DialogProvider>,
  );
  return userEvent.setup();
}

async function openManager(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole("button", { name: "Manage profiles" }));
  return screen.findByRole("dialog");
}

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.Fields).mockResolvedValue({ productBase: "", clientVisibility: "", clientImpact: "" } as never);
  vi.mocked(api.ListCases).mockResolvedValue([]);
  vi.mocked(api.StartupError).mockResolvedValue("");
  vi.mocked(api.ListQueries).mockResolvedValue([]);
  vi.mocked(api.ListProfiles).mockResolvedValue([quality] as never);
  vi.mocked(api.DefaultProfile).mockResolvedValue("");
  vi.mocked(api.ConfluenceSettings).mockResolvedValue(noConfluence as never);
});

it("asks for a profile first when there are none, and creates one", async () => {
  vi.mocked(api.ListProfiles).mockResolvedValue([]);
  vi.mocked(api.CreateProfile).mockResolvedValue(quality as never);
  const user = show();
  expect(await screen.findByText("Add a profile to run a query.")).toBeVisible();
  const dialog = await openManager(user);
  await user.click(within(dialog).getAllByRole("button", { name: /Create new profile/ })[0]);
  await user.type(within(dialog).getByLabelText("Name"), "Quality");
  await user.type(within(dialog).getByLabelText("Jira URL"), "https://jira.example.com");
  await user.type(within(dialog).getByLabelText("Personal access token"), "secret");
  vi.mocked(api.ListProfiles).mockResolvedValue([quality] as never);
  await user.click(within(dialog).getByRole("button", { name: "Save" }));

  expect(api.CreateProfile).toHaveBeenCalledWith("Quality", "https://jira.example.com", "secret");
  // No Confluence was entered, so none is stored.
  expect(api.SetConfluenceSettings).not.toHaveBeenCalled();
  expect(await screen.findByRole("option", { name: "Quality" })).toBeInTheDocument();
});

it("creates a demo profile without asking for a token", async () => {
  vi.mocked(api.CreateProfile).mockResolvedValue({ id: "d1", name: "Demo", jiraUrl: "demo" } as never);
  const user = show();
  const dialog = await openManager(user);
  await user.click(within(dialog).getAllByRole("button", { name: /Create new profile/ })[0]);
  await user.type(within(dialog).getByLabelText("Name"), "Demo");
  await user.type(within(dialog).getByLabelText("Jira URL"), "demo");
  expect(within(dialog).queryByLabelText("Personal access token")).not.toBeInTheDocument();
  await user.click(within(dialog).getByRole("button", { name: "Save" }));
  expect(api.CreateProfile).toHaveBeenCalledWith("Demo", "demo", "");
});

it("keeps the form open and shows why when a profile cannot be saved", async () => {
  vi.mocked(api.CreateProfile).mockRejectedValue("a profile needs a name");
  const user = show();
  const dialog = await openManager(user);
  await user.click(within(dialog).getAllByRole("button", { name: /Create new profile/ })[0]);
  await user.click(within(dialog).getByRole("button", { name: "Save" }));
  expect(await within(dialog).findByText("a profile needs a name")).toBeVisible();
  expect(within(dialog).getByLabelText("Name")).toBeVisible();
});

it("edits a profile in place, keeping its token, and stores its Confluence settings", async () => {
  vi.mocked(api.ConfluenceSettings).mockResolvedValue({ ...noConfluence, baseURL: "https://wiki.example.com", spaceKey: "SEC" } as never);
  vi.mocked(api.UpdateProfile).mockResolvedValue({ ...quality, name: "Quality team" } as never);
  vi.mocked(api.SetConfluenceSettings).mockResolvedValue();
  const user = show();
  const dialog = await openManager(user);
  await user.click(within(dialog).getByRole("button", { name: /Quality/ }));
  const name = within(dialog).getByLabelText("Name");
  expect(name).toHaveValue("Quality");
  expect(await within(dialog).findByLabelText("Confluence URL")).toHaveValue("https://wiki.example.com");
  await user.clear(name);
  await user.type(name, "Quality team");
  await user.type(within(dialog).getByLabelText("Parent page id"), "7");
  await user.type(within(dialog).getByLabelText("Confluence token"), "wiki-secret");
  await user.click(within(dialog).getByRole("button", { name: "Save" }));

  expect(api.UpdateProfile).toHaveBeenCalledWith("p1", "Quality team", "https://jira.example.com", "");
  expect(api.SetConfluenceSettings).toHaveBeenCalledWith(
    "p1",
    { ...noConfluence, baseURL: "https://wiki.example.com", spaceKey: "SEC", rootPageID: "7" },
    "wiki-secret",
  );
});

it("opens the default profile at launch and stars it", async () => {
  vi.mocked(api.ListProfiles).mockResolvedValue([quality, mine] as never);
  vi.mocked(api.DefaultProfile).mockResolvedValue("p2");
  vi.mocked(api.SetDefaultProfile).mockResolvedValue();
  const user = show();
  expect(await screen.findByDisplayValue("Mine")).toBeVisible();
  const dialog = await openManager(user);
  await user.click(within(dialog).getByRole("button", { name: "Set as default" }));
  expect(api.SetDefaultProfile).toHaveBeenCalledWith("p1");
});

it("deletes a profile only after confirming", async () => {
  vi.mocked(api.DeleteProfile).mockResolvedValue(undefined as never);
  const user = show();
  const dialog = await openManager(user);
  await user.click(within(dialog).getByRole("button", { name: /Quality/ }));
  await user.click(within(dialog).getByRole("button", { name: /Delete/ }));
  expect(api.DeleteProfile).not.toHaveBeenCalled();
  vi.mocked(api.ListProfiles).mockResolvedValue([]);
  await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Delete profile" }));
  expect(api.DeleteProfile).toHaveBeenCalledWith("p1");
  expect(await screen.findByText("Add a profile to run a query.")).toBeInTheDocument();
});
