import { describe, it, expect, vi } from "vitest";
import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TokenPicker, validateLabel } from "./TokenPicker";
import { Modal } from "@agile-suite/core";

function Harness(props: {
  initial?: string[];
  suggestions?: string[];
  allowCreate?: boolean;
  onCreate?: (v: string) => Promise<boolean>;
  spy?: (v: string[]) => void;
  separator?: RegExp | null;
  validate?: (v: string) => string | null;
}) {
  const [value, setValue] = useState(props.initial ?? []);
  return (
    <TokenPicker
      label="Labels"
      value={value}
      onChange={(v) => {
        setValue(v);
        props.spy?.(v);
      }}
      suggestions={props.suggestions ?? ["smoke", "Smoke", "login"]}
      allowCreate={props.allowCreate ?? true}
      onCreate={props.onCreate}
      validate={props.validate ?? validateLabel}
      separator={props.separator}
    />
  );
}

const input = () => screen.getByRole("combobox", { name: "Labels" });

describe("TokenPicker", () => {
  it("filters suggestions, keeping case variants, and picks with the keyboard", async () => {
    const spy = vi.fn();
    render(<Harness spy={spy} />);
    await userEvent.type(input(), "smo");
    const options = screen.getAllByRole("option").map((o) => o.textContent);
    expect(options).toEqual(["smoke", "Smoke", 'Create "smo"']);
    await userEvent.keyboard("{ArrowDown}{Enter}");
    expect(spy).toHaveBeenLastCalledWith(["Smoke"]);
  });

  it("hides suggestions already chosen", async () => {
    render(<Harness initial={["smoke"]} />);
    await userEvent.type(input(), "s");
    expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual([
      "Smoke",
      'Create "s"',
    ]);
  });

  it("creates an unknown value on Enter", async () => {
    const spy = vi.fn();
    render(<Harness spy={spy} />);
    await userEvent.type(input(), "regression{Enter}");
    expect(spy).toHaveBeenLastCalledWith(["regression"]);
    expect(screen.getByRole("button", { name: "Remove regression" })).toBeTruthy();
  });

  it("splits pasted whitespace into separate values", async () => {
    const spy = vi.fn();
    render(<Harness spy={spy} />);
    await userEvent.click(input());
    await userEvent.paste("a b  c");
    expect(spy).toHaveBeenLastCalledWith(["a", "b", "c"]);
  });

  it("shows the validation error and refuses the add", async () => {
    const spy = vi.fn();
    render(<Harness spy={spy} />);
    await userEvent.click(input());
    await userEvent.paste("x".repeat(256));
    await userEvent.keyboard("{Enter}");
    expect(screen.getByRole("alert").textContent).toBe(
      "A label can be at most 255 characters.",
    );
    expect(spy).not.toHaveBeenCalled();
  });

  it("does not add when onCreate resolves false", async () => {
    const spy = vi.fn();
    const onCreate = vi.fn(async () => false);
    render(<Harness spy={spy} onCreate={onCreate} />);
    await userEvent.type(input(), "newthing{Enter}");
    expect(onCreate).toHaveBeenCalledWith("newthing");
    expect(spy).not.toHaveBeenCalled();
  });

  it("without allowCreate, Enter on an unknown value adds nothing", async () => {
    const spy = vi.fn();
    render(<Harness spy={spy} allowCreate={false} />);
    await userEvent.type(input(), "nope{Enter}");
    expect(spy).not.toHaveBeenCalled();
    expect(screen.queryByText('Create "nope"')).toBeNull();
  });

  it("removes the last chip on Backspace in an empty input", async () => {
    const spy = vi.fn();
    render(<Harness initial={["a", "b"]} spy={spy} />);
    await userEvent.click(input());
    await userEvent.keyboard("{Backspace}");
    expect(spy).toHaveBeenLastCalledWith(["a"]);
  });

  it("Escape closes the list without closing the modal around it", async () => {
    const onClose = vi.fn();
    render(
      <Modal onClose={onClose} labelledBy="t">
        <h2 id="t">Labels</h2>
        <Harness />
      </Modal>,
    );
    await userEvent.type(input(), "smo");
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(onClose).not.toHaveBeenCalled();
    // A second Escape, with the list already closed, is the modal's again.
    await userEvent.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("keeps focus in the picker after a chip is removed", async () => {
    render(<Harness initial={["a", "b"]} />);
    await userEvent.click(screen.getByRole("button", { name: "Remove a" }));
    expect(document.activeElement).toBe(input());
  });

  it("validateLabel rejects whitespace and over-long labels", () => {
    expect(validateLabel("ok")).toBeNull();
    expect(validateLabel("two words")).toBe("A label cannot contain spaces.");
    expect(validateLabel("x".repeat(256))).toBe(
      "A label can be at most 255 characters.",
    );
  });

  it("with separator null, keeps pasted and blurred text whole", async () => {
    const spy = vi.fn();
    render(
      <>
        <Harness
          spy={spy}
          separator={null}
          suggestions={["User Management"]}
          validate={() => null}
        />
        <button>elsewhere</button>
      </>,
    );
    await userEvent.click(input());
    await userEvent.paste("Data Platform");
    await userEvent.click(screen.getByRole("button", { name: "elsewhere" }));
    expect(spy).toHaveBeenLastCalledWith(["Data Platform"]);
  });
});
