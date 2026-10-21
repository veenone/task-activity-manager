// jsdom applies no stylesheet, so no render test can see a control whose box
// is shorter than the text inside it. These read the stylesheet text, the way
// styles.test.ts does, and pin the two rules that stop it happening.
//
// The bug they come from (#323): .detail-input left line-height at `normal`
// and the filter bar forced `height: 30px`. With box-sizing: border-box that
// is 30 - 2 border - 12 padding = 16px of content box, and a 13px glyph in
// Segoe UI, whose normal line height is about 1.33, wants 17.3px. The
// descenders were cut off. clientHeight and scrollHeight both read 28px while
// it happened, because the box was not overflowing; the line box inside it
// was.
import { describe, expect, it } from "vitest";
import { appCss as readApp, coreStyle, declarationsMentioning, declarationsOf, valueOf } from "./test/cssRules";

const appCss = readApp();
const primitivesCss = coreStyle("primitives.css");

describe("control sizing", () => {
  // `normal` is whatever the platform's font says, and the font is not the
  // same on every machine the app runs on. Stating the line height is what
  // makes the height below a number this stylesheet controls.
  it("states a line height rather than leaving it to whichever font the platform picks", () => {
    expect(declarationsOf(primitivesCss, ".detail-input")).toMatch(/line-height:\s*[\d.]/);
  });

  // A floor lets a control grow when its text needs more room. A ceiling
  // makes the text the thing that gives way, and text gives way by being cut.
  it("the filter bar sets a floor on control height, never a ceiling", () => {
    const bar = declarationsMentioning(appCss, ".filter-bar .detail-input");
    expect(bar).toMatch(/min-height:\s*\d/);
    expect(bar).not.toMatch(/(^|[^-])height:\s*\d/);
  });

  // The row still has one height. It can grow past it, which is the whole
  // difference: the old `height` made the text give way instead. This moved
  // here from styles.test.ts with the fix, so control sizing reads in one
  // place.
  it.each([".filter-bar .detail-input", ".filter-bar .btn"])("gives %s the bar's one floor", (selector) => {
    const rule = declarationsOf(appCss, selector);
    expect(valueOf(rule, "min-height")).toMatch(/^\d+px$/);
    expect(valueOf(rule, "height")).toBeUndefined();
  });
});

// #329: the rituals sidebar shipped with its cards flush against the panel
// border and filled the same colour as the panel behind them.
describe("the rituals sidebar", () => {
  // .ritual-docs ul was declared twice, seven lines apart, and the second one
  // zeroed the padding the first had set. One rule per selector is what stops
  // the next edit landing in whichever copy the author happened to find.
  it("declares its list once", () => {
    const declarations = appCss.split(".ritual-docs ul {").length - 1;
    expect(declarations).toBe(1);
  });

  // A card whose fill is its container's fill is separated by its border
  // alone. XTM's preconditions panel is the reference: the cards are
  // --surface and the ground behind them is not.
  it("does not fill the ground behind the cards with the cards' own colour", () => {
    const layout = declarationsOf(appCss, ".rituals-layout");
    expect(valueOf(layout, "background")).not.toBe("var(--surface)");
  });
});
