// The three bugs in issue #56 are cascade bugs: jsdom applies no stylesheet, so
// no render test can see them. These assertions read the stylesheet text and
// pin the rules whose absence caused each one. They fail against the code
// before the fix, which is what makes them worth keeping.
import { describe, expect, it } from "vitest";
import { appCss as readApp, coreStyle, declarationsMentioning, declarationsOf, valueOf } from "./test/cssRules";

const appCss = readApp();
const primitivesCss = coreStyle("primitives.css");
const allCss = [appCss, coreStyle("primitives.css"), coreStyle("tokens.css")].join("\n");

/** Declarations of every rule whose selector mentions `selector`, joined. */
const rulesFor = (css: string, selector: string) => declarationsMentioning(css, selector);

describe("edit-row layout", () => {
  it("the description head spans the whole row instead of the 96px label track", () => {
    const head = rulesFor(appCss, ".edit-row-head");
    expect(head).toMatch(/grid-column:\s*1\s*\/\s*-1/);
  });

  it("the description head wraps rather than overflowing its track", () => {
    const head = rulesFor(appCss, ".edit-row-head");
    expect(head).toMatch(/flex-wrap:\s*wrap/);
    expect(head).toMatch(/min-width:\s*0/);
  });
});

describe("ritual prose", () => {
  const prose = rulesFor(allCss, "ritual-editor-content");

  it("plain bullet and numbered lists keep the padding their markers live in", () => {
    const lists = `${rulesFor(allCss, "ritual-editor-content ul")} ${rulesFor(allCss, "ritual-editor-content ol")}`;
    expect(lists).toMatch(/padding-inline-start:/);
  });

  it("paragraphs and headings are separated", () => {
    expect(rulesFor(allCss, "ritual-editor-content p")).toMatch(/margin-block:/);
    expect(rulesFor(allCss, "ritual-editor-content h2")).toMatch(/margin/);
  });

  it("the measure is capped and the text has a line height", () => {
    expect(prose).toMatch(/max-width:\s*\d+ch/);
    expect(prose).toMatch(/line-height:/);
  });

  it("removing the editor outline leaves a visible focus indicator behind", () => {
    // The reader returns declarations, so the pseudo-class is asked for as a
    // selector of its own rather than searched for in a body.
    expect(declarationsOf(appCss, ".ritual-editor-content:focus-within")).toMatch(/box-shadow|outline/);
  });
});

describe("family row indentation", () => {
  it("the step a child is indented by is written once, as a custom property", () => {
    expect(rulesFor(allCss, ":root")).toMatch(/--subtask-indent:\s*\d+px/);
    expect(rulesFor(allCss, ".row-lead")).toMatch(
      /padding-inline-start:\s*calc\(\s*var\(--subtask-indent\)/,
    );
  });

  it("the toggle fills the slot the lead reserves rather than sizing itself", () => {
    // Its own 32px min-width plus a 6px margin is what made a parent's
    // summary start further right than its own subtask's.
    const toggle = rulesFor(appCss, ".subtask-toggle");
    expect(toggle).not.toMatch(/min-width:/);
    expect(toggle).not.toMatch(/margin-inline-end:/);
  });

  // Naming the selectors to check was the wrong shape: two of the three were
  // deleted along with their rules, so the assertion ran against an empty
  // string and passed whatever the stylesheet said. This reads every rule in
  // the app's own sheet instead, so a literal indent fails wherever it lands
  // and under whatever new name.
  it("no rule in the app indents anything with a pixel literal", () => {
    // Pixels only. The prose lists set their padding in em, which scales
    // with the text rather than standing in for the family indent.
    const offenders = appCss
      .split("}")
      .filter((rule) => /(padding|margin)-inline-start:\s*[\d.]+px/.test(rule))
      .map((rule) => rule.trim().split("\n").pop()?.trim());
    expect(offenders, `indent literals: ${offenders.join(" | ")}`).toEqual([]);
  });

  it("the shared token is what the family indent is spelled with", () => {
    expect(rulesFor(appCss, ".board-card-subtask")).toMatch(
      /margin-inline-start:\s*var\(--subtask-indent\)/,
    );
  });
});

// The chip states are pinned in appearance.test.ts, which compares the two
// opacity values rather than matching one of them by pattern.

// Issue #63 finding 1. The Reports view put a scrollbar on the window even
// though .main clips and .report-body scrolls inside it. The escapee was
// visually hidden content: .sr-only positions absolutely, nothing between it
// and the page is positioned, so its containing block is the page itself.
// ChartFrame's screen-reader data table is 300px tall whatever .sr-only says
// (width, height and overflow do not constrain a table box), so laid out at
// the foot of a scrolled report it grew the document's own scroll area, and
// the velocity table's 1px caption kept it alive after that.
describe("the app frame", () => {
  it("defines the visually hidden helper in one stylesheet", () => {
    const defining = [
      [".sr-only in App.css", declarationsOf(appCss, ".sr-only")],
      [".sr-only in primitives.css", declarationsOf(primitivesCss, ".sr-only")],
    ].filter(([, body]) => body !== "");
    expect(defining.map(([where]) => where)).toEqual([".sr-only in primitives.css"]);
  });

  it("takes visually hidden content out of every scroll container", () => {
    // The app's sheet is imported last, so its copy is the one that runs.
    const effective = declarationsOf(appCss, ".sr-only") || declarationsOf(primitivesCss, ".sr-only");
    expect(valueOf(effective, "position")).toBe("fixed");
  });

  it("clips the main pane rather than letting it scroll", () => {
    const main = declarationsOf(appCss, ".main");
    expect(valueOf(main, "overflow")).toBe("hidden");
    expect(valueOf(main, "min-height")).toBe("0");
  });
});

describe("the two main panes", () => {
  it("frame the rituals page the way the report frame is framed", () => {
    // Exact selector: mentioning .ritual-page also collects .ritual-page-body
    // th, which has a border of its own and made this pass before the fix.
    const page = declarationsOf(appCss, ".ritual-page");
    const frame = declarationsOf(appCss, ".report-frame");
    for (const prop of ["border", "border-radius", "background"]) {
      // A leading boundary, so `border:` does not read `border-radius:`.
      const want = new RegExp(`(?:^|[; ])${prop}: *([^;]+)`).exec(frame)?.[1];
      expect(want, `.report-frame sets no ${prop}`).toBeTruthy();
      expect(page, `.ritual-page ${prop}`).toContain(`${prop}: ${want}`);
    }
  });
});

describe("classes the components already reference", () => {
  it.each([".ritual-banner-actions", ".ritual-page-body"])("%s resolves to a rule", (selector) => {
    expect(rulesFor(allCss, selector)).not.toBe("");
  });
});
