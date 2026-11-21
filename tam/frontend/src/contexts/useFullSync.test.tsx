import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DialogProvider } from "@agile-suite/core";
import { useFullSync } from "./useFullSync";

// Full sync refetches every issue and can run for minutes on a large
// project, so it asks first, the way XTM's full resync does (#534).

function Harness({ runSync, canSync = true }: { runSync: (full: boolean) => Promise<void>; canSync?: boolean }) {
  const fullSync = useFullSync(runSync, canSync);
  return <button onClick={() => void fullSync()}>Full sync</button>;
}

function renderHarness(runSync: (full: boolean) => Promise<void>, canSync?: boolean) {
  render(
    <DialogProvider>
      <Harness runSync={runSync} canSync={canSync} />
    </DialogProvider>,
  );
}

async function answer(choice: "Cancel" | "Full sync") {
  const dialog = await screen.findByRole("alertdialog", { name: "Full sync" });
  const buttons = within(dialog).getAllByRole("button", { name: choice });
  await userEvent.click(buttons[buttons.length - 1]);
}

describe("useFullSync", () => {
  it("runs a full sync only after the user confirms", async () => {
    const runSync = vi.fn(async () => {});
    renderHarness(runSync);
    await userEvent.click(screen.getByRole("button", { name: "Full sync" }));
    expect(runSync).not.toHaveBeenCalled();
    await answer("Full sync");
    expect(runSync).toHaveBeenCalledWith(true);
  });

  it("does nothing when the user cancels", async () => {
    const runSync = vi.fn(async () => {});
    renderHarness(runSync);
    await userEvent.click(screen.getByRole("button", { name: "Full sync" }));
    await answer("Cancel");
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(runSync).not.toHaveBeenCalled();
  });

  it("does not ask while a sync cannot start", async () => {
    const runSync = vi.fn(async () => {});
    renderHarness(runSync, false);
    await userEvent.click(screen.getByRole("button", { name: "Full sync" }));
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(runSync).not.toHaveBeenCalled();
  });
});
