// App.css set 49 colours as hex literals and named two tokens, --warn-bg and
// --warn-fg, that are defined nowhere, so their fallbacks were the only value
// that ever applied. Nothing it coloured changed when the window went dark.
// jsdom applies no stylesheet, so these read the stylesheet and the token
// table and work out what a reader sees in each theme.
import { describe, expect, it } from "vitest";
import {
  appCss,
  contrast,
  coreStyle,
  declarationsOf,
  resolve,
  tokenTable,
  tokensRead,
  valueOf,
} from "./test/cssRules";

const app = appCss();
const tokensCss = coreStyle("tokens.css");
const light = tokenTable(tokensCss, ":root");
const dark = new Map([...light, ...tokenTable(tokensCss, ':root[data-theme="dark"]')]);

// The two grid tracks a component measures from the keys on the page and sets
// on the element itself. They are widths, not colours, and no stylesheet can
// hold them.
const MEASURED_AT_RUNTIME = ["--issue-key-w", "--sprint-key-w"];

const isColour = (value: string) => /^(#|rgb|hsl|color-mix)/.test(value);

describe("the tokens App.css names", () => {
  it.each(tokensRead(app).filter((t) => !MEASURED_AT_RUNTIME.includes(t)))(
    "%s is defined in tokens.css rather than falling back to a literal",
    (token) => {
      expect(light.get(token)).toBeDefined();
    },
  );

  it.each(
    tokensRead(app)
      .filter((t) => !MEASURED_AT_RUNTIME.includes(t))
      .filter((t) => isColour(light.get(t) ?? "")),
  )("%s has a dark counterpart", (token) => {
    expect(tokenTable(tokensCss, ':root[data-theme="dark"]').get(token)).toBeDefined();
  });
});

describe("App.css leaves colour to the tokens", () => {
  it("names no colour of its own, so every colour it sets re-points with the theme", () => {
    const literals: string[] = [];
    for (const [, body] of app.replace(/\/\*[\s\S]*?\*\//g, " ").matchAll(/\{([^{}]*)\}/g)) {
      for (const [hit] of body.matchAll(
        // The lookahead keeps white-space and the other hyphenated property
        // names out; a colour keyword is never followed by a hyphen.
        /#[0-9a-fA-F]{3,8}\b|\b(?:rgba?|hsla?)\(|\b(?:white|black|silver|gray|grey)\b(?!-)/g,
      )) {
        literals.push(`${hit} in { ${body.trim()} }`);
      }
    }
    expect(literals).toEqual([]);
  });
});

// Each pair is a fill and the text drawn on it. A rule that sets no colour
// inherits the body's, which is var(--text).
const FILLED = [
  ".pending-card-conflict",
  ".chip-type-task",
  ".chip-type-story",
  ".chip-type-bug",
  ".chip-type-epic",
  ".chip-type-requirement",
  ".chip-type-subtask",
  ".chip-type-none",
  ".chip-status-todo",
  ".chip-status-active",
  ".chip-status-done",
  ".chip-label",
  ".chip-draft",
  ".chip-held",
  ".chip-conflict",
];

describe.each([
  ["light", light],
  ["dark", dark],
])("%s theme", (_theme, table) => {
  it.each(FILLED)("%s reads at 4.5 to 1 or better against its own fill", (selector) => {
    const declarations = declarationsOf(app, selector);
    const fill = valueOf(declarations, "background") ?? valueOf(declarations, "background-color");
    expect(fill).toBeDefined();
    const text = valueOf(declarations, "color") ?? "var(--text)";
    expect(contrast(resolve(fill as string, table), resolve(text, table))).toBeGreaterThanOrEqual(4.5);
  });
});
