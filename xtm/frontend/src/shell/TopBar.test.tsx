import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TopBar } from "./TopBar";
import type { AppState } from "./useAppState";
import type { AppActions } from "./useAppActions";

function renderBar() {
  const s = {
    profiles: [{ id: "p1", name: "Alpha", projectKey: "AL" }],
    activeId: "p1",
    setActiveId: vi.fn(),
    theme: "dark",
    showCoverage: false,
    chooseTheme: vi.fn(),
    openModal: vi.fn(),
    pulling: false,
    view: "browse",
    groupBy: "folder",
    selectedFolder: "",
    setSelectedKey: vi.fn(),
    startTour: vi.fn(),
    pendingChanges: [{ id: 1 }],
    isDemo: false,
    syncActive: false,
  } as unknown as AppState;
  const a = { runSync: vi.fn(), runFullSync: vi.fn(), toggleCoverage: vi.fn(), openNewTest: vi.fn() } as unknown as AppActions;
  const { container } = render(<TopBar s={s} a={a} />);
  return { a, container };
}

// TAM's top bar follows this order since #540, so the two apps put pending,
// their own create or refresh button, More and Sync in the same places.
describe("TopBar", () => {
  it("keeps pending, then More, then Sync last on the right", () => {
    const { container } = renderBar();
    const right = container.querySelector(".topbar-right") as HTMLElement;
    const labels = Array.from(right.querySelectorAll("button")).map((b) => b.textContent?.replace("▾", "").trim());
    expect(labels).toEqual(["1 pending", "＋ New Test", "More", "Sync"]);
  });

  it("syncs on one click and keeps Full resync under More", async () => {
    const user = userEvent.setup();
    const { a } = renderBar();
    await user.click(screen.getByRole("button", { name: "Sync" }));
    expect(a.runSync).toHaveBeenCalledTimes(1);
    await user.click(screen.getByRole("button", { name: /More/ }));
    await user.click(screen.getByRole("menuitem", { name: /Full resync/ }));
    expect(a.runFullSync).toHaveBeenCalledTimes(1);
  });
});
