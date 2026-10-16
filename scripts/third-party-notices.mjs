#!/usr/bin/env node
// Writes tam/frontend/src/assets/third-party.json, the notices TAM's Help →
// About → Licences dialog shows (#289). Plain Node, no dependencies.
//
//   node scripts/third-party-notices.mjs
//
// Go entries are the modules tam/go.mod requires, with the LICENSE or COPYING
// file from each module's directory in the module cache. npm entries are the
// production dependency tree of tam/frontend and frontend/core, with each
// package's `license` field and its LICENSE file. Identical licence texts are
// stored once, keyed by a short hash, to keep the bundle small.
//
// frontend/core/src/instruction-gate.notices.test.ts fails when a dependency
// is missing from the file, so rerun this after changing go.mod or a
// package.json and commit the result.
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const OUT = path.join(repoRoot, 'tam/frontend/src/assets/third-party.json');
const NPM_ROOTS = ['tam/frontend', 'frontend/core'];
const OWN = new Set(['agile-suite/core', '@agile-suite/core']);
const LICENCE_FILE = /^(licen[cs]e|copying)(\.(md|txt|markdown))?$/i;

const texts = {};

function storeText(text) {
  if (!text) return '';
  const normalised = text.replace(/\r\n/g, '\n').trim();
  const key = createHash('sha256').update(normalised).digest('hex').slice(0, 12);
  texts[key] = normalised;
  return key;
}

function licenceText(dir) {
  if (!dir || !fs.existsSync(dir)) return '';
  const file = fs.readdirSync(dir).filter((f) => LICENCE_FILE.test(f)).sort()[0];
  return file ? fs.readFileSync(path.join(dir, file), 'utf8') : '';
}

// Go modules carry no licence field, so the identifier is read off the text.
// Anything not recognised says so rather than guessing.
function guessSpdx(text) {
  const t = text.replace(/\s+/g, ' ');
  if (/Apache License,? Version 2\.0/i.test(t)) return 'Apache-2.0';
  if (/Mozilla Public License,? (version|v\.?) ?2\.0/i.test(t)) return 'MPL-2.0';
  if (/GNU GENERAL PUBLIC LICENSE Version 3/i.test(t)) return 'GPL-3.0';
  if (/Permission is hereby granted, free of charge/i.test(t)) return 'MIT';
  if (/Permission to use, copy, modify, and\/or distribute/i.test(t)) return 'ISC';
  if (/Redistribution and use in source and binary forms/i.test(t)) {
    return /endorse or promote|names of its contributors/i.test(t) ? 'BSD-3-Clause' : 'BSD-2-Clause';
  }
  if (/This is free and unencumbered software released into the public domain/i.test(t)) return 'Unlicense';
  return 'see text';
}

function goRequires() {
  const mod = fs.readFileSync(path.join(repoRoot, 'tam/go.mod'), 'utf8');
  const out = new Map();
  for (const block of mod.matchAll(/^require \(([\s\S]*?)^\)/gm)) {
    for (const line of block[1].split('\n')) {
      const m = /^\s*(\S+)\s+(v\S+)/.exec(line);
      if (m && !OWN.has(m[1])) out.set(m[1], m[2]);
    }
  }
  for (const m of mod.matchAll(/^require (\S+) (v\S+)/gm)) {
    if (!OWN.has(m[1])) out.set(m[1], m[2]);
  }
  return out;
}

function goModules() {
  const required = goRequires();
  const go = (args) =>
    execFileSync('go', args, {
      cwd: path.join(repoRoot, 'tam'),
      env: { ...process.env, GOWORK: 'off' },
      encoding: 'utf8',
      maxBuffer: 64 * 1024 * 1024,
    });
  const specs = [...required].map(([name, version]) => `${name}@${version}`);
  // -json prints one object per module, back to back; split on the boundary.
  const raw = go(['mod', 'download', '-json', ...specs]);
  const infos = JSON.parse(`[${raw.replace(/}\s*{/g, '},{')}]`);
  return infos.map((info) => {
    if (info.Error) throw new Error(`go mod download ${info.Path}: ${info.Error}`);
    const text = licenceText(info.Dir);
    return { name: info.Path, version: info.Version, license: text ? guessSpdx(text) : 'not found', kind: 'go', text: storeText(text) };
  });
}

function resolvePackage(name, fromDir) {
  let dir = fromDir;
  for (;;) {
    const candidate = path.join(dir, 'node_modules', name, 'package.json');
    if (fs.existsSync(candidate)) return path.dirname(candidate);
    if (path.resolve(dir) === repoRoot) return null;
    dir = path.dirname(dir);
  }
}

function spdxOf(pkg) {
  if (typeof pkg.license === 'string') return pkg.license;
  if (pkg.license?.type) return pkg.license.type;
  if (Array.isArray(pkg.licenses)) return pkg.licenses.map((l) => l.type).join(' OR ');
  return 'see text';
}

function npmPackages() {
  const rootDirs = NPM_ROOTS.map((rel) => path.join(repoRoot, rel));
  const queue = [...rootDirs];
  const visited = new Set();
  const out = new Map();
  while (queue.length) {
    const dir = queue.shift();
    if (visited.has(dir)) continue;
    visited.add(dir);
    const pkg = JSON.parse(fs.readFileSync(path.join(dir, 'package.json'), 'utf8'));
    const isRoot = rootDirs.includes(dir);
    if (!isRoot && !OWN.has(pkg.name)) {
      out.set(`${pkg.name}@${pkg.version}`, {
        name: pkg.name, version: pkg.version, license: spdxOf(pkg), kind: 'npm', text: storeText(licenceText(dir)),
      });
    }
    const deps = { ...pkg.dependencies, ...pkg.optionalDependencies, ...(isRoot ? {} : pkg.peerDependencies) };
    for (const name of Object.keys(deps)) {
      if (OWN.has(name)) continue;
      const found = resolvePackage(name, dir);
      if (found) queue.push(found);
    }
  }
  return [...out.values()];
}

const own = storeText(fs.readFileSync(path.join(repoRoot, 'LICENSE'), 'utf8'));
const packages = [...goModules(), ...npmPackages()].sort(
  (a, b) => a.kind.localeCompare(b.kind) || a.name.localeCompare(b.name),
);
const used = new Set([own, ...packages.map((p) => p.text)]);
const keptTexts = Object.fromEntries(Object.entries(texts).filter(([k]) => used.has(k)).sort());
const notices = { license: 'GPL-3.0-only', text: own, packages, texts: keptTexts };
fs.writeFileSync(OUT, `${JSON.stringify(notices, null, 1)}\n`);
process.stdout.write(`wrote ${path.relative(repoRoot, OUT)}: ${packages.length} packages, ${Object.keys(keptTexts).length} licence texts\n`);
