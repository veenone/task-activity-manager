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
  ProductField: vi.fn(),
  SetProductField: vi.fn(),
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
  vi.mocked(api.ProductField).mockResolvedValue("Old base");
});

async function nameField(user: ReturnType<typeof userEvent.setup>, name: string) {
  await user.click(await screen.findByRole("button", { name: "Product base field" }));
  const dialog = await screen.findByRole("dialog");
  const box = within(dialog).getByRole("textbox");
  expect(box).toHaveValue("Old base");
  await user.clear(box);
  await user.type(box, name);
  await user.click(within(dialog).getByRole("button", { name: "Save" }));
}

it("stores the product base field the analyst names for the profile", async () => {
  vi.mocked(api.SetProductField).mockResolvedValue();
  const user = show();
  await nameField(user, "Product base");
  expect(api.ProductField).toHaveBeenCalledWith("p1");
  expect(api.SetProductField).toHaveBeenCalledWith("p1", "Product base");
});

it("says why Jira refused the field name", async () => {
  vi.mocked(api.SetProductField).mockRejectedValue('Jira has no custom field named "Nope"');
  const user = show();
  await nameField(user, "Nope");
  expect(await screen.findByRole("alert")).toHaveTextContent('Jira has no custom field named "Nope"');
});
