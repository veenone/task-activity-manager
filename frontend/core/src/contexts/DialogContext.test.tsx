import React from "react";
import { describe, it, expect, vi } from "vitest";
import {
  render,
  screen,
  waitFor,
  renderHook,
  fireEvent,
} from "@testing-library/react";
import { DialogProvider, useDialogs } from "./DialogContext";
import { useConfirm } from "../components/useConfirm";

// LiveRegion.announce touches a module-level DOM node; stub it so the notice
// path doesn't depend on it in jsdom.
vi.mock("../components/LiveRegion", () => ({ announce: vi.fn() }));

describe("useDialogs", () => {
  it("throws when used outside a DialogProvider", () => {
    // Silence the expected error boundary console noise.
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() =>
      renderHook(() => useDialogs()),
    ).toThrow(/DialogProvider/);
    spy.mockRestore();
  });
});

describe("DialogProvider", () => {
  it("resolves confirm(true) when the confirm button is clicked", async () => {
    const onResult = vi.fn();

    function Trigger() {
      const { confirm } = useConfirm();
      return (
        <button
          onClick={async () => onResult(await confirm({ title: "Delete it?" }))}
        >
          go
        </button>
      );
    }

    render(
      <DialogProvider>
        <Trigger />
      </DialogProvider>,
    );

    fireEvent.click(screen.getByText("go"));
    // The dialog renders once at the provider root.
    expect(await screen.findByText("Delete it?")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(onResult).toHaveBeenCalledWith(true));
    // Dialog is dismissed after resolving.
    expect(screen.queryByText("Delete it?")).not.toBeInTheDocument();
  });

  it("opens a confirm on its Cancel button and a prompt on its input", async () => {
    function Trigger() {
      const { confirm, prompt, notice } = useDialogs();
      return (
        <>
          <button onClick={() => void confirm({ title: "Delete it?" })}>c</button>
          <button onClick={() => void prompt({ title: "Name?" })}>p</button>
          <button onClick={() => void notice({ title: "Done" })}>n</button>
        </>
      );
    }
    render(
      <DialogProvider>
        <Trigger />
      </DialogProvider>,
    );

    fireEvent.click(screen.getByText("c"));
    await screen.findByText("Delete it?");
    expect(document.activeElement).toBe(screen.getByText("Cancel"));
    fireEvent.click(screen.getByText("Cancel"));

    fireEvent.click(screen.getByText("p"));
    await screen.findByText("Name?");
    expect(document.activeElement).toBe(screen.getByRole("textbox"));
    fireEvent.keyDown(screen.getByRole("textbox"), { key: "Escape" });

    fireEvent.click(screen.getByText("n"));
    await screen.findByText("Done");
    expect(document.activeElement).toBe(screen.getByText("OK"));
  });
});
