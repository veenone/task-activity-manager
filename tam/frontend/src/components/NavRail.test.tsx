import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { VIEWS } from "../nav";
import { NavRail } from "./NavRail";

vi.mock("../api", () => ({ SetNavRailVisible: vi.fn(async () => {}) }));

describe("NavRail", () => {
  // The rail used to end with a disabled "Tests (XTM)" entry for a suite
  // launcher that does not exist (#542). Every entry it shows now opens a view.
  it("lists only the views, each one enabled", () => {
    render(<NavRail view="backlog" onSelect={vi.fn()} onHide={vi.fn()} />);
    const rail = screen.getByRole("navigation", { name: "Navigation rail" });
    const entries = within(rail)
      .getAllByRole("button")
      .filter((b) => !b.classList.contains("nav-rail-hide"));
    expect(entries.map((b) => b.textContent)).toEqual(VIEWS.map((v) => v.label));
    expect(entries.filter((b) => (b as HTMLButtonElement).disabled)).toEqual([]);
  });
});
