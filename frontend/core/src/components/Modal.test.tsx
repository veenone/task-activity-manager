import { useEffect, useRef, useState } from "react";
import { describe, expect, it } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { Modal } from "./Modal";

// Opener renders a trigger button and, once clicked, a Modal whose first
// focusable control is a close button. The input named by `initialFocus`
// comes after it, so a dialog that ignores the prop focuses the wrong element.
function Opener({ named }: { named: boolean }) {
  const [open, setOpen] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  return (
    <>
      <button onClick={() => setOpen(true)}>Open</button>
      {open && (
        <Modal
          onClose={() => setOpen(false)}
          label="Rename"
          initialFocus={named ? inputRef : undefined}
        >
          <button onClick={() => setOpen(false)}>Close</button>
          <input ref={inputRef} aria-label="New name" />
        </Modal>
      )}
    </>
  );
}

function openFrom(named: boolean): HTMLElement {
  render(<Opener named={named} />);
  const opener = screen.getByRole("button", { name: "Open" });
  opener.focus();
  fireEvent.click(opener);
  return opener;
}

// GrabsFocus moves focus to itself from its own mount effect, which runs before
// the Modal's, the way an autoFocus attribute or a child's focus effect does.
function GrabsFocus() {
  const ref = useRef<HTMLInputElement>(null);
  useEffect(() => ref.current?.focus(), []);
  return <input ref={ref} aria-label="Eager" />;
}

describe("Modal initial focus", () => {
  it("focuses the element named by initialFocus when it opens", () => {
    openFrom(true);
    expect(document.activeElement).toBe(screen.getByLabelText("New name"));
  });

  it("falls back to the first focusable control without initialFocus", () => {
    openFrom(false);
    expect(document.activeElement).toBe(
      screen.getByRole("button", { name: "Close" }),
    );
  });

  it("returns focus to the opener when Escape closes it", () => {
    const opener = openFrom(true);
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.activeElement).toBe(opener);
  });

  it("keeps Tab inside the dialog after focusing the named element", () => {
    openFrom(true);
    const input = screen.getByLabelText("New name");
    // jsdom has no layout, so offsetParent is null for every element and the
    // trap sees no candidates; it must still hold focus inside the dialog.
    fireEvent.keyDown(input, { key: "Tab" });
    expect(screen.getByRole("dialog").contains(document.activeElement)).toBe(
      true,
    );
  });

  it("returns focus to the opener even when a child took focus while mounting", () => {
    function EagerOpener() {
      const [open, setOpen] = useState(false);
      return (
        <>
          <button onClick={() => setOpen(true)}>Open</button>
          {open && (
            <Modal onClose={() => setOpen(false)} label="Eager">
              <GrabsFocus />
            </Modal>
          )}
        </>
      );
    }
    render(<EagerOpener />);
    const opener = screen.getByRole("button", { name: "Open" });
    opener.focus();
    fireEvent.click(opener);
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    expect(document.activeElement).toBe(opener);
  });
});
