import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ViewTabs } from "./ViewTabs";

const VIEWS = [
  { id: "browse", label: "Browse" },
  { id: "dashboard", label: "Dashboard", tour: "tab-dash" },
];

describe("ViewTabs", () => {
  it("marks only the current view as the page you are on", () => {
    render(<ViewTabs views={VIEWS} current="dashboard" onSelect={() => {}} />);
    const nav = screen.getByRole("navigation", { name: "Views" });
    expect(nav).toContainElement(screen.getByRole("button", { name: "Dashboard" }));
    expect(screen.getByRole("button", { name: "Dashboard" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("button", { name: "Browse" })).not.toHaveAttribute("aria-current");
  });

  it("reports the view a click picked", async () => {
    const onSelect = vi.fn();
    render(<ViewTabs views={VIEWS} current="dashboard" onSelect={onSelect} />);
    await userEvent.click(screen.getByRole("button", { name: "Browse" }));
    expect(onSelect).toHaveBeenCalledWith("browse");
  });

  // The onboarding tour finds the row and each tab by data-tour.
  it("keeps the tour hooks, defaulting a tab's to tab-<id>", () => {
    render(<ViewTabs views={VIEWS} current="browse" onSelect={() => {}} />);
    expect(screen.getByRole("button", { name: "Browse" })).toHaveAttribute("data-tour", "tab-browse");
    expect(screen.getByRole("button", { name: "Dashboard" })).toHaveAttribute("data-tour", "tab-dash");
    expect(screen.getByRole("button", { name: "Browse" }).parentElement).toHaveAttribute("data-tour", "views");
  });
});
