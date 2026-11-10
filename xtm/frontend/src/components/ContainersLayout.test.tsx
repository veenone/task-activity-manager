// The harness goes first: its mocks must be in place before ../api loads.
import { renderWithCaps, selectTestExecutions } from "./ContainersView.harness";
import { describe, it, expect, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GetContainerBoard, ListContainers, SyncContainers } from "../api";

// The layout of #456: a one-line header, a one-line card and one filter row,
// so the member table keeps most of the height on every container type.
describe("ContainersView compact layout", () => {
  it("keeps New and Actions on the header, with Sync and Import JUnit in Actions", async () => {
    renderWithCaps({});
    await selectTestExecutions();
    const head = document.querySelector(".board-head") as HTMLElement;
    expect(within(head).getByRole("button", { name: "New Test Execution" })).toBeVisible();
    expect(within(head).queryByRole("button", { name: "Sync" })).toBeNull();
    expect(within(head).queryByRole("button", { name: "Import JUnit" })).toBeNull();

    await userEvent.click(within(head).getByRole("button", { name: /Actions/ }));
    expect(screen.getByRole("menuitem", { name: /Import JUnit/ })).toBeInTheDocument();
    await userEvent.click(screen.getByRole("menuitem", { name: /Sync Test Executions/ }));
    expect(SyncContainers).toHaveBeenCalledWith("p1");
  });

  it("shows the card as one line, with the details and related bugs a click away", async () => {
    renderWithCaps({});
    await selectTestExecutions();
    expect(await screen.findByRole("heading", { name: "Nightly run" })).toBeVisible();
    // Environments live in the details, folded at first.
    expect(screen.queryByText("Environments")).toBeNull();
    const bugs = screen.getByRole("button", { name: /Related bugs/ });
    expect(bugs).toHaveAttribute("aria-expanded", "false");
    await userEvent.click(bugs);
    expect(bugs).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("None")).toBeVisible();

    await userEvent.click(screen.getByRole("button", { name: "Show details" }));
    expect(screen.getByText("Environments")).toBeVisible();
  });

  it("folds the filters behind one button that counts them and shows each as a chip", async () => {
    renderWithCaps({});
    await selectTestExecutions();
    await screen.findByText("Login works");
    const toggle = screen.getByRole("button", { name: "Filters" });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByRole("combobox", { name: /Result/ })).toBeNull();

    await userEvent.click(toggle);
    await userEvent.selectOptions(screen.getByRole("combobox", { name: /Result/ }), "FAIL");
    expect(screen.getByRole("button", { name: "Filters (1)" })).toBeVisible();
    // Closed, the row still shows the filter in force.
    await userEvent.click(screen.getByRole("button", { name: "Filters (1)" }));
    expect(screen.queryByRole("combobox", { name: /Result/ })).toBeNull();
    expect(screen.getByText("Result: FAIL")).toBeVisible();

    await userEvent.click(screen.getByRole("button", { name: "Remove filter Result: FAIL" }));
    expect(screen.getByRole("button", { name: "Filters" })).toBeVisible();
    expect(screen.getByText("Login works")).toBeVisible();
  });

  it("shows no execution-only filter as a chip once another type is open", async () => {
    const list = vi.mocked(ListContainers).getMockImplementation()!;
    vi.mocked(ListContainers).mockImplementation(async (profileId: string, kind: string) =>
      (await list(profileId, kind)).map((c) => ({ ...c, environments: ["staging"] })),
    );
    renderWithCaps({});
    await selectTestExecutions();
    await screen.findByText("Login works");
    await userEvent.click(screen.getByRole("button", { name: "Filters" }));
    await userEvent.selectOptions(screen.getByRole("combobox", { name: /test environment/i }), "staging");
    expect(screen.getByRole("button", { name: "Filters (1)" })).toBeVisible();
    await userEvent.selectOptions(screen.getByRole("combobox", { name: "Type" }), "testplan");
    // Plans have no environments, so the filter is not in force here.
    expect(await screen.findByRole("button", { name: "Filters" })).toBeVisible();
    expect(screen.queryByText(/^Environment:/)).toBeNull();
    vi.mocked(ListContainers).mockImplementation(list);
  });

  it("filters a Test Plan's tests by label too", async () => {
    vi.mocked(GetContainerBoard).mockImplementation(async () => ({
      key: "PLAN-1",
      summary: "Release plan",
      description: "",
      rows: [
        { testKey: "QA-1", summary: "Login works", status: "Approved", runStatus: "", isExternal: false, labels: ["smoke"] },
        { testKey: "QA-2", summary: "Logout works", status: "Approved", runStatus: "", isExternal: false, labels: [] },
      ],
      runCounts: [],
    }) as never);
    renderWithCaps({});
    await screen.findByText("Logout works");
    await userEvent.click(screen.getByRole("button", { name: "Filters" }));
    await userEvent.selectOptions(screen.getByRole("combobox", { name: /Label/ }), "smoke");
    expect(screen.getByText("Login works")).toBeVisible();
    expect(screen.queryByText("Logout works")).toBeNull();
  });
});
