import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ComponentsView } from "./ComponentsView";

const api = vi.hoisted(() => ({
  ListProjectComponentDetails: vi.fn(),
  ListComponents: vi.fn(),
  CreateComponent: vi.fn(),
  UpdateComponent: vi.fn(),
  DeleteComponent: vi.fn(),
  ComponentIssueCount: vi.fn(),
  SearchUsers: vi.fn(async () => []),
}));
vi.mock("../../api", () => ({ ...api, errMsg: (e: unknown) => (e instanceof Error ? e.message : String(e)) }));
vi.mock("../../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1", activeProfile: { projectKey: "QA" } }),
}));

const core = { id: "1", name: "Core", description: "Engine", leadName: "bob", leadDisplayName: "Bob Brown", assigneeType: "PROJECT_DEFAULT" };
const apiComp = { id: "2", name: "API", description: "", leadName: "", leadDisplayName: "", assigneeType: "PROJECT_DEFAULT" };

function renderView(onChanged = vi.fn()) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <ComponentsView onChanged={onChanged} />
    </QueryClientProvider>,
  );
  return onChanged;
}

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  api.ListProjectComponentDetails.mockResolvedValue([core, apiComp]);
  api.ListComponents.mockResolvedValue([{ label: "Core", count: 4 }]);
  api.SearchUsers.mockResolvedValue([]);
});

describe("ComponentsView", () => {
  it("lists components with lead and local test count", async () => {
    renderView();
    const row = (await screen.findByText("Core")).closest("tr")!;
    expect(within(row).getByText("Engine")).toBeTruthy();
    expect(within(row).getByText("Bob Brown")).toBeTruthy();
    expect(within(row).getByText("4")).toBeTruthy();
    const apiRow = screen.getByText("API").closest("tr")!;
    expect(within(apiRow).getByText("0")).toBeTruthy();
  });

  it("creates a component and reloads the list", async () => {
    api.CreateComponent.mockResolvedValue({ ...apiComp, id: "3", name: "Mobile" });
    const onChanged = renderView();
    await screen.findByText("Core");
    await userEvent.click(screen.getByRole("button", { name: "New component" }));
    await userEvent.type(screen.getByRole("textbox", { name: "Name" }), "Mobile");
    api.ListProjectComponentDetails.mockResolvedValue([core, apiComp, { ...apiComp, id: "3", name: "Mobile" }]);
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    expect(api.CreateComponent).toHaveBeenCalledWith("p1", expect.objectContaining({ name: "Mobile" }));
    expect(await screen.findByText("Mobile")).toBeTruthy();
    expect(onChanged).toHaveBeenCalled();
  });

  it("deletes with a move target after showing the issue count", async () => {
    api.ComponentIssueCount.mockResolvedValue(5);
    api.DeleteComponent.mockResolvedValue(undefined);
    renderView();
    const row = (await screen.findByText("Core")).closest("tr")!;
    await userEvent.click(within(row).getByRole("button", { name: "Delete Core" }));
    expect(await screen.findByText(/5 issues use Core/)).toBeTruthy();
    await userEvent.selectOptions(screen.getByRole("combobox", { name: "Move its issues to" }), "2");
    await userEvent.click(screen.getByRole("button", { name: "Delete component" }));
    expect(api.DeleteComponent).toHaveBeenCalledWith("p1", "1", "2");
  });

  it("shows the admin-rights message and reloads after a failed write", async () => {
    api.UpdateComponent.mockRejectedValue(
      new Error("You need project admin rights in Jira to change components."),
    );
    renderView();
    const row = (await screen.findByText("Core")).closest("tr")!;
    await userEvent.click(within(row).getByRole("button", { name: "Edit Core" }));
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(
      await screen.findByText("You need project admin rights in Jira to change components."),
    ).toBeTruthy();
    expect(api.ListProjectComponentDetails.mock.calls.length).toBeGreaterThanOrEqual(2);
  });
});
