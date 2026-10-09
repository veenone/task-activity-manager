import { describe, it, expect } from "vitest";
import { VIEWS, visibleViews } from "./views";
import type { ViewContext } from "./views";

const all: ViewContext = {
  supportsPreconditionObjects: true,
  supportsRequirementObjects: true,
  supportsComponentAdmin: true,
  showCoverage: true,
};

const none: ViewContext = {
  supportsPreconditionObjects: false,
  supportsRequirementObjects: false,
  supportsComponentAdmin: false,
  showCoverage: false,
};

describe("the view list", () => {
  // This is what #171 is for. The tab bar and the navigation rail both render
  // visibleViews, so there is nothing to keep in step: a view can only appear
  // in one and not the other if one of them stops calling this.
  it("is the only list, so the tabs and the rail cannot disagree", () => {
    const ids = visibleViews(all).map((v) => v.id);
    expect(ids).toEqual(VIEWS.map((v) => v.id));
  });

  it("hides a view whose capability the backend lacks", () => {
    const ids = visibleViews(none).map((v) => v.id);
    expect(ids).not.toContain("preconditions");
    expect(ids).not.toContain("requirements");
    expect(ids).not.toContain("components");
    expect(ids).not.toContain("coverage");
    // The unconditional ones are still there, so the filter is not just
    // emptying the list.
    expect(ids).toContain("browse");
    expect(ids).toContain("plans");
    expect(ids).toContain("misspellings");
  });

  it("turns one capability on without turning the others on", () => {
    const ids = visibleViews({ ...none, supportsComponentAdmin: true }).map(
      (v) => v.id,
    );
    expect(ids).toContain("components");
    expect(ids).not.toContain("coverage");
    expect(ids).not.toContain("preconditions");
  });

  // Coverage is a user preference rather than a backend capability, and is
  // the one entry that would be wrong to key off caps alone.
  it("offers Coverage from the preference, not from the backend", () => {
    expect(
      visibleViews({ ...none, showCoverage: true }).map((v) => v.id),
    ).toContain("coverage");
    expect(
      visibleViews({ ...all, showCoverage: false }).map((v) => v.id),
    ).not.toContain("coverage");
  });

  it("keeps Browse first, because that is where the app opens", () => {
    expect(visibleViews(none)[0].id).toBe("browse");
  });

  // The tour points at tabs by this hook, so a duplicate would make its
  // target ambiguous.
  it("gives every view a distinct tour hook", () => {
    const tours = VIEWS.map((v) => v.tour);
    expect(new Set(tours).size).toBe(tours.length);
  });
});
