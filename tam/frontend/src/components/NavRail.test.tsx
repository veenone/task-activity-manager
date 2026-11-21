import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ViewTabs } from "@agile-suite/core";
import { VIEWS } from "../nav";
import type { View } from "../nav";
import { NavRail } from "./NavRail";

vi.mock("../api", () => ({ SetNavRailVisible: vi.fn().mockResolvedValue(undefined) }));

function Shell() {
  const [rail, setRail] = useState(true);
  const [view, setView] = useState<View>("boards");
  return (
    <>
      <ViewTabs views={VIEWS} current={view} onSelect={setView} />
      {rail && <NavRail view={view} onSelect={setView} onHide={() => setRail(false)} />}
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
      within(screen.getByRole("navigation", { name: "Views" })).getByRole("button", { name: "Boards" }),
    );
  });
});
