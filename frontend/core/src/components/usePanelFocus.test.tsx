import { useState } from "react";
import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { usePanelFocus } from "./usePanelFocus";

function Panel({ itemKey, onClose }: { itemKey: string; onClose: () => void }) {
  const ref = usePanelFocus<HTMLElement>(onClose, itemKey);
  return (
    <aside ref={ref} aria-label={`Detail ${itemKey}`}>
      <button type="button">Edit {itemKey}</button>
      <input aria-label={`Note on ${itemKey}`} />
    </aside>
  );
}

function Grid() {
  const [open, setOpen] = useState("");
  return (
    <>
      <input aria-label="Filter" />
      {["A-1", "A-2"].map((k) => (
        <button key={k} type="button" onClick={() => setOpen(k)}>
          Row {k}
        </button>
      ))}
      {open && <Panel itemKey={open} onClose={() => setOpen("")} />}
    </>
  );
}

describe("usePanelFocus", () => {
  it("closes on Escape from inside and returns focus to the row that opened it", async () => {
    const user = userEvent.setup();
    render(<Grid />);
    screen.getByRole("button", { name: "Row A-2" }).focus();
    await user.keyboard("{Enter}");
    await user.click(screen.getByLabelText("Note on A-2"));
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("complementary")).not.toBeInTheDocument();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Row A-2" }));
  });

  it("closes on Escape from the row itself, which keeps focus", async () => {
    const user = userEvent.setup();
    render(<Grid />);
    await user.click(screen.getByRole("button", { name: "Row A-1" }));
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Row A-1" }));
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("complementary")).not.toBeInTheDocument();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Row A-1" }));
  });

  it("leaves an Escape typed into a field outside the panel alone", async () => {
    const user = userEvent.setup();
    render(<Grid />);
    await user.click(screen.getByRole("button", { name: "Row A-1" }));
    await user.click(screen.getByLabelText("Filter"));
    await user.keyboard("{Escape}");
    expect(screen.getByRole("complementary")).toHaveAccessibleName("Detail A-1");
  });

  it("returns to the latest opener when the panel switches items", async () => {
    const user = userEvent.setup();
    render(<Grid />);
    await user.click(screen.getByRole("button", { name: "Row A-1" }));
    await user.click(screen.getByRole("button", { name: "Row A-2" }));
    await user.click(screen.getByRole("button", { name: "Edit A-2" }));
    await user.keyboard("{Escape}");
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Row A-2" }));
  });
});
