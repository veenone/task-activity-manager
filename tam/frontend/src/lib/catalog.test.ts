import { describe, expect, it } from "vitest";
import type { Component, Version } from "../api";
import { catalogNames, catalogOptions, idsText, parseIds } from "./catalog";

const v = (id: number, name: string, extra: Partial<Version> = {}): Version => ({
  id, project: "PLAT", name, description: "", startDate: "", releaseDate: "", released: false, archived: false, draft: false, ...extra,
});
const c = (id: number, name: string): Component => ({
  id, project: "PLAT", name, description: "", leadName: "", leadDisplayName: "", assigneeType: "", draft: false,
});

describe("catalog helpers", () => {
  it("reads and writes the journal's id list", () => {
    expect(parseIds("10001, 10000,, 10001")).toEqual([10001, 10000]);
    expect(parseIds("")).toEqual([]);
    expect(idsText([10000, 10001])).toBe("10000, 10001");
  });

  // A shipped release stays on the issues it shipped with and cannot be
  // picked again, so it is offered only to be cleared.
  it("disables a released or archived release unless the issue has it", () => {
    const rows = [v(1, "v1.2"), v(2, "v1.1", { released: true }), v(3, "v1.0", { archived: true }), v(-1, "v2.0", { draft: true })];
    const opts = catalogOptions(rows, [3]);
    expect(opts.map((o) => [o.value, o.label, !!o.disabled])).toEqual([
      ["1", "v1.2", false],
      ["2", "v1.1 (released)", true],
      ["3", "v1.0 (archived)", false],
      ["-1", "v2.0 (draft)", false],
    ]);
  });

  it("offers every component", () => {
    expect(catalogOptions([c(10, "Auth")], []).map((o) => [o.value, o.label, !!o.disabled])).toEqual([["10", "Auth", false]]);
  });

  // An id the cache no longer holds still shows, as its number, rather than
  // vanishing from the issue.
  it("names ids, keeping one it does not know", () => {
    expect(catalogNames([c(10, "Auth"), c(11, "Board")], "11, 99")).toBe("Board, 99");
    expect(catalogNames([], "")).toBe("");
  });
});
