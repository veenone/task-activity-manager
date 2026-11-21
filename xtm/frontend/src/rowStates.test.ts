// jsdom applies no stylesheet, so the row states are pinned on the rules.
// Hovering a row used to paint it exactly like the selected row (#539).
// The file is read from disk: with css: false in the test config a ?raw
// import of a stylesheet comes back empty, and every assertion would pass.
/// <reference types="node" />
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const here = path.dirname(fileURLToPath(import.meta.url));
const css = fs.readFileSync(path.join(here, "App.css"), "utf8").replace(/\/\*[\s\S]*?\*\//g, " ");

function background(selector: string): string | undefined {
  const start = css.indexOf(`\n${selector} {`);
  if (start < 0) return undefined;
  const open = css.indexOf("{", start);
  const body = css.slice(open + 1, css.indexOf("}", open));
  return /background:\s*([^;]+);/.exec(body)?.[1].trim();
}

describe("table row states", () => {
  it("hover and selected rows have different fills", () => {
    const hover = background("tbody tr:hover");
    const selected = background("tr.row-selected");
    expect(hover).toBeDefined();
    expect(selected).toBeDefined();
    expect(hover).not.toBe(selected);
  });

  it("hover uses the same token as TAM's rows", () => {
    expect(background("tbody tr:hover")).toBe("var(--row-hover)");
  });
});
