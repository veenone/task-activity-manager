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
 * Class names a stylesheet in this repo never defines, and should not.
 * ProseMirror and its plugins set their own classes on nodes we render.
 */
const VENDOR_CLASSES = /^(ProseMirror|tiptap)/;
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

  // One synchronous `git grep` per cited name, so this is spawn-bound: it fits
  // in the default 5s when run alone and does not when the whole workspace
  // suite runs in parallel. The timeout is generous on purpose, so a failure
  // here means a name is genuinely missing rather than that the machine was
  // busy.
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
  }, 120_000);

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
    // Walks every Go file in three modules, so it outruns the default 5s when
    // the whole workspace suite competes for the machine. Generous on purpose:
    // a failure here means a real offender, not a busy machine.
  }, 120_000);

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
    //
    // ritual_document is the one board-keyed table deliberately left out of
    // RemoveBoards: it holds page text people wrote and may never have pushed,
    // and a board leaves Jira's list for reasons that say nothing about that
    // text (a setting switched off, a location change, a lost permission).
    // PurgeProfile still sweeps it, so the rows are owned, not orphaned. The
    // reasoning lives on RemoveBoards itself in boardrepo/boards.go. Naming it
    // here keeps the gate failing for every other board-keyed table.
    const REMOVE_BOARDS_EXEMPT = new Set(['ritual_document']);
    const purgeLists =
      read('tam/internal/issuerepo/state.go') + read('tam/internal/boardrepo/boardrepo.go');
    const removeBoards = read('tam/internal/boardrepo/boards.go');
    const missing: string[] = [];
    for (const table of tables) {
      const boardKeyed = new RegExp(`CREATE TABLE IF NOT EXISTS ${table} \\(([\\s\\S]*?)\\);`)
        .exec(schema)?.[1]
        .includes('board_id');
      if (!purgeLists.includes(`"${table}"`)) missing.push(`${table} in no PurgeProfile`);
      if (boardKeyed && !REMOVE_BOARDS_EXEMPT.has(table) && !removeBoards.includes(`"${table}"`)) {
        missing.push(`${table} not in RemoveBoards`);
      }
    }
    expect(missing, missing.join('; ')).toEqual([]);
  });

  // Visually hidden is not the same as out of the way. `.sr-only` positions
  // absolutely, and in an app whose panes are all static that puts the box in
  // the page's own scrollable area, where it escapes every overflow: hidden
  // between it and the window. Its width, height and overflow do not shrink
  // every box either: they do nothing to a table, which sizes to its rows, so
  // TAM's report drew a 300px screen-reader table at the foot of a scrolled
  // view and put a scrollbar on the window. A fixed box contributes to no
  // scrollable overflow at all. Every sheet is checked because the two apps
  // keep their own copies of this rule: xtm/frontend loads no stylesheet from
  // frontend/core, so a fix in the primitive cannot reach it.
  it('no stylesheet hides content by taking it out of flow into the page', () => {
    const offenders: string[] = [];
    for (const dir of SRC_DIRS.concat(['frontend/core/styles'])) {
      for (const file of walk(path.join(repoRoot, dir), /\.css$/)) {
        const rules = fs.readFileSync(file, 'utf8').replace(/\/\*[\s\S]*?\*\//g, ' ').split('}');
        for (const rule of rules) {
          const at = rule.lastIndexOf('{');
          if (at < 0) continue;
          const head = rule.slice(0, at);
          if (!head.split(',').some((part) => part.trim() === '.sr-only')) continue;
          const position = /(?:^|[;{\s])position\s*:\s*([\w-]+)/.exec(rule.slice(at + 1))?.[1];
          if (position !== 'fixed') {
            offenders.push(`${path.relative(repoRoot, file).replace(/\\/g, '/')}: position: ${position ?? 'unset'}`);
          }
        }
      }
    }
    expect(offenders, offenders.join(' | ')).toEqual([]);
  });

  // A class a component sets and no stylesheet defines fails silently: the
  // element renders unstyled and nothing reports it. Two shipped that way
  // here. `.row` left the rituals conflict buttons touching each other, and
  // `.ritual-page-body` left the fallback for an unparseable Confluence page
  // as bare HTML under this app's own margin reset.
  it('every class a component sets is defined in a stylesheet', () => {
    const sheets = SRC_DIRS.concat(['frontend/core/styles'])
      .flatMap((d) => walk(path.join(repoRoot, d), /\.css$/))
      .map((f) => fs.readFileSync(f, 'utf8'))
      .join('\n');
    const defined = new Set([...sheets.matchAll(/\.(-?[_a-zA-Z][\w-]*)/g)].map((m) => m[1]));

    const used = new Map<string, string>();
    // The two trees this gate is clean over. xtm/frontend has its own backlog
    // of orphans and is heading out of this repo; adding it here would mean
    // an allowed number, which belongs in the ratchet, not in this file.
    for (const dir of ['frontend/core/src', 'tam/frontend/src']) {
      for (const file of walk(path.join(repoRoot, dir), SOURCE_EXT)) {
        const rel = path.relative(repoRoot, file).replace(/\\/g, '/');
        if (rel.includes('.test.')) continue;
        const src = stripComments(fs.readFileSync(file, 'utf8'));
        // Plain attributes, and the literal parts of an assembled one. Most
        // rows in this app name their state through a template, so reading
        // only `className="..."` left the majority of the tree unchecked:
        // three classes with no rule sat in templates while this gate passed.
        //
        // Inside a braced expression, a string is a class unless it is the
        // operand of a comparison (`f.id === "description"`), which is the
        // one shape that produced false names.
        for (const m of src.matchAll(/className=(?:"([^"{}]*)"|\{((?:[^{}]|\{[^{}]*\})*)\})/g)) {
          const literal = m[1];
          const expr = m[2];
          const names: string[] = [];
          if (literal !== undefined) names.push(literal);
          if (expr !== undefined) {
            for (const t of expr.matchAll(/`([^`]*)`/g)) {
              // The static text between the ${} holes.
              names.push(t[1].replace(/\$\{[^}]*\}/g, ' '));
              // And the classes a hole appends. One that appends to the list
              // carries its own leading space (`${n ? " nested-subtask" : ""}`),
              // which is what separates it from a fragment completing the
              // name before it (`chip-type-${type || "none"}`).
              for (const hole of t[1].matchAll(/\$\{([^}]*)\}/g)) {
                // Every quoted token in order, so a pattern cannot pair the
                // closing quote of one string with the opening quote of the
                // next and read the gap between them as a name.
                for (const q of hole[1].matchAll(/"([^"]*)"/g)) {
                  if (/^\s/.test(q[1])) names.push(q[1]);
                }
              }
            }
            // A braced ternary names its classes outright. The operand of a
            // comparison is not one of them.
            const branches = expr.replace(/`[^`]*`/g, ' ');
            for (const q of branches.matchAll(/([=!]==?)?\s*"([^"]*)"/g)) {
              if (!q[1]) names.push(q[2]);
            }
          }
          for (const chunk of names) {
            for (const name of chunk.trim().split(/\s+/)) {
              // A trailing hyphen is the head of a name an expression
              // completes (`chart-bar-${f.key}`), so the whole name is not
              // in the source to check.
              if (!name || name.endsWith('-') || used.has(name)) continue;
              used.set(name, rel);
            }
          }
        }
      }
    }
    expect(used.size, 'found no class names to check').toBeGreaterThan(50);

    // Each of these is a second class beside one that carries the styling,
    // or a wrapper the markup names for structure. They are the backlog this
    // gate started with: each one is either given a rule or taken off the
    // element, and the list only shrinks.
    const HOOKS = new Set([
      'confirm-modal', 'richfield', 'detail-section', 'link-groups', 'folder-node',
      'epic-cell-summary', 'epic-cell-status', 'issue-summary', 'sprint-tree',
      'sprint-cell-state',
    ]);
    const orphans = [...used]
      .filter(([name]) => !defined.has(name) && !VENDOR_CLASSES.test(name) && !HOOKS.has(name))
      .map(([name, rel]) => `${name} (${rel})`);
    // A hook that gained a rule leaves the list, so the list cannot rot.
    const stale = [...HOOKS].filter((name) => defined.has(name));
    expect(stale, `these are defined now and can leave HOOKS: ${stale.join(', ')}`).toEqual([]);
    expect(orphans, `classes no stylesheet defines: ${orphans.join(', ')}`).toEqual([]);
  }, 120_000);
});
