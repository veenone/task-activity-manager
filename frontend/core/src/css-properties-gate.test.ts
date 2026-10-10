/**
 * Custom property gate (#185). A rule that reads `var(--x)` when no sheet the
 * app loads declares `--x` gets the fallback, or nothing: a light-only grey in
 * dark mode, or no background at all. The check is per app because the apps
 * load different sheets: TAM imports core's tokens.css and primitives.css,
 * XTM imports only its own until its chrome moves onto the shared shell.
 *
 * A property a component sets from TSX (`style={{ "--board-cols": n }}`)
 * counts as declared.
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
    sheets: ['frontend/core/src', 'xtm/frontend/src'],
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

describe('custom property gate', () => {
  for (const app of APPS) {
    it(`every var() in ${app.name}'s stylesheets has a declaration ${app.name} loads`, () => {
      const css = app.sheets.flatMap((d) => files(d, /\.css$/)).map((f) => [f, source(f)] as const);
      const declared = new Set<string>();
      for (const [, src] of css) {
        for (const m of src.matchAll(/(--[\w-]+)\s*:/g)) declared.add(m[1]);
      }
      for (const f of app.tsx.flatMap((d) => files(d, /\.tsx?$/))) {
        for (const m of source(f).matchAll(/["'](--[\w-]+)["']\s*:/g)) declared.add(m[1]);
      }
      const missing = new Set<string>();
      for (const [f, src] of css) {
        for (const m of src.matchAll(/var\(\s*(--[\w-]+)/g)) {
          if (!declared.has(m[1])) missing.add(`${m[1]} in ${f}`);
        }
      }
      expect([...missing].sort()).toEqual([]);
    });
  }
});
