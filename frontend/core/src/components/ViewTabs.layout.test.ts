/**
 * The view tab row in a narrow window (#531). jsdom does no layout, so these
 * read the rules that decide it. A flex row centred with `justify-content:
 * center` overflows both edges once its tabs are wider than the window, and
 * the part past the left edge cannot be scrolled to. Auto margins on the row
 * centre it while it fits and collapse to zero when it does not, so the
 * overflow goes to the right, where the bar scrolls.
 */
import { describe, expect, it } from "vitest";
import fs from "node:fs";
import path from "node:path";

const repoRoot = path.resolve(__dirname, "../../../..");

function read(rel: string): string {
  return fs.readFileSync(path.join(repoRoot, rel), "utf8");
}

/** The declarations of the first rule whose selector is exactly `selector`. */
function rule(css: string, selector: string): Record<string, string> {
  const body = css.replace(/\/\*[\s\S]*?\*\//g, " ").match(
    new RegExp(`(?:^|\\})\\s*${selector.replace(/\./g, "\\.")}\\s*\\{([^}]*)\\}`),
  );
  if (!body) throw new Error(`no rule for ${selector}`);
  const out: Record<string, string> = {};
  for (const decl of body[1].split(";")) {
    const i = decl.indexOf(":");
    if (i > 0) out[decl.slice(0, i).trim()] = decl.slice(i + 1).trim();
  }
  return out;
}

describe("view tabs in a narrow window", () => {
  const css = read("frontend/core/styles/primitives.css");

  it("keeps the first tab reachable when the row is wider than the window", () => {
    const bar = rule(css, ".view-tabs-bar");
    const row = rule(css, ".view-tabs");
    expect(bar["overflow-x"]).toBe("auto");
    expect(bar["justify-content"] ?? "flex-start").not.toBe("center");
    expect(row["margin-inline"]).toBe("auto");
    expect(row["flex"]).toBe("none");
  });

  it.each(["tam/main.go", "xtm/main.go"])("%s sets a minimum window width", (file) => {
    const min = read(file).match(/MinWidth:\s*(\d+)/);
    expect(Number(min?.[1] ?? 0)).toBeGreaterThanOrEqual(800);
  });
});
