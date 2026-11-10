// The harness goes first: its mocks must be in place before ../api loads.
import { mockConfirm, renderNavigationHarness, renderWithCaps, selectTestExecutions } from "./ContainersView.harness";
import { describe, it, expect, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BulkEditContainers, BulkSetTestRunStatus, DeallocateTests, EditContainer } from "../api";

describe("ContainersView dataset navigation", () => {
  it("leaves Bugs mode and selects the exact target after changing kind", async () => {
    renderNavigationHarness();
    await userEvent.click(screen.getByRole("button", { name: "Bugs" }));
    expect(screen.getByRole("button", { name: "Bugs" })).toHaveClass("seg-btn-active");

    await userEvent.click(screen.getByRole("button", { name: "Open target execution" }));

    expect(await screen.findByRole("combobox", { name: "Type" })).toHaveValue(
      "testexec",
    );
    expect(
      await screen.findByTitle("Pick a Test Execution (type to filter)"),
    ).toHaveTextContent("TE-1 · Nightly run");
    expect(screen.getByRole("button", { name: "Containers" })).toHaveClass(
      "seg-btn-active",
    );
  });
});

describe("ContainersView create button", () => {
  it("names the kind it creates", async () => {
    renderWithCaps({});
    await selectTestExecutions();
    expect(
      screen.getByRole("button", { name: "New Test Execution" }),
    ).toBeInTheDocument();
  });
});

describe("ContainersView Create bug gate", () => {
  it("offers Create bug when the backend can create one directly (Xray)", async () => {
    renderWithCaps({ supportsBugCreation: true, supportsBugRouting: false });
    await selectTestExecutions();

    expect(
      await screen.findByRole("button", { name: /create bug for this failed test/i }),
    ).toBeInTheDocument();
  });

  it("offers Create bug when the profile routes bugs elsewhere (configured Kiwi)", async () => {
    renderWithCaps({ supportsBugCreation: false, supportsBugRouting: true });
    await selectTestExecutions();

    expect(
      await screen.findByRole("button", { name: /create bug for this failed test/i }),
    ).toBeInTheDocument();
  });

  it("hides Create bug when the backend cannot file one and nothing is routed (unconfigured Kiwi)", async () => {
    renderWithCaps({ supportsBugCreation: false, supportsBugRouting: false });
    await selectTestExecutions();

    // Wait for the FAIL row to render so the query isn't just still loading.
    await screen.findByText("QA-1");
    expect(
      screen.queryByRole("button", { name: /create bug for this failed test/i }),
    ).toBeNull();
  });
});

describe("ContainersView keyboard", () => {
  it("starts renaming the container from the keyboard, as a click does", async () => {
    renderWithCaps({});
    await selectTestExecutions();

    const heading = await screen.findByRole("heading", { name: "Nightly run" });
    within(heading).getByRole("button").focus();
    await userEvent.keyboard("{Enter}");

    expect(screen.getByDisplayValue("Nightly run")).toBeInTheDocument();
  });

  it("filters the members by run status with Enter on a run badge", async () => {
    renderWithCaps({});
    await selectTestExecutions();

    // The result badges sit in the card's details, folded at first (#456).
    await userEvent.click(await screen.findByRole("button", { name: "Show details" }));
    const badge = await screen.findByRole("button", { name: "FAIL 1" });
    badge.focus();
    await userEvent.keyboard("{Enter}");

    expect(badge).toHaveAttribute("aria-pressed", "true");
  });

  it("names the Set env panel and closes it on Escape", async () => {
    renderWithCaps({});
    await selectTestExecutions();

    await userEvent.click(await screen.findByRole("button", { name: "Filters" }));
    await userEvent.click(await screen.findByRole("button", { name: /Set env on shown/ }));
    expect(screen.getByRole("dialog", { name: /Set environment on the 1 shown/ })).toBeInTheDocument();

    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("takes focus into the Set env dialog and hands it back on close", async () => {
    renderWithCaps({});
    await selectTestExecutions();

    await userEvent.click(await screen.findByRole("button", { name: "Filters" }));
    const trigger = await screen.findByRole("button", { name: /Set env on shown/ });
    await userEvent.click(trigger);
    const dialog = screen.getByRole("dialog", { name: /Set environment on the 1 shown/ });
    expect(dialog).toContainElement(document.activeElement as HTMLElement);
    for (let i = 0; i < 12; i++) await userEvent.tab();
    expect(dialog).toContainElement(document.activeElement as HTMLElement);

    await userEvent.keyboard("{Escape}");
    expect(document.activeElement).toBe(trigger);
  });
});

describe("ContainersView Set env on shown", () => {
  async function applySetEnv() {
    renderWithCaps({});
    await selectTestExecutions();
    await screen.findByText("TE-1");
    await userEvent.click(screen.getByRole("button", { name: "Filters" }));
    await userEvent.click(screen.getByRole("button", { name: /set env on shown/i }));
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: /batch environment operation/i }),
      "set_env",
    );
    await userEvent.type(screen.getByPlaceholderText("Environment…"), "staging");
    await userEvent.click(screen.getByRole("button", { name: /^apply to \d+$/i }));
  }

  it("leaves the executions alone when the confirm is cancelled", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    await applySetEnv();
    expect(mockConfirm).toHaveBeenCalledTimes(1);
    expect(BulkEditContainers).not.toHaveBeenCalled();
  });

  it("sets the environment on every shown execution once confirmed", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(false);
    mockConfirm.mockResolvedValue(true);
    await applySetEnv();
    await vi.waitFor(() =>
      expect(BulkEditContainers).toHaveBeenCalledWith("p1", ["TE-1"], {
        operation: "set_env",
        field: "",
        value: "staging",
      }),
    );
  });
});

// Pins for the T3.2 split (#231): what the view does with its members has to
// survive the code moving into smaller files.
describe("ContainersView members", () => {
  it("sets the result of the selected members in one go", async () => {
    vi.mocked(BulkSetTestRunStatus).mockResolvedValue({ succeeded: ["QA-1"], failed: [] } as never);
    renderWithCaps({});
    await selectTestExecutions();
    await screen.findByText("Login works");
    const boxes = screen.getAllByRole("checkbox");
    await userEvent.click(boxes[boxes.length - 1]);
    expect(screen.getByText("1 selected")).toBeInTheDocument();
    await userEvent.selectOptions(screen.getAllByRole("combobox").find((c) => c.className === "run-select" && c.closest(".board-bulk"))!, "PASS");
    await userEvent.click(screen.getByRole("button", { name: "Apply to selected" }));
    expect(BulkSetTestRunStatus).toHaveBeenCalledWith("p1", "TE-1", ["QA-1"], "PASS");
  });

  it("removes a member from the container", async () => {
    renderWithCaps({});
    await selectTestExecutions();
    await screen.findByText("Login works");
    await userEvent.click(screen.getByRole("button", { name: "Remove from this Test Execution" }));
    expect(DeallocateTests).toHaveBeenCalledWith("p1", "TE-1", ["QA-1"]);
  });

  it("renames the container with Enter", async () => {
    renderWithCaps({});
    await selectTestExecutions();
    const heading = await screen.findByRole("heading", { name: "Nightly run" });
    await userEvent.click(within(heading).getByRole("button"));
    const input = screen.getByDisplayValue("Nightly run");
    await userEvent.clear(input);
    await userEvent.type(input, "Weekly run{Enter}");
    expect(EditContainer).toHaveBeenCalledWith("p1", "TE-1", "Weekly run");
  });
});
