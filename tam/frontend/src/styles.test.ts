// The three bugs in issue #56 are cascade bugs: jsdom applies no stylesheet, so
// no render test can see them. These assertions read the stylesheet text and
// pin the rules whose absence caused each one. They fail against the code
// before the fix, which is what makes them worth keeping.
import { describe, expect, it } from "vitest";
import { appCss as readApp, coreStyle, declarationsMentioning, declarationsOf } from "./test/cssRules";

const appCss = readApp();
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

  it("no table indents a subtask with a literal of its own", () => {
    for (const selector of [".issue-row-subtask", ".nested-subtask", ".board-card-subtask"]) {
      const rules = rulesFor(appCss, selector);
      expect(rules).not.toMatch(/(padding|margin)-inline-start:\s*\d+px/);
    }
  });
});

describe("selected and hovered filter chips", () => {
  it("are told apart by more than a 1px border colour", () => {
    // One rule gave .chip-on and :hover the same opacity, so hovering an
    // unselected type filter made it look selected.
    const hover = rulesFor(appCss, ".chip-toggle:hover");
    const on = rulesFor(appCss, ".chip-toggle.chip-on");
    expect(hover).not.toMatch(/opacity:\s*1/);
    expect(on).toMatch(/background|box-shadow|font-weight/);
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
