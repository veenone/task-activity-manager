import { createRef, useState } from "react";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { MarkdownField, type MarkdownFieldHandle } from "./MarkdownField";

function Harness({ initial = "hello world", onCommit = vi.fn() }) {
  const [value, setValue] = useState(initial);
  const ref = createRef<MarkdownFieldHandle>();
  return (
    <>
      <MarkdownField
        ref={ref}
        value={value}
        onChange={setValue}
        onCommit={onCommit}
        placeholder="Action"
      />
      <button onMouseDown={(event) => event.preventDefault()} onClick={() => ref.current?.insertText("${USER}")}>Insert</button>
      <button onClick={() => ref.current?.insertText("earth", { start: 6, end: 11 })}>Replace</button>
      <button onClick={() => ref.current?.focus()}>Focus</button>
    </>
  );
}

describe("MarkdownField", () => {
  it("inserts at the native caret and commits only on the final blur", async () => {
    const onCommit = vi.fn();
    const user = userEvent.setup();
    render(<Harness onCommit={onCommit} />);

    await user.click(screen.getByRole("textbox"));
    const editor = screen.getByRole("textbox") as HTMLTextAreaElement;
    editor.setSelectionRange(5, 5);
    await user.click(screen.getByRole("button", { name: "Insert" }));

    expect(editor).toHaveValue("hello${USER} world");
    expect(editor.selectionStart).toBe(12);
    expect(editor.selectionEnd).toBe(12);
    expect(document.activeElement).toBe(editor);
    expect(onCommit).not.toHaveBeenCalled();

    await user.tab();
    expect(onCommit).toHaveBeenCalledTimes(1);
  });

  it("replaces only an explicit selection and restores focus after render", async () => {
    const user = userEvent.setup();
    render(<Harness />);

    await user.click(screen.getByRole("button", { name: "Replace" }));
    const editor = await screen.findByRole("textbox") as HTMLTextAreaElement;
    expect(editor).toHaveValue("hello earth");
    expect(editor.selectionStart).toBe(11);
    expect(editor.selectionEnd).toBe(11);

    await user.click(screen.getByRole("button", { name: "Focus" }));
    await act(async () => {});
    expect(document.activeElement).toBe(screen.getByRole("textbox"));
  });
});
