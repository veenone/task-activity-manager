import { useState } from "react";
import { describe, expect, it } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { useFocusWhen } from "./useFocusWhen";

// Editor reveals an input when the user clicks Edit, the way an inline editor
// or a dropdown's search box appears.
function Editor() {
  const [editing, setEditing] = useState<string | null>(null);
  const [draft, setDraft] = useState("");
  const ref = useFocusWhen<HTMLInputElement>(editing);
  return (
    <>
      <button onClick={() => setEditing("A-1")}>Edit A-1</button>
      <button onClick={() => setEditing("A-2")}>Edit A-2</button>
      <button>Elsewhere</button>
      {editing && (
        <input
          ref={ref}
          aria-label={`Name of ${editing}`}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
        />
      )}
    </>
  );
}

describe("useFocusWhen", () => {
  it("leaves focus alone until a user action reveals the control", () => {
    render(<Editor />);
    const elsewhere = screen.getByRole("button", { name: "Elsewhere" });
    elsewhere.focus();
    expect(document.activeElement).toBe(elsewhere);
  });

  it("focuses the revealed control, and again when the trigger changes", () => {
    render(<Editor />);
    fireEvent.click(screen.getByRole("button", { name: "Edit A-1" }));
    expect(document.activeElement).toBe(screen.getByLabelText("Name of A-1"));

    screen.getByRole("button", { name: "Elsewhere" }).focus();
    fireEvent.click(screen.getByRole("button", { name: "Edit A-2" }));
    expect(document.activeElement).toBe(screen.getByLabelText("Name of A-2"));
  });

  it("does not pull focus back while the user types elsewhere", () => {
    render(<Editor />);
    fireEvent.click(screen.getByRole("button", { name: "Edit A-1" }));
    const input = screen.getByLabelText("Name of A-1");
    const elsewhere = screen.getByRole("button", { name: "Elsewhere" });
    elsewhere.focus();
    fireEvent.change(input, { target: { value: "x" } });
    expect(document.activeElement).toBe(elsewhere);
  });
});
