import { describe, it, expect } from "vitest";
import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { TOURS, TOUR_VERSION, shouldOfferTour } from "./steps";
import { VIEWS } from "../nav";

// src is where the components that carry the anchors live.
const src = join(__dirname, "..");

function everySource(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) everySource(path, out);
    else if (/\.tsx?$/.test(entry.name) && !/\.test\.tsx?$/.test(entry.name)) out.push(path);
  }
  return out;
}

// Anchors are set two ways. Most are literal; the view tabs and the two
// grids that serve more than one view build theirs from the view id, so a
// template is read as the shape it can produce (`tab-${v.id}` becomes
// "tab-" + anything). A step matches if a literal or a shape covers it.
const sources = everySource(src).map((f) => readFileSync(f, "utf8"));
const literal = new Set(
  sources.flatMap((text) => [...text.matchAll(/data-tour="([^"]+)"/g)].map((m) => m[1])),
);
const shapes = sources
  .flatMap((text) => [...text.matchAll(/data-tour=\{`([^`]+)`\}/g)].map((m) => m[1]))
  .map((t) => new RegExp("^" + t.replace(/\$\{[^}]+\}/g, "[A-Za-z0-9_-]+") + "$"));

const anchored = (target: string) => literal.has(target) || shapes.some((re) => re.test(target));

describe("the tours", () => {
  it("offers one for the first run and one for every view", () => {
    expect(TOURS.start.length).toBeGreaterThan(0);
    for (const v of VIEWS) {
      expect(TOURS[v.id], `${v.id} has no tour`).toBeTruthy();
      expect(TOURS[v.id].length, `${v.id}'s tour is empty`).toBeGreaterThan(0);
    }
  });

  // A step whose target is nowhere in the app is a step that silently
  // vanishes at run time: the hook drops it rather than pointing at the
  // page. This is the gate that says so at build time instead.
  it("points every step at an anchor some component actually sets", () => {
    // M2: the scan has to have found something, or the check below passes
    // by reading no files at all.
    expect(literal.size + shapes.length).toBeGreaterThan(0);
    const missing: string[] = [];
    for (const [tour, steps] of Object.entries(TOURS)) {
      for (const step of steps) {
        if (!anchored(step.target)) missing.push(`${tour}/${step.id} -> ${step.target}`);
      }
    }
    expect(missing, missing.join(", ")).toEqual([]);
  });

  it("gives every step its own id within its tour", () => {
    for (const [tour, steps] of Object.entries(TOURS)) {
      const ids = steps.map((s) => s.id);
      expect(new Set(ids).size, `${tour} repeats a step id`).toBe(ids.length);
    }
  });

  it("carries a version, so a rewritten tour can be offered again", () => {
    expect(TOUR_VERSION).toBeGreaterThan(0);
  });

  it("offers the first-run tour after the first sync, once", () => {
    // Nothing synced yet: a tour of an empty backlog explains nothing.
    expect(shouldOfferTour(0, false)).toBe(false);
    expect(shouldOfferTour(0, true)).toBe(true);
    // Seen it: not again.
    expect(shouldOfferTour(TOUR_VERSION, true)).toBe(false);
    // An older version seen means a rewritten tour is offered again.
    expect(shouldOfferTour(TOUR_VERSION - 1, true)).toBe(true);
  });
});
