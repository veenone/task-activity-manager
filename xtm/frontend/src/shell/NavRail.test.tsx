import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ViewTabs } from "@agile-suite/core";
import { NavRail } from "./NavRail";
import type { AppState } from "./useAppState";

vi.mock("../api", () => ({ SetNavRailVisible: vi.fn().mockResolvedValue(undefined) }));

const views = [
  { id: "tests", label: "Tests" },
  { id: "plans", label: "Plans" },
] as const;

function Shell() {
  const [navRail, setNavRail] = useState(true);
  const [view, setView] = useState<string>("plans");
  const s = { navRail, setNavRail, view, setView, shownViews: views } as unknown as AppState;
  return (
    <>
      <ViewTabs views={views} current={view} onSelect={setView} />
      <NavRail s={s} />
    </>
  );
}

describe("NavRail", () => {
  // The hide button unmounts with the rail, which used to drop focus to the
  // page body (#529).
  it("moves focus to the current view tab when it hides itself", async () => {
    const user = userEvent.setup();
    render(<Shell />);
    await user.click(screen.getByRole("button", { name: "Hide the navigation rail" }));
    expect(screen.queryByRole("navigation", { name: "Navigation rail" })).not.toBeInTheDocument();
    expect(document.activeElement).toBe(
      within(screen.getByRole("navigation", { name: "Views" })).getByRole("button", { name: "Plans" }),
    );
  });
});
