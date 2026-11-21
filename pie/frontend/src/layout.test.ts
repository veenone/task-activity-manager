// jsdom lays nothing out, so this reads the window's minimum width from
// main.go and the side panes' widths from App.css, and does the sum a narrow
// window does (#552): the Link map rail, the graph options and the details
// pane sit beside the canvas, and the canvas gets what is left.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { expect, it } from "vitest";

const css = readFileSync(resolve(__dirname, "App.css"), "utf8");
const mainGo = readFileSync(resolve(__dirname, "../../main.go"), "utf8");

// paneWidth is a pane's width in a window `at` pixels wide, from a plain
// `width: Npx` or a `width: clamp(Apx, Bvw, Cpx)`.
function paneWidth(selector: string, at: number): number {
  const rule = new RegExp(`\\n${selector.replace(".", "\\.")}\\s*\\{([^}]*)\\}`).exec(css);
  const decl = rule && /(?:^|[;\s])width:\s*([^;]+);/.exec(rule[1]);
  if (!decl) throw new Error(`no width for ${selector}`);
  const value = decl[1].trim();
  const clamp = /^clamp\((\d+)px,\s*([\d.]+)vw,\s*(\d+)px\)$/.exec(value);
  if (clamp) return Math.min(Number(clamp[3]), Math.max(Number(clamp[1]), (Number(clamp[2]) * at) / 100));
  const px = /^(\d+)px$/.exec(value);
  if (!px) throw new Error(`cannot read ${selector} width ${value}`);
  return Number(px[1]);
}

it("sets a minimum window width at which the graph canvas keeps room to work in", () => {
  const min = /MinWidth:\s*(\d+)/.exec(mainGo);
  expect(min, "main.go sets MinWidth").not.toBeNull();
  const at = Number(min![1]);
  // pie-main's padding both sides, the link map's gap, and the graph's two gaps.
  const chrome = 2 * 14 + 14 + 2 * 12;
  const panes = paneWidth(".pie-linkmap-rail", at) + paneWidth(".pie-graph-side", at) + paneWidth(".pie-graph-details", at);
  expect(at - chrome - panes).toBeGreaterThanOrEqual(320);
});

it("keeps the panes at their full width in the default 1280 pixel window", () => {
  expect(paneWidth(".pie-linkmap-rail", 1280)).toBe(250);
  expect(paneWidth(".pie-graph-side", 1280)).toBe(220);
  expect(paneWidth(".pie-graph-details", 1280)).toBe(280);
});
