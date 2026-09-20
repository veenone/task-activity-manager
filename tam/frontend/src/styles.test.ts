// The three bugs in issue #56 are cascade bugs: jsdom applies no stylesheet, so
// no render test can see them. These assertions read the stylesheet text and
// pin the rules whose absence caused each one. They fail against the code
// before the fix, which is what makes them worth keeping.
import fs from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

const here = path.dirname(new URL(import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, "$1"));
const appCss = fs.readFileSync(path.join(here, "App.css"), "utf8");
const coreStyle = (name: string) =>
  fs.readFileSync(path.join(here, "..", "..", "..", "frontend", "core", "styles", name), "utf8");
const primitives = coreStyle("primitives.css");
const allCss = `${appCss}\n${primitives}\n${coreStyle("tokens.css")}`;

/** The declarations of every rule whose selector mentions `selector`. */
function rulesFor(css: string, selector: string): string[] {
  const flat = css.replace(/\/\*[\s\S]*?\*\//g, " ").replace(/\s+/g, " ");
  const out: string[] = [];
  // Split on the closing brace rather than matching whole rules with one
  // regex: a regex consumes the brace that delimits the next rule, so every
  // rule that follows a match directly goes unseen.
  for (const chunk of flat.split("}")) {
    const at = chunk.indexOf("{");
    if (at < 0) continue;
    const head = chunk.slice(0, at).trim();
    if (!head.includes(selector)) continue;
    out.push(`${head} { ${chunk.slice(at + 1).trim()} }`);
  }
  return out;
}

describe("edit-row layout", () => {
  it("the description head spans the whole row instead of the 96px label track", () => {
    const head = rulesFor(appCss, ".edit-row-head");
    expect(head.join(" ")).toMatch(/grid-column:\s*1\s*\/\s*-1/);
  });

  it("the description head wraps rather than overflowing its track", () => {
    const head = rulesFor(appCss, ".edit-row-head");
    expect(head.join(" ")).toMatch(/flex-wrap:\s*wrap/);
    expect(head.join(" ")).toMatch(/min-width:\s*0/);
  });
});

describe("ritual prose", () => {
  const prose = rulesFor(allCss, "ritual-editor-content").join(" ");

  it("plain bullet and numbered lists keep the padding their markers live in", () => {
    const lists = rulesFor(allCss, "ritual-editor-content ul").concat(
      rulesFor(allCss, "ritual-editor-content ol"),
    );
    const plain = lists.filter((r) => !r.includes("taskList"));
    expect(plain.join(" ")).toMatch(/padding-inline-start:/);
  });

  it("paragraphs and headings are separated", () => {
    expect(rulesFor(allCss, "ritual-editor-content p").join(" ")).toMatch(/margin-block:/);
    expect(rulesFor(allCss, "ritual-editor-content h2").join(" ")).toMatch(/margin/);
  });

  it("the measure is capped and the text has a line height", () => {
    expect(prose).toMatch(/max-width:\s*\d+ch/);
    expect(prose).toMatch(/line-height:/);
  });

  it("removing the editor outline leaves a visible focus indicator behind", () => {
    expect(prose).toMatch(/:focus-within/);
  });
});

describe("family row indentation", () => {
  it("the step a child is indented by is written once, as a custom property", () => {
    expect(rulesFor(allCss, ":root").join(" ")).toMatch(/--subtask-indent:\s*\d+px/);
    expect(rulesFor(allCss, ".row-lead").join(" ")).toMatch(
      /padding-inline-start:\s*calc\(\s*var\(--subtask-indent\)/,
    );
  });

  it("the toggle fills the slot the lead reserves rather than sizing itself", () => {
    // Its own 32px min-width plus a 6px margin is what made a parent's
    // summary start further right than its own subtask's.
    const toggle = rulesFor(appCss, ".subtask-toggle").join(" ");
    expect(toggle).not.toMatch(/min-width:/);
    expect(toggle).not.toMatch(/margin-inline-end:/);
  });

  it("no table indents a subtask with a literal of its own", () => {
    for (const selector of [".issue-row-subtask", ".nested-subtask", ".board-card-subtask"]) {
      const rules = rulesFor(appCss, selector).join(" ");
      expect(rules).not.toMatch(/(padding|margin)-inline-start:\s*\d+px/);
    }
  });
});

describe("classes the components already reference", () => {
  it.each([".ritual-banner-actions", ".ritual-page-body"])("%s resolves to a rule", (selector) => {
    expect(rulesFor(allCss, selector).length).toBeGreaterThan(0);
  });
});
