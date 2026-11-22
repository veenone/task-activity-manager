/// <reference types="node" />
// jsdom applies no stylesheet, so these read App.css as text. Through fs,
// not a ?raw import: with css off in the test config, ?raw hands back "".
import fs from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

const appCss = fs.readFileSync(path.join(__dirname, "App.css"), "utf8").replace(/\/\*[\s\S]*?\*\//g, "");

// The selector of every rule in the sheet, as written.
const selectors = [...appCss.matchAll(/([^{}]+)\{/g)].map((m) => m[1].trim());

describe("rules core already owns", () => {
  it("reads the stylesheet it checks", () => {
    expect(selectors).toContain(".table-footer");
  });

  // XTM kept its own .btn-danger and .board-head, so a change to core's
  // version never reached XTM and the two apps drew them differently (#542).
  it.each([".btn-danger", ".btn-danger:hover:not(:disabled)", ".board-head", ".bulk-toolbar"])("does not redefine %s", (selector) => {
    expect(selectors).not.toContain(selector);
  });
});

// Requirements drew its New and More actions buttons in a left-aligned row of
// its own, so the right-aligned menu opened past the panel's left edge and was
// cut in half. Preconditions' toolbar is the reference both panels share (#591).
describe("the Requirements toolbar", () => {
  const src = (file: string) => fs.readFileSync(path.join(__dirname, "components", file), "utf8");
  const requirements = src("RequirementsView.tsx");
  const preconditions = src(path.join("preconditions", "PreconditionList.tsx"));

  it("uses the Preconditions toolbar and button classes", () => {
    for (const cls of ['className="precond-list-toolbar"', 'className="btn btn-primary precond-new"', 'triggerClassName="btn precond-new"']) {
      expect(preconditions).toContain(cls);
      expect(requirements).toContain(cls);
    }
  });

  it("keeps no toolbar rules of its own", () => {
    expect(selectors).not.toContain(".reqs-list-toolbar");
    expect(selectors).not.toContain(".reqs-sources-btn");
  });
});
