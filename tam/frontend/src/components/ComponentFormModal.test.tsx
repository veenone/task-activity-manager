import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, LiveRegion, createQueryClient } from "@agile-suite/core";
import * as api from "../api";
import type { ComponentSummary } from "../api";
import { ComponentFormModal } from "./ComponentFormModal";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, CreateComponent: vi.fn(), EditComponent: vi.fn(), SearchUsers: vi.fn() };
});

vi.mock("../contexts/SyncContext", () => ({
  useSync: () => ({ status: "idle", runQuietLock: async <T,>(action: () => Promise<T>) => action() }),
}));

const AUTH: ComponentSummary = {
  id: 10000, project: "PLAT", name: "Auth", description: "", leadName: "", leadDisplayName: "", assigneeType: "PROJECT_DEFAULT", draft: false, issues: 2,
};

function show(component?: ComponentSummary, onSaved = vi.fn()) {
  render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <LiveRegion />
        <ComponentFormModal profileId="p1" component={component} takenNames={["Auth", "Board"]} onClose={vi.fn()} onSaved={onSaved} />
      </DialogProvider>
    </QueryClientProvider>,
  );
  return onSaved;
}

beforeEach(() => {
  vi.mocked(api.SearchUsers).mockResolvedValue([{ name: "alee", displayName: "A. Lee" }]);
  vi.mocked(api.CreateComponent).mockResolvedValue({ ...AUTH, id: -1, name: "Reports", draft: true });
  vi.mocked(api.EditComponent).mockResolvedValue(undefined);
});

describe("ComponentFormModal", () => {
  it("refuses a blank or taken name before anything is sent", async () => {
    show();
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    expect(screen.getByText("A component needs a name.")).toHaveAttribute("role", "alert");
    await userEvent.type(screen.getByLabelText("Name"), "auth");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    expect(screen.getByText('A component named "auth" already exists.')).toHaveAttribute("role", "alert");
    expect(api.CreateComponent).not.toHaveBeenCalled();
  });

  it("drafts a component with its lead and default assignee", async () => {
    const onSaved = show();
    await userEvent.type(screen.getByLabelText("Name"), "Reports");
    await userEvent.click(screen.getByLabelText("Lead"));
    await userEvent.click(await screen.findByRole("option", { name: /A\. Lee/ }));
    await userEvent.selectOptions(screen.getByLabelText("Default assignee"), "COMPONENT_LEAD");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    await waitFor(() => expect(api.CreateComponent).toHaveBeenCalled());
    expect(vi.mocked(api.CreateComponent).mock.calls[0][1]).toEqual({
      name: "Reports", description: "", leadName: "alee", leadDisplayName: "A. Lee", assigneeType: "COMPONENT_LEAD",
    });
    expect(onSaved).toHaveBeenCalledWith(-1, "Reports was drafted. Commit creates it in Jira.");
  });

  it("saves an edit as a pending change", async () => {
    const onSaved = show(AUTH);
    const name = screen.getByLabelText("Name");
    await userEvent.clear(name);
    await userEvent.type(name, "Login");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(api.EditComponent).toHaveBeenCalled());
    expect(vi.mocked(api.EditComponent).mock.calls[0].slice(1)).toEqual([10000, { name: "Login", description: "", leadName: "", leadDisplayName: "", assigneeType: "PROJECT_DEFAULT" }]);
    expect(onSaved).toHaveBeenCalledWith(10000, "Login was saved locally. Commit sends the change to Jira.");
  });
});
