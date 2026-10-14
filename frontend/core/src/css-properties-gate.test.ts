/**
 * Custom property gate (#185). A rule that reads `var(--x)` when no sheet the
 * app loads declares `--x` gets the fallback, or nothing: a light-only grey in
 * dark mode, or no background at all. The check is per app because the apps
 * load different sheets. Both import core's tokens.css and primitives.css
 * (#212); each adds its own app stylesheet.
 *
 * A property a component sets from TSX (`style={{ "--board-cols": n }}`)
 * counts as declared. In core's shared stylesheets a property set by any
 * app's TSX counts: core styles TAM's board for TAM, and XTM never renders the
 * component that reads it.
 */
import { describe, expect, it } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';

const repoRoot = path.resolve(__dirname, '../../..');

/** The stylesheets each app loads, and the source trees that can set a property. */
const APPS: { name: string; sheets: string[]; tsx: string[] }[] = [
  {
    name: 'tam',
    sheets: ['frontend/core/styles', 'frontend/core/src', 'tam/frontend/src'],
    tsx: ['frontend/core/src', 'tam/frontend/src'],
  },
  {
    name: 'xtm',
    sheets: ['frontend/core/styles', 'frontend/core/src', 'xtm/frontend/src'],
    tsx: ['frontend/core/src', 'xtm/frontend/src'],
  },
];

function files(dir: string, ext: RegExp, acc: string[] = []): string[] {
  for (const entry of fs.readdirSync(path.join(repoRoot, dir), { withFileTypes: true })) {
    const rel = path.posix.join(dir, entry.name);
    if (entry.isDirectory()) files(rel, ext, acc);
    else if (ext.test(entry.name) && !entry.name.includes('.test.')) acc.push(rel);
  }
  return acc;
}

function source(rel: string): string {
  return fs.readFileSync(path.join(repoRoot, rel), 'utf8').replace(/\/\*[\s\S]*?\*\//g, ' ');
}

/** Properties a component sets from TSX in the given trees. */
function tsxProperties(trees: string[]): Set<string> {
  const out = new Set<string>();
  for (const f of trees.flatMap((d) => files(d, /\.tsx?$/))) {
    for (const m of source(f).matchAll(/["'](--[\w-]+)["']\s*:/g)) out.add(m[1]);
  }
  return out;
}

const SHARED_SHEETS = 'frontend/core/styles/';

describe('custom property gate', () => {
  const anyAppTsx = tsxProperties([...new Set(APPS.flatMap((a) => a.tsx))]);
  for (const app of APPS) {
    it(`every var() in ${app.name}'s stylesheets has a declaration ${app.name} loads`, () => {
      const css = app.sheets.flatMap((d) => files(d, /\.css$/)).map((f) => [f, source(f)] as const);
      const declared = new Set<string>();
      for (const [, src] of css) {
        for (const m of src.matchAll(/(--[\w-]+)\s*:/g)) declared.add(m[1]);
      }
      for (const p of tsxProperties(app.tsx)) declared.add(p);
      const missing = new Set<string>();
      for (const [f, src] of css) {
        for (const m of src.matchAll(/var\(\s*(--[\w-]+)/g)) {
          const shared = f.startsWith(SHARED_SHEETS) && anyAppTsx.has(m[1]);
          if (!declared.has(m[1]) && !shared) missing.add(`${m[1]} in ${f}`);
        }
      }
      expect([...missing].sort()).toEqual([]);
    });
  }
});

describe('shared stylesheets (#212)', () => {
  // Both apps take their tokens and primitives from core, ahead of their own
  // sheet so an app rule can still override a primitive. A copy kept in step
  // by hand drifted before; there is now only the one.
  for (const app of ['tam', 'xtm']) {
    it(`${app} loads core's tokens and primitives before its own stylesheet`, () => {
      const imports = [...source(`${app}/frontend/src/main.tsx`).matchAll(/^import\s+["']([^"']+\.css)["']/gm)].map(
        (m) => m[1],
      );
      expect(imports.slice(0, 2)).toEqual([
        '@agile-suite/core/styles/tokens.css',
        '@agile-suite/core/styles/primitives.css',
      ]);
      expect(imports.length).toBeGreaterThan(2);
    });
  }

  // `.btn:hover:not(:disabled):not([aria-disabled="true"])` is four classes
  // deep. A variant's hover with fewer loses to it whatever the source order,
  // and a danger button hovered to plain grey.
  it('every .btn variant hover is at least as specific as the base .btn hover', () => {
    const css = source('frontend/core/styles/primitives.css');
    const weak = [...css.matchAll(/(^|[\s,}])(\.btn-[\w-]+:hover[^,{]*)/g)]
      .map((m) => m[2].trim())
      .filter((sel) => !sel.includes(':not([aria-disabled="true"])'));
    expect(weak).toEqual([]);
  });
});
