import { describe, expect, it } from "vitest";
import catalogCss from "./datasets.css?raw";
import editorCss from "../test-detail/dataset.css?raw";
import macroCss from "../test-detail/dataset-macro.css?raw";

// classes is every class a stylesheet's selectors name, comments stripped.
function classes(text: string): Set<string> {
  const css = text.replace(/\/\*[\s\S]*?\*\//g, "");
  const selectors = css.replace(/\{[^}]*\}/g, ",");
  return new Set([...selectors.matchAll(/\.([a-zA-Z][\w-]*)/g)].map((m) => m[1]));
}

describe("the Datasets view's stylesheet", () => {
  // #307: the catalog table and the test panel's dataset editor both used
  // .dataset-table, so the catalog drew the editor's bordered grid with its
  // small type. Both stylesheets load globally, so a class one defines must
  // not be one the other defines.
  it("shares no class with the dataset editor's stylesheet", () => {
    const catalog = classes(catalogCss);
    const editor = new Set([...classes(editorCss), ...classes(macroCss)]);
    expect([...catalog].filter((c) => editor.has(c))).toEqual([]);
  });
});
