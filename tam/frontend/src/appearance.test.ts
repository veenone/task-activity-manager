// A control's states are drawn by the stylesheet alone, and jsdom applies no
// stylesheet, so a render test cannot see them. These read the rules instead.
// The frames and the family indent are pinned the same way in styles.test.ts.
import { describe, expect, it } from "vitest";
import { appCss, declarationsMentioning, declarationsOf, valueOf } from "./test/cssRules";

const app = appCss();

describe("the type filter's chips", () => {
  const hover = declarationsMentioning(app, ".chip-toggle:hover");
  const selected = declarationsOf(app, ".chip-toggle.chip-on");

  it("hovering an unselected chip does not fade it up to a selected one", () => {
    expect(valueOf(hover, "opacity")).toBeDefined();
    expect(valueOf(selected, "opacity")).toBeDefined();
    expect(valueOf(hover, "opacity")).not.toBe(valueOf(selected, "opacity"));
  });

  it("selection is marked by more than the colour of a 1px border", () => {
    expect(selected).toMatch(/box-shadow:|outline:|font-weight:/);
  });
});
