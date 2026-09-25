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

  it("the text has a line height", () => {
    expect(prose).toMatch(/line-height:/);
  });

  // Issue #63 finding 3. #56 capped every prose surface at 72 characters,
  // the editor included, so a wide pane drew the text in a column with a
  // band of empty pane beside it and nothing to say where the field ended.
  // The cap belongs to prose someone reads, not to the box they type in.
  it("the editor is an outlined box whose text uses the width of the pane", () => {
    // The exact selector: .ritual-editor-content th sets a border of its
    // own, so mentioning the name would pass on the table's rule.
    const box = declarationsOf(appCss, ".ritual-editor-content");
    expect(box).toMatch(/border:\s*1px solid var\(--border-strong\)/);
    expect(prose).not.toMatch(/max-width:\s*\d+ch/);
    expect(prose).not.toMatch(/margin-inline:\s*auto/);
  });

  it("keeps the measure on the prose that is only read", () => {
    expect(rulesFor(allCss, ".rich-text")).toMatch(/max-width:\s*\d+ch/);
    expect(rulesFor(allCss, ".ritual-page-body")).toMatch(/max-width:\s*\d+ch/);
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

// Issue #63 finding 2. The description read as bare text in a panel where
// every other field is a bordered box, so nothing said where it began or
// that it was a field at all.
describe("the description in the detail panel", () => {
  it("is drawn as a bordered field, the way the panel's other fields are", () => {
    const box = declarationsOf(appCss, ".detail-description");
    const field = declarationsOf(primitivesCss, ".detail-input");
    for (const prop of ["border", "border-radius", "background"]) {
      const want = new RegExp(`(?:^|[; ])${prop}: *([^;]+)`).exec(field)?.[1];
      expect(want, `.detail-input sets no ${prop}`).toBeTruthy();
      expect(box, `.detail-description ${prop}`).toContain(`${prop}: ${want}`);
    }
  });
});

describe("classes the components already reference", () => {
  it.each([".ritual-banner-actions", ".ritual-page-body"])("%s resolves to a rule", (selector) => {
    expect(rulesFor(allCss, selector)).not.toBe("");
  });
});

// The sprint row's narrow layout is a cascade rule too: jsdom applies no
// stylesheet and has no viewport, so nothing a render test can see says
// whether the timeline stacks or stays in a 170px track that no bar can say
// anything in.
describe("the sprint row below 900px", () => {
  it("gives the timeline the width under the sprint name rather than a track of its own", () => {
    const timeline = declarationsOf(appCss, ".sprint-cell-timeline");
    // valueOf takes the last declaration, and the narrow block comes after
    // the wide one, so this is what a 900px reader gets.
    expect(timeline).toMatch(/grid-column:\s*2\s*\/\s*-1/);
    expect(timeline).toMatch(/grid-row:\s*2/);
  });

  it("drops the timeline's own track from the row template at that width", () => {
    const row = declarationsOf(appCss, ".sprint-row");
    const templates = [...row.matchAll(/grid-template-columns: *([^;]+)/g)].map((m) => m[1].trim());
    expect(templates.length).toBe(2);
    // Five tracks at the narrow width against six at the wide one: the
    // timeline's is the one that goes.
    expect(templates[1].split(/(?<=\))\s+|\s+(?![^(]*\))/).length).toBeLessThan(
      templates[0].split(/(?<=\))\s+|\s+(?![^(]*\))/).length,
    );
  });
});

// Issue #65 item 1. The bar is the same flex row in the Backlog, Assigned to
// me and Epics views, and the critique found three things that stopped it
// reading as one strip: the controls sat on different centre lines, they
// were four different heights, and the search field's fixed width wrapped
// the bar long before the pane ran out of room.
describe("the filter bar", () => {
  it("drops the stacked-field margin the detail panel's inputs carry", () => {
    // .detail-input carries margin-bottom: 12px for the panel's stacked
    // fields. In a centred flex row that margin is counted in the item's
    // outer size, so the search box and the sprint select rode 6px above
    // the chips and the buttons beside them.
    expect(valueOf(declarationsOf(appCss, ".filter-bar .detail-input"), "margin-bottom")).toBe("0");
  });

  it.each([".filter-bar .detail-input", ".filter-bar .btn"])("gives %s the bar's one height", (selector) => {
    expect(valueOf(declarationsOf(appCss, selector), "height")).toMatch(/^\d+px$/);
  });

  it("lets the search field give way before the bar wraps", () => {
    // A fixed 300px search box plus the type chips plus the sprint select
    // wrapped the Backlog bar at about 1000px, which is a normal window.
    const search = declarationsOf(appCss, ".filter-search");
    expect(valueOf(search, "width")).toBeUndefined();
    expect(valueOf(search, "flex")).toBeTruthy();
  });

  it("ends every bar on its primary action", () => {
    // The Backlog bar pushed Import and + New to the end; the Epics bar's
    // + New epic sat wherever the buttons before it left it, so the two
    // bars ended differently.
    expect(valueOf(declarationsOf(appCss, ".filter-new"), "margin-left")).toBe("auto");
  });

  it("keeps its rows apart once it does wrap", () => {
    const bar = declarationsOf(appCss, ".filter-bar");
    expect(valueOf(bar, "row-gap")).toMatch(/^\d+px$/);
  });
});

// Issue #79. The chip is drawn in cells that clip, and in tracks that were
// sized before the sync started returning the project's whole vocabulary.
describe("chips", () => {
  it("is a box of its own, so the cells that clip do not cut its border", () => {
    // A chip is a span with padding and a border. As an inline box neither
    // contributes to the line box, so in a cell with overflow: hidden and a
    // row locked at 34px the top and bottom border were cut off. An
    // inline-flex box is atomic: the line box is its full height.
    const chip = declarationsOf(appCss, ".chip");
    expect(valueOf(chip, "display")).toBe("inline-flex");
    expect(valueOf(chip, "align-items")).toBe("center");
  });

  const TRACKED = [
    [".issue-row", "the Backlog grid"],
    [".sprint-issue-row", "the sprint tree"],
    [".epic-row", "the Epics tree"],
  ] as const;

  it.each(TRACKED)("%s sizes its type track from the page, not a literal", (selector) => {
    // 104px in the Backlog and 44px in both trees, against names like
    // Improvement and Sub Test Execution that the sync now returns.
    const css = selector === ".epic-row" ? primitivesCss : appCss;
    const tracks = valueOf(declarationsOf(css, selector), "grid-template-columns");
    expect(tracks).toContain("var(--type-col-w");
  });
});

describe("sprint band heading", () => {
  it("reads as a band rather than as another row", () => {
    // Only a bottom border separated it from the cards under it, which is
    // the same separator the cards use between themselves.
    const band = declarationsOf(appCss, ".sprint-group");
    expect(valueOf(band, "background")).toBeTruthy();
  });
});

// Issue #85. The view is fixed height: only the velocity table scrolls, and
// the page itself does not. A critique measured the budget at roughly 640px
// against about 1050px of content.
/** The stylesheet with every @media block removed, for asserting on a base
 *  rule that a media query deliberately overrides. */
function withoutMediaBlocks(css: string): string {
  let out = "";
  for (let i = 0; i < css.length; i++) {
    if (!css.startsWith("@media", i)) {
      out += css[i];
      continue;
    }
    let depth = 0;
    for (; i < css.length; i++) {
      if (css[i] === "{") depth++;
      else if (css[i] === "}" && --depth === 0) break;
    }
  }
  return out;
}

describe("reports view fits its frame", () => {
  it("the body does not scroll and lays its children out in a column", () => {
    // The base rule, with the media blocks taken out: those deliberately
    // hand the scrollbar back at a narrow or a short window, and
    // declarationsOf collects every matching rule, overrides included.
    const body = declarationsOf(withoutMediaBlocks(appCss), ".report-body");
    expect(valueOf(body, "overflow")).toBe("hidden");
    expect(valueOf(body, "display")).toBe("flex");
    expect(valueOf(body, "flex-direction")).toBe("column");
    expect(valueOf(body, "min-height")).toBe("0");
  });

  it("the velocity table's panel is the one thing that scrolls", () => {
    // All three, or the panel grows to its content instead of scrolling.
    const wrap = declarationsOf(appCss, ".report-table-wrap");
    expect(valueOf(wrap, "overflow")).toBe("auto");
    expect(valueOf(wrap, "flex")).toBe("1");
    expect(valueOf(wrap, "min-height")).toBe("0");
  });

  it("does not make the panel scroll sideways as well", () => {
    // .report-table carried min-width: 420px, which gave the panel a second
    // axis to scroll on inside a column already narrow enough.
    expect(valueOf(declarationsOf(appCss, ".report-table"), "min-width")).toBeUndefined();
  });

  it("keeps the table's header visible while its rows scroll under it", () => {
    expect(declarationsMentioning(appCss, ".report-table thead")).toMatch(/position:\s*sticky/);
  });

  it("shows the panel's own focus ring, since it is a scroller and takes focus", () => {
    expect(declarationsMentioning(appCss, ".report-table-wrap:focus-visible")).toMatch(/box-shadow|outline/);
  });

  it("gives the loading and unavailable states the frame rather than stretching them", () => {
    // They are single children of a flex column, so without this they are
    // stretched down the page by the panel's flex: 1 sibling.
    const centred = declarationsMentioning(appCss, ".report-body > .report-unavailable");
    expect(centred).toMatch(/place-content:\s*center/);
  });

  const FALLBACK = [
    ["a narrow window", "max-width: 900px"],
    ["a short window", "max-height"],
  ] as const;

  it.each(FALLBACK)("lets the page scroll again in %s", (_label, query) => {
    // The charts are fixed heights that do not shrink, so below a floor the
    // honest answer is a scrollbar rather than crushed content.
    const at = appCss.slice(appCss.indexOf(`@media (${query}`));
    expect(at.slice(0, 600)).toMatch(/\.report-body\s*\{[^}]*overflow:\s*auto/);
  });
});
