import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ComponentList } from "./ComponentList";

// The sidebar rows were divs with only a click handler, so a keyboard user
// could neither reach nor pick one. Each row has to answer Enter and Space
// the way it answers a click.
describe("ComponentList keyboard", () => {
  const components = [{ label: "Payments", count: 3 }];

  it("picks a component with Enter", async () => {
    const onSelect = vi.fn();
    render(<ComponentList components={components} selected="" emptyLabel="None" onSelect={onSelect} />);

    screen.getByRole("button", { name: /Payments/ }).focus();
    await userEvent.keyboard("{Enter}");

    expect(onSelect).toHaveBeenCalledWith("Payments");
  });

  it("goes back to All tests with Space", async () => {
    const onSelect = vi.fn();
    render(<ComponentList components={components} selected="Payments" emptyLabel="None" onSelect={onSelect} />);

    screen.getByRole("button", { name: /All tests/ }).focus();
    await userEvent.keyboard(" ");

    expect(onSelect).toHaveBeenCalledWith("");
  });
});
