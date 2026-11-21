import fs from "node:fs";
import path from "node:path";
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DialogBody, DialogFooter, DialogHeader } from "./DialogFrame";

const primitivesCss = fs
  .readFileSync(path.resolve(__dirname, "../../styles/primitives.css"), "utf8")
  .replace(/\/\*[\s\S]*?\*\//g, " ");

/** Declarations of the rule whose selector is exactly `selector`. */
function rule(selector: string): string {
  const start = primitivesCss.indexOf(`\n${selector} {`);
  if (start < 0) return "";
  const open = primitivesCss.indexOf("{", start);
  return primitivesCss.slice(open + 1, primitivesCss.indexOf("}", open));
}

describe("DialogFrame", () => {
  it("names the dialog in the header and closes from its close button", async () => {
    const onClose = vi.fn();
    render(
      <div>
        <DialogHeader titleId="t" title="Settings" onClose={onClose} />
        <DialogBody>Body text</DialogBody>
        <DialogFooter>
          <button>Save</button>
        </DialogFooter>
      </div>,
    );
    expect(screen.getByRole("heading", { name: "Settings" })).toHaveAttribute("id", "t");
    await userEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("cannot be closed from the header while the dialog is busy", () => {
    render(<DialogHeader titleId="t" title="Working" onClose={() => {}} disabled />);
    expect(screen.getByRole("button", { name: "Close" })).toBeDisabled();
  });

  // jsdom applies no stylesheet, so the arrangement is pinned on the rules.
  // The footer used to spread its buttons end to end (space-between), which
  // put a lone OK at the left and a Cancel and Save at opposite edges.
  it("groups footer buttons at the right, with a leading status line pushed left", () => {
    expect(rule(".pending-actions")).toMatch(/justify-content:\s*flex-end/);
    expect(rule(".pending-actions > :first-child:not(button):not(:only-child)")).toMatch(/margin-right:\s*auto/);
  });

  it("pads the body away from the dialog edge", () => {
    expect(rule(".bulk-body")).toMatch(/padding:\s*14px 16px/);
  });
});
