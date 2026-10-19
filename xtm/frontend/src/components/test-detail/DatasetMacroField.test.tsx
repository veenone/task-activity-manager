import { useState } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { DatasetParameter } from "../../api";
import { DatasetMacroField } from "./DatasetMacroField";

const parameters: DatasetParameter[] = [
  { name: "USERNAME", combinatorial: false },
  { name: "ACCOUNT REGION", combinatorial: false },
  { name: "ACCOUNT ID", combinatorial: false },
];

function Harness({ initial = "Use old value", onCommit = vi.fn() }) {
  const [value, setValue] = useState(initial);
  return (
    <DatasetMacroField
      parameters={parameters}
      value={value}
      onChange={setValue}
      onCommit={onCommit}
      placeholder="Action"
    />
  );
}

describe("DatasetMacroField", () => {
  it("inserts a parameter over the selection and commits the final value once", async () => {
    const onCommit = vi.fn();
    const user = userEvent.setup();
    render(<Harness onCommit={onCommit} />);

    await user.click(screen.getByRole("textbox"));
    const editor = screen.getByRole("textbox") as HTMLTextAreaElement;
    editor.setSelectionRange(4, 7);
    await user.click(screen.getByRole("button", { name: "Insert dataset parameter" }));
    await user.click(screen.getByRole("option", { name: "ACCOUNT REGION" }));

    expect(editor).toHaveValue("Use ${ACCOUNT REGION} value");
    expect(document.activeElement).toBe(editor);
    expect(onCommit).not.toHaveBeenCalled();
    await user.tab();
    expect(onCommit).toHaveBeenCalledTimes(1);
  });

  it("filters after a typed trigger and supports arrows, Enter, and Escape", async () => {
    const user = userEvent.setup();
    render(<Harness initial="" />);
    await user.click(screen.getByRole("textbox"));
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "${ACCOUNT " } });

    expect(screen.queryByRole("option", { name: "USERNAME" })).not.toBeInTheDocument();
    expect(screen.getByRole("option", { name: "ACCOUNT REGION" })).toBeVisible();
    expect(screen.getByRole("option", { name: "ACCOUNT ID" })).toBeVisible();
    await user.keyboard("{ArrowDown}{Enter}");
    expect(screen.getByRole("textbox")).toHaveValue("${ACCOUNT ID}");

    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "${ACCOUNT ID} ${" },
    });
    expect(screen.getByRole("listbox", { name: "Dataset parameters" })).toBeVisible();
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("listbox", { name: "Dataset parameters" })).not.toBeInTheDocument();
  });

  it("keeps an undefined macro visible and explains it without relying on color", () => {
    render(<Harness initial="Use ${MISSING}" />);

    expect(screen.getByText(/Use.*MISSING/)).toBeVisible();
    expect(screen.getByText("Undefined dataset parameter: MISSING")).toBeVisible();
  });
});
