import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ComponentsView } from "./ComponentsView";

const api = vi.hoisted(() => ({
  ListProjectComponentDetails: vi.fn(),
  ListComponents: vi.fn(),
  ListTests: vi.fn(),
  CreateComponent: vi.fn(),
  UpdateComponent: vi.fn(),
  DeleteComponent: vi.fn(),
  ComponentIssueCount: vi.fn(),
  SearchUsers: vi.fn(async () => []),
}));
vi.mock("../../api", () => ({
  ...api,
  errMsg: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}));
vi.mock("../../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1", activeProfile: { projectKey: "QA" } }),
}));

const core = {
  id: "1",
  name: "Core",
  description: "Engine",
  leadName: "bob",
  leadDisplayName: "Bob Brown",
  assigneeType: "PROJECT_DEFAULT",
};
const apiComp = {
  id: "2",
  name: "API",
  description: "",
  leadName: "",
  leadDisplayName: "",
  assigneeType: "PROJECT_DEFAULT",
};

// Letters rather than numbers, because the list sorts by name as text and
// "Gen 10" sorts before "Gen 2".
function comp(letter: string) {
  return {
    id: `g${letter}`,
    name: `Gen ${letter}`,
    description: "",
    leadName: "",
    leadDisplayName: "",
    assigneeType: "PROJECT_DEFAULT",
  };
}

function renderView(onChanged = vi.fn()) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <ComponentsView onChanged={onChanged} />
    </QueryClientProvider>,
  );
  return onChanged;
}

// The selected component's name is in both panes, so every lookup says which
// one it means. The list is also where the default selection lands: sorted by
// name, so "API" comes before "Core".
async function listPane() {
  await screen.findByLabelText("Filter components");
  return document.querySelector(".components-list") as HTMLElement;
}

async function selectComponent(name: string) {
  const list = await listPane();
  const row = await within(list).findByText(name);
  await userEvent.click(row.closest("button")!);
  const heading = await screen.findByRole("heading", { name });
  return heading.closest(".components-detail") as HTMLElement;
}

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  api.ListProjectComponentDetails.mockResolvedValue([core, apiComp]);
  api.ListComponents.mockResolvedValue([{ label: "Core", count: 4 }]);
  api.ListTests.mockResolvedValue({ tests: [], total: 0 });
  api.SearchUsers.mockResolvedValue([]);
});

describe("ComponentsView", () => {
  it("lists components with their local test count", async () => {
    renderView();
    const list = await listPane();
    const coreRow = (await within(list).findByText("Core")).closest("button")!;
    expect(within(coreRow).getByText("4 tests")).toBeTruthy();
    const apiRow = within(list).getByText("API").closest("button")!;
    expect(within(apiRow).getByText("0 tests")).toBeTruthy();
  });

  // The write path here is the exception to XTM's queue-then-commit rule, and
  // nothing else on screen would say so.
  it("says that changes here are not queued", async () => {
    renderView();
    expect(
      await screen.findByText(/written to Jira straight away/i),
    ).toBeTruthy();
    expect(screen.getByText(/not queued/i)).toBeTruthy();
  });

  // The pane answers the question somebody opens this view asking: what
  // carries this component.
  it("lists the tests carrying the selected component", async () => {
    api.ListTests.mockResolvedValue({
      tests: [
        { key: "QA-1", summary: "Login works", status: "Approved" },
        { key: "QA-2", summary: "Logout works", status: "Draft" },
      ],
      total: 2,
    });
    renderView();
    const pane = await selectComponent("Core");
    expect(await within(pane).findByText("Login works")).toBeTruthy();
    expect(within(pane).getByText("QA-2")).toBeTruthy();
    // Filtered by the selected component, and paged by the server rather than
    // fetched whole and sliced.
    expect(api.ListTests).toHaveBeenCalledWith(
      "p1",
      expect.objectContaining({ component: "Core", limit: 10, offset: 0 }),
    );
  });

  it("selecting another component asks for that component's tests", async () => {
    renderView();
    await selectComponent("Core");
    expect(api.ListTests).toHaveBeenCalledWith(
      "p1",
      expect.objectContaining({ component: "Core" }),
    );
  });

  // Ten per page, which is the point of adding a pager to a list that used to
  // render every component at once.
  it("shows ten components per page", async () => {
    const letters = "ABCDEFGHIJKLMNOPQRSTUVW".split("");
    api.ListProjectComponentDetails.mockResolvedValue(letters.map(comp));
    renderView();
    const list = await listPane();
    expect(await within(list).findByText("Gen A")).toBeTruthy();
    expect(within(list).getByText("Gen J")).toBeTruthy();
    expect(within(list).queryByText("Gen K")).toBeNull();
  });

  it("filters the list by name", async () => {
    renderView();
    const list = await listPane();
    await within(list).findByText("Core");
    await userEvent.type(screen.getByLabelText("Filter components"), "api");
    expect(within(list).queryByText("Core")).toBeNull();
    expect(within(list).getByText("API")).toBeTruthy();
  });

  it("creates a component and reloads the list", async () => {
    api.CreateComponent.mockResolvedValue({
      ...apiComp,
      id: "3",
      name: "Mobile",
    });
    const onChanged = renderView();
    const list = await listPane();
    await within(list).findByText("Core");
    await userEvent.click(
      screen.getByRole("button", { name: "New component" }),
    );
    await userEvent.type(
      screen.getByRole("textbox", { name: "Name" }),
      "Mobile",
    );
    api.ListProjectComponentDetails.mockResolvedValue([
      core,
      apiComp,
      { ...apiComp, id: "3", name: "Mobile" },
    ]);
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    expect(api.CreateComponent).toHaveBeenCalledWith(
      "p1",
      expect.objectContaining({ name: "Mobile" }),
    );
    expect(await within(list).findByText("Mobile")).toBeTruthy();
    expect(onChanged).toHaveBeenCalled();
  });

  it("deletes with a move target after showing the issue count", async () => {
    api.ComponentIssueCount.mockResolvedValue(5);
    api.DeleteComponent.mockResolvedValue(undefined);
    renderView();
    const pane = await selectComponent("Core");
    await userEvent.click(
      within(pane).getByRole("button", { name: "Delete Core" }),
    );
    expect(await screen.findByText(/5 issues use Core/)).toBeTruthy();
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Move its issues to" }),
      "2",
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Delete component" }),
    );
    expect(api.DeleteComponent).toHaveBeenCalledWith("p1", "1", "2");
  });

  it("shows the admin-rights message and reloads after a failed write", async () => {
    api.UpdateComponent.mockRejectedValue(
      new Error("You need project admin rights in Jira to change components."),
    );
    renderView();
    const pane = await selectComponent("Core");
    await userEvent.click(
      within(pane).getByRole("button", { name: "Edit Core" }),
    );
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(
      await screen.findByText(
        "You need project admin rights in Jira to change components.",
      ),
    ).toBeTruthy();
    expect(
      api.ListProjectComponentDetails.mock.calls.length,
    ).toBeGreaterThanOrEqual(2);
  });
});
