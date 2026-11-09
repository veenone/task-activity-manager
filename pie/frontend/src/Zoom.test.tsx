import { expect, it } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Zoom } from "./Zoom";

function show() {
  render(
    <Zoom label="Project flow">
      <svg role="img" aria-label="chart" />
    </Zoom>,
  );
  const area = screen.getByRole("img", { name: "chart" }).parentElement!;
  return { user: userEvent.setup(), area };
}

it("zooms in and out in steps, says the level, and resets", async () => {
  const { user, area } = show();
  expect(screen.getByText("100%")).toBeVisible();
  await user.click(screen.getByRole("button", { name: "Zoom in" }));
  await user.click(screen.getByRole("button", { name: "Zoom in" }));
  expect(screen.getByText("156%")).toBeVisible();
  expect(area).toHaveStyle({ width: "156.25%" });
  await user.click(screen.getByRole("button", { name: "Zoom out" }));
  expect(screen.getByText("125%")).toBeVisible();
  await user.click(screen.getByRole("button", { name: "Reset zoom" }));
  expect(area).toHaveStyle({ width: "100%" });
});

it("zooms with Ctrl and the wheel, and stops at its limits", async () => {
  const { user, area } = show();
  fireEvent.wheel(area, { deltaY: -100, ctrlKey: true });
  expect(screen.getByText("125%")).toBeVisible();
  fireEvent.wheel(area, { deltaY: 100 });
  expect(screen.getByText("125%")).toBeVisible();
  for (let i = 0; i < 20; i += 1) await user.click(screen.getByRole("button", { name: "Zoom out" }));
  expect(screen.getByText("25%")).toBeVisible();
  expect(screen.getByRole("button", { name: "Zoom out" })).toBeDisabled();
});

it("zooms a trackpad pinch by as much as it moved, not a whole step each event", () => {
  const { area } = show();
  for (let i = 0; i < 6; i += 1) fireEvent.wheel(area, { deltaY: -2, ctrlKey: true });
  expect(screen.getByText("103%")).toBeVisible();
});
