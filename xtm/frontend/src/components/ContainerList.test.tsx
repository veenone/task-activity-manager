import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ContainerList } from "./ContainerList";
import type { Container } from "../api";

// The sidebar rows were divs with only a click handler, so a keyboard user
// could neither reach nor pick one. Each row has to answer Enter and Space
// the way it answers a click.
describe("ContainerList keyboard", () => {
  const containers = [{ key: "TP-1", summary: "Payments" } as Container];

  it("picks a container with Enter", async () => {
    const onSelect = vi.fn();
    render(<ContainerList containers={containers} selected="" emptyLabel="None" onSelect={onSelect} />);

    screen.getByRole("button", { name: /Payments/ }).focus();
    await userEvent.keyboard("{Enter}");

    expect(onSelect).toHaveBeenCalledWith("TP-1");
  });

  it("goes back to All tests with Space", async () => {
    const onSelect = vi.fn();
    render(<ContainerList containers={containers} selected="TP-1" emptyLabel="None" onSelect={onSelect} />);

    screen.getByRole("button", { name: /All tests/ }).focus();
    await userEvent.keyboard(" ");

    expect(onSelect).toHaveBeenCalledWith("");
  });
});
