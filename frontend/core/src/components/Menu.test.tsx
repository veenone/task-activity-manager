import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Menu } from "./Menu";

function renderMenu(onPick = vi.fn()) {
  render(
    <Menu
      label="Sync"
      items={[
        { key: "a", label: "Sync now", onClick: onPick },
        { key: "d", divider: true },
        { key: "b", label: "Locked", disabled: true },
        { key: "c", label: "Full sync" },
        { key: "e", label: "Sync settings" },
      ]}
    />,
  );
  return screen.getByRole("button", { name: /Sync/ });
}

describe("Menu", () => {
  it("moves focus to the first item when it opens", async () => {
    const user = userEvent.setup();
    await user.click(renderMenu());
    expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "Sync now" }));
  });

  it("walks the enabled items with Up, Down, Home and End", async () => {
    const user = userEvent.setup();
    await user.click(renderMenu());
    await user.keyboard("{ArrowDown}");
    expect(document.activeElement).toHaveTextContent("Full sync");
    await user.keyboard("{End}");
    expect(document.activeElement).toHaveTextContent("Sync settings");
    await user.keyboard("{ArrowDown}");
    expect(document.activeElement).toHaveTextContent("Sync now");
    await user.keyboard("{ArrowUp}");
    expect(document.activeElement).toHaveTextContent("Sync settings");
    await user.keyboard("{Home}");
    expect(document.activeElement).toHaveTextContent("Sync now");
  });

  it("returns focus to its trigger after a pick", async () => {
    const onPick = vi.fn();
    const user = userEvent.setup();
    const trigger = renderMenu(onPick);
    await user.click(trigger);
    await user.keyboard("{Enter}");
    expect(onPick).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    expect(document.activeElement).toBe(trigger);
  });

  it("returns focus to its trigger on Escape", async () => {
    const user = userEvent.setup();
    const trigger = renderMenu();
    await user.click(trigger);
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    expect(document.activeElement).toBe(trigger);
  });
});
