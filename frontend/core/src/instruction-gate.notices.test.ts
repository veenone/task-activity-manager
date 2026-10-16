/**
 * Instruction gate: TAM's third-party notices (#289).
 *
 * TAM shows the licence of every Go module and production npm package it
 * bundles, read from tam/frontend/src/assets/third-party.json, which
 * scripts/third-party-notices.mjs writes. Adding or bumping a dependency
 * without rerunning the script would ship a binary whose notices are wrong, so
 * this compares the file against tam/go.mod and the installed npm tree.
 */
import { describe, expect, it } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';

const repoRoot = path.resolve(__dirname, '../../..');
const NOTICES = 'tam/frontend/src/assets/third-party.json';
/** Workspaces whose production dependencies end up in the TAM binary. */
const NPM_ROOTS = ['tam/frontend', 'frontend/core'];
/** Our own modules and packages; TAM's licence covers them. */
const OWN = new Set(['agile-suite/core', '@agile-suite/core']);

interface Notice { name: string; version: string; kind: 'go' | 'npm' }

function read(rel: string): string {
  return fs.readFileSync(path.join(repoRoot, rel), 'utf8');
}

function goRequires(): string[] {
  const mod = read('tam/go.mod');
  const out: string[] = [];
  for (const block of mod.matchAll(/^require \(([\s\S]*?)^\)/gm)) {
    for (const line of block[1].split('\n')) {
      const m = /^\s*(\S+)\s+(v\S+)/.exec(line);
      if (m && !OWN.has(m[1])) out.push(`${m[1]}@${m[2]}`);
    }
  }
  for (const m of mod.matchAll(/^require (\S+) (v\S+)/gm)) {
    if (!OWN.has(m[1])) out.push(`${m[1]}@${m[2]}`);
  }
  return out;
}

/** Finds the installed copy of `name` the way Node would from `fromDir`. */
function resolvePackage(name: string, fromDir: string): string | null {
  let dir = fromDir;
  for (;;) {
    const candidate = path.join(dir, 'node_modules', name, 'package.json');
    if (fs.existsSync(candidate)) return path.dirname(candidate);
    if (path.resolve(dir) === repoRoot) return null;
    dir = path.dirname(dir);
  }
}

function npmProduction(): string[] {
  const seen = new Set<string>();
  const queue = NPM_ROOTS.map((rel) => path.join(repoRoot, rel));
  const visitedDirs = new Set<string>();
  while (queue.length) {
    const dir = queue.shift()!;
    if (visitedDirs.has(dir)) continue;
    visitedDirs.add(dir);
    const pkg = JSON.parse(fs.readFileSync(path.join(dir, 'package.json'), 'utf8'));
    const isRoot = NPM_ROOTS.some((rel) => path.join(repoRoot, rel) === dir);
    if (!isRoot && !OWN.has(pkg.name)) seen.add(`${pkg.name}@${pkg.version}`);
    const deps = {
      ...pkg.dependencies, ...pkg.optionalDependencies, ...(isRoot ? {} : pkg.peerDependencies),
    };
    for (const name of Object.keys(deps)) {
      if (OWN.has(name)) continue;
      const found = resolvePackage(name, dir);
      if (found) queue.push(found);
    }
  }
  return [...seen];
}

describe('instruction gate: third-party notices', () => {
  const notices: Notice[] = JSON.parse(read(NOTICES)).packages;
  const listed = (kind: Notice['kind']) =>
    new Set(notices.filter((n) => n.kind === kind).map((n) => `${n.name}@${n.version}`));

  it('lists every module tam/go.mod requires', () => {
    const want = goRequires();
    expect(want.length, 'read no requires from tam/go.mod').toBeGreaterThan(10);
    const have = listed('go');
    const missing = want.filter((m) => !have.has(m));
    expect(missing, `rerun node scripts/third-party-notices.mjs: ${missing.join(', ')}`).toEqual([]);
  });

  it('declares the licence the notices file and the root LICENSE carry', () => {
    expect(read('LICENSE')).toMatch(/GNU GENERAL PUBLIC LICENSE\s+Version 3/);
    expect(JSON.parse(read(NOTICES)).license).toBe('GPL-3.0-only');
    expect(JSON.parse(read('tam/frontend/package.json')).license).toBe('GPL-3.0-only');
    // wails.json's info block has no licence key, so the copyright line says it.
    expect(JSON.parse(read('tam/wails.json')).info.copyright).toContain('GPL-3.0-only');
  });

  it('lists every production npm package of tam/frontend and frontend/core', () => {
    const want = npmProduction();
    expect(want.length, 'walked no npm dependencies').toBeGreaterThan(10);
    const have = listed('npm');
    const missing = want.filter((p) => !have.has(p));
    expect(missing, `rerun node scripts/third-party-notices.mjs: ${missing.join(', ')}`).toEqual([]);
  });
});
