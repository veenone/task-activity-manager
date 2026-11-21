import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TopBarActions } from "./TopBarActions";

function renderBar(over: Partial<Parameters<typeof TopBarActions>[0]> = {}) {
  const props = {
    pendingCount: 3,
    canSync: true,
    theme: "dark",
    onPending: vi.fn(),
    onRefresh: vi.fn(),
    onSync: vi.fn(),
    onTheme: vi.fn(),
    onTour: vi.fn(),
    onFirstRunTour: vi.fn(),
    onDiagnostics: vi.fn(),
    onAbout: vi.fn(),
    ...over,
  };
  const { container } = render(<TopBarActions {...props} />);
  return { props, container };
}

// TAM's Sync was a dropdown with Full sync inside it, and Theme and Help were
// menus of their own, where XTM syncs on one click and keeps Full resync,
// the tour, Diagnostics and the theme under More (#540).
describe("TopBarActions", () => {
  it("keeps XTM's order: pending, then More, then Sync last", () => {
    const { container } = renderBar();
    const labels = Array.from(container.querySelectorAll("button")).map((b) => b.textContent?.replace("▾", "").trim());
    expect(labels).toEqual(["3 pending", "Refresh", "More", "Sync"]);
  });

  it("syncs changes on one click", async () => {
    const user = userEvent.setup();
    const { props } = renderBar();
    await user.click(screen.getByRole("button", { name: "Sync" }));
    expect(props.onSync).toHaveBeenCalledWith(false);
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });

  it("keeps Full sync, the tours, Diagnostics and the theme under More", async () => {
    const user = userEvent.setup();
    const { props } = renderBar();
    await user.click(screen.getByRole("button", { name: /More/ }));
    expect(screen.getAllByRole("menuitem").map((m) => m.textContent?.replace("✓", "").trim())).toEqual([
      "Full sync",
      "Take the tour",
      "Take the first-run tour",
      "Diagnostics",
      "About",
      "Theme: Light",
      "Theme: Dark",
      "Theme: System",
    ]);
    await user.click(screen.getByRole("menuitem", { name: "Full sync" }));
    expect(props.onSync).toHaveBeenCalledWith(true);
  });

  it("holds both syncs while another run has the lock", async () => {
    const user = userEvent.setup();
    renderBar({ canSync: false });
    expect(screen.getByRole("button", { name: "Sync" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: /More/ }));
    expect(screen.getByRole("menuitem", { name: "Full sync" })).toBeDisabled();
  });
});
