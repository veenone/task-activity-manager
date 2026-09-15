/**
 * Instruction gate. Adapted from gap-trap's vitest reference.
 *
 * The instruction files are read by every session, so they are checked like
 * code: every name a contract cites exists, the portable core stays portable,
 * the always-loaded files stay small, cited commits exist, and no private data
 * leaks into knowledge files. The grep gates at the bottom are the contract
 * Never clauses a text search settles and that are clean today; clauses with a
 * backlog live in the ratchet instead, so this file never carries an allowed
 * number.
 *
 * It lives in frontend/core so `npm test --workspaces` runs it, which CI
 * already does. It checks repo-root files, not this package.
 */
import { describe, expect, it } from 'vitest';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

// ---- config ---------------------------------------------------------------
const repoRoot = path.resolve(__dirname, '../../..');
/** Every workspace source tree contracts talk about. */
const SRC_DIRS = ['frontend/core/src', 'tam/frontend/src', 'xtm/frontend/src'];
/** Go module roots, for the contracts that talk about Go. */
const GO_DIRS = ['core', 'tam', 'xtm'];
const SOURCE_EXT = /\.(ts|tsx)$/;
const GO_EXT = /\.go$/;
/** Project-specific tokens that must never appear in the portable core. */
const FORBIDDEN_IN_CORE = [
  'agile-suite', 'Jira', 'Confluence', 'Wails', 'TAM', 'XTM',
  'tam/', 'xtm/', 'agents/project',
];
/** Current always-loaded count is 1,596. Raising this needs a reason in the commit message (C7). */
const WORD_BUDGET = 2500;
const ALWAYS_LOADED = ['AGENTS.md', 'AGENTS.project.md', 'CLAUDE.md'];
const MIN_CONTRACTS = 6;
const KNOWLEDGE_FILES = [
  'agents/project/domain-context.md',
  'agents/project/glossary.md',
  'agents/project/out-of-scope.md',
  'agents/project/testing.md',
  'agents/generic/claude-workflows.md',
];
/**
 * One entry per contract Never clause a grep settles AND that is clean today.
 * A clause with a backlog belongs in the ratchet, not here.
 */
const GREP_GATES: { name: string; dirs: string[]; ext: RegExp; pattern: RegExp; exempt?: (rel: string) => boolean }[] = [
  {
    name: 'Module privacy: no cross-module internal imports',
    dirs: GO_DIRS,
    ext: GO_EXT,
    pattern: /"agile-suite\/(tam|xtm)\/internal/,
    // A module may import its own internal tree.
    exempt: (rel) => rel.startsWith('tam/') || rel.startsWith('xtm/'),
  },
  {
    // The Credentials contract named this gate before it existed (M1).
    // Clean today, so it is zero-tolerance rather than a ratchet.
    name: 'Credentials: no token reaching a log or SQL call',
    dirs: GO_DIRS,
    ext: GO_EXT,
    pattern: /(log\.(?:Print|Fatal)\w*|ExecContext|QueryContext|\bExec|\bQuery)\s*\([^)]*\b(?:token|Token|PAT|secret|Secret)\b/,
    exempt: (rel) => rel.endsWith('_test.go'),
  },
];
// ---- end config -----------------------------------------------------------

function walk(dir: string, ext: RegExp, acc: string[] = []): string[] {
  if (!fs.existsSync(dir)) return acc;
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (entry.name === 'node_modules' || entry.name === 'wailsjs') continue;
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) walk(full, ext, acc);
    else if (ext.test(entry.name)) acc.push(full);
  }
  return acc;
}

/** Run git with an argument array: no shell, so no token can inject. */
function git(args: string[]): string {
  try {
    return execFileSync('git', args, { cwd: repoRoot, encoding: 'utf8' }).trim();
  } catch {
    return '';
  }
}

function gitOk(args: string[]): boolean {
  try {
    execFileSync('git', args, { cwd: repoRoot, stdio: 'ignore' });
    return true;
  } catch {
    return false;
  }
}

function read(rel: string): string {
  return fs.readFileSync(path.join(repoRoot, rel), 'utf8');
}

/** Strip // and /* *\/ comments so a mention in prose is not a violation. */
function stripComments(src: string): string {
  return src.replace(/\/\*[\s\S]*?\*\//g, '').replace(/^\s*\/\/.*$/gm, '');
}

const projectFile = read('AGENTS.project.md');
const contractBlocks = projectFile
  .split(/^### /m)
  .slice(1)
  .filter((b) => /^Owns:/m.test(b));

describe('instruction gate', () => {
  it('AGENTS.project.md declares its contracts in full', () => {
    expect(contractBlocks.length).toBeGreaterThanOrEqual(MIN_CONTRACTS);
    for (const block of contractBlocks) {
      const name = block.split('\n')[0].trim();
      for (const field of ['Owns:', 'Path:', 'Never:', 'Gate:']) {
        expect(block, `contract "${name}" is missing ${field}`).toMatch(
          new RegExp(`^${field}`, 'm'),
        );
      }
    }
  });

  it('every name a contract cites exists in the tree', () => {
    const tokens = new Set<string>();
    for (const line of projectFile.split('\n')) {
      if (!/^(Path|Gate):/.test(line)) continue;
      for (const m of line.matchAll(/`([^`]+)`/g)) tokens.add(m[1]);
    }
    expect(tokens.size).toBeGreaterThan(0);

    const missing: string[] = [];
    for (const token of tokens) {
      if (token.includes('/')) {
        // A path, or a bare package segment such as `internal/`.
        if (fs.existsSync(path.join(repoRoot, token))) continue;
        const bare = token.replace(/\/$/, '');
        if (!git(['grep', '-l', '-F', '--', bare])) missing.push(token);
      } else {
        if (!git(['grep', '-l', '-w', '--', token])) missing.push(token);
      }
    }
    expect(missing, `contracts name things that do not exist: ${missing.join(', ')}`).toEqual([]);
  });

  it('the portable core stays portable', () => {
    const core = read('AGENTS.md');
    const leaked = FORBIDDEN_IN_CORE.filter((t) => core.includes(t));
    expect(leaked, `AGENTS.md leaked project names: ${leaked.join(', ')}`).toEqual([]);
  });

  it('the always-loaded files stay under the word budget', () => {
    const words = ALWAYS_LOADED.reduce(
      (n, f) => n + read(f).split(/\s+/).filter(Boolean).length,
      0,
    );
    expect(
      words,
      `always-loaded instruction files are ${words} words, budget ${WORD_BUDGET}`,
    ).toBeLessThanOrEqual(WORD_BUDGET);
  });

  it('every commit hash cited in a knowledge file exists', () => {
    const hashes = new Set<string>();
    for (const file of KNOWLEDGE_FILES) {
      if (!fs.existsSync(path.join(repoRoot, file))) continue;
      for (const m of read(file).matchAll(/\(([0-9a-f]{7,40})\)/g)) hashes.add(m[1]);
    }
    const missing = [...hashes].filter((h) => !gitOk(['cat-file', '-e', `${h}^{commit}`]));
    expect(missing, `knowledge files cite commits that do not exist: ${missing.join(', ')}`).toEqual([]);
  });

  it('knowledge files carry no email or IP address', () => {
    const offenders: string[] = [];
    for (const file of KNOWLEDGE_FILES) {
      if (!fs.existsSync(path.join(repoRoot, file))) continue;
      const text = read(file);
      if (/[\w.+-]+@[\w-]+\.[\w.]+/.test(text)) offenders.push(`${file}: email`);
      if (/\b\d{1,3}(\.\d{1,3}){3}\b/.test(text)) offenders.push(`${file}: IP address`);
    }
    expect(offenders, offenders.join('; ')).toEqual([]);
  });

  it.each(GREP_GATES)('$name', ({ dirs, ext, pattern, exempt }) => {
    let scanned = 0;
    const offenders: string[] = [];
    for (const dir of dirs) {
      for (const file of walk(path.join(repoRoot, dir), ext)) {
        const rel = path.relative(repoRoot, file).replace(/\\/g, '/');
        scanned += 1;
        if (exempt?.(rel)) continue;
        if (pattern.test(stripComments(fs.readFileSync(file, 'utf8')))) offenders.push(rel);
      }
    }
    // M2: assert the gate actually read something.
    expect(scanned, 'grep gate scanned no files').toBeGreaterThan(0);
    expect(offenders, offenders.join(', ')).toEqual([]);
  });

  it('every profile-keyed table is swept by both purge lists', () => {
    const schema = read('tam/internal/tamstore/tamstore.go');
    const tables = [...schema.matchAll(/CREATE TABLE IF NOT EXISTS (\w+) \(([\s\S]*?)\);/g)]
      .filter(([, , body]) => /profile_id/.test(body))
      .map(([, name]) => name);
    expect(tables.length, 'found no profile-keyed tables to check').toBeGreaterThan(0);

    // TAM purges a profile through two repositories that partition the tables:
    // issuerepo sweeps issue data, boardrepo sweeps board data. A table must
    // appear in one of them, and a board-keyed table must also be in
    // RemoveBoards or removing a board orphans its rows.
    const purgeLists =
      read('tam/internal/issuerepo/state.go') + read('tam/internal/boardrepo/boardrepo.go');
    const removeBoards = read('tam/internal/boardrepo/boards.go');
    const missing: string[] = [];
    for (const table of tables) {
      const boardKeyed = new RegExp(`CREATE TABLE IF NOT EXISTS ${table} \\(([\\s\\S]*?)\\);`)
        .exec(schema)?.[1]
        .includes('board_id');
      if (!purgeLists.includes(`"${table}"`)) missing.push(`${table} in no PurgeProfile`);
      if (boardKeyed && !removeBoards.includes(`"${table}"`)) missing.push(`${table} not in RemoveBoards`);
    }
    expect(missing, missing.join('; ')).toEqual([]);
  });
});
