/**
 * The navigation rail's shortcut (#528). A native menu accelerator fires
 * before the webview sees the key, so Ctrl+B both bolded text in TAM's
 * ritual editor and toggled the rail. The rail's accelerator has to be one no
 * editor binds, and each app's rail tooltip has to name the key the menu
 * actually uses.
 */
import { describe, expect, it } from "vitest";
import fs from "node:fs";
import path from "node:path";

const repoRoot = path.resolve(__dirname, "../../..");
const read = (rel: string) => fs.readFileSync(path.join(repoRoot, rel), "utf8");

/** The Navigation Rail item's accelerator, spelled the way a tooltip says it. */
function railShortcut(mainGo: string): string {
  const m = mainGo.match(/AddCheckbox\("Navigation Rail", [^,]+, keys\.CmdOrCtrl\("([^"]+)"\)/);
  if (!m) throw new Error("no Navigation Rail accelerator");
  return `Ctrl+${m[1].replace(/\\\\/g, "\\").toUpperCase()}`;
}

/**
 * Every Ctrl shortcut the ritual editor advertises, plus the TipTap bindings
 * it does not list (Mod-e code, Mod-Shift-b blockquote) and the editing keys
 * every text field takes. Upper-cased, so "Ctrl+b" and "Ctrl+B" are one key.
 */
function editorShortcuts(): Set<string> {
  const out = new Set(["CTRL+B", "CTRL+I", "CTRL+U", "CTRL+E", "CTRL+Z", "CTRL+Y", "CTRL+A", "CTRL+C", "CTRL+V", "CTRL+X", "CTRL+SHIFT+B"]);
  const toolbar = read("tam/frontend/src/components/ritual-editor/useRitualToolbar.ts");
  for (const m of toolbar.matchAll(/shortcut: "(Ctrl\+[^"]+)"/g)) out.add(m[1].toUpperCase());
  return out;
}

describe("navigation rail shortcut", () => {
  it.each([
    ["tam/main.go", "tam/frontend/src/components/NavRail.tsx"],
    ["xtm/main.go", "xtm/frontend/src/shell/NavRail.tsx"],
  ])("%s binds a key no editor uses, and %s names it", (goFile, railFile) => {
    const key = railShortcut(read(goFile));
    expect(editorShortcuts().has(key.toUpperCase()), `${key} is an editor shortcut`).toBe(false);
    expect(read(railFile)).toContain(`(View menu, ${key})`);
  });
});
