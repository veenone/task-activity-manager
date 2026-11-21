// jsdom applies no stylesheet, so these read App.css as text.
import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

const appCss = readFileSync(path.resolve(__dirname, "App.css"), "utf8").replace(/\/\*[\s\S]*?\*\//g, "");

// The selector of every rule in the sheet, as written.
const selectors = [...appCss.matchAll(/([^{}]+)\{/g)].map((m) => m[1].trim());

describe("rules core already owns", () => {
  // XTM kept its own .btn-danger and .board-head, so a change to core's
  // version never reached XTM and the two apps drew them differently (#542).
  it.each([".btn-danger", ".btn-danger:hover:not(:disabled)", ".board-head"])("does not redefine %s", (selector) => {
    expect(selectors).not.toContain(selector);
  });
});
