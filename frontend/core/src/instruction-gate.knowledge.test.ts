/**
 * Instruction gate: the knowledge files.
 *
 * Split out of instruction-gate.test.ts, which was at the 400-line limit C2
 * sets. These two cases are the ones that read agents/ rather than the
 * instruction files or the source tree, so they were the cut that cost the
 * least: they share KNOWLEDGE_FILES and `read` with nothing else in there.
 *
 * What a playbook claims has to stay checkable. A citation naming a commit
 * nobody can resolve is a claim with no evidence behind it, and six of them
 * were in that state (#355).
 */
import { describe, expect, it } from 'vitest';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

const repoRoot = path.resolve(__dirname, '../../..');
const KNOWLEDGE_FILES = [
  'agents/project/domain-context.md',
  'agents/project/glossary.md',
  'agents/project/out-of-scope.md',
  'agents/project/testing.md',
  'agents/generic/claude-workflows.md',
];

function read(rel: string): string {
  return fs.readFileSync(path.join(repoRoot, rel), 'utf8');
}

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

/**
 * The ref the hash check measures citations against.
 *
 * It has to be the default branch rather than this clone. `git cat-file -e`
 * passes for any object the clone happens to hold, which is every commit of
 * every branch it has fetched and, locally, everything still in the reflog
 * after a rebase. This repo squash-merges, so a commit written on a branch is
 * never on main: the old check was quiet while the branch survived, and would
 * have started failing on main the day somebody deleted it (#355).
 *
 * Throwing when no ref resolves is deliberate. A gate that skips its own
 * check when it cannot find the thing to check against reports a pass for
 * work it never did.
 */
function defaultBranchRef(): string {
  for (const ref of ['origin/main', 'main']) {
    if (gitOk(['rev-parse', '--verify', '--quiet', `${ref}^{commit}`])) return ref;
  }
  throw new Error('no origin/main or main ref to measure cited commits against');
}

describe('instruction gate: knowledge files', () => {
  it('every commit hash cited in a knowledge file is on the default branch', () => {
    const hashes = new Set<string>();
    for (const file of KNOWLEDGE_FILES) {
      if (!fs.existsSync(path.join(repoRoot, file))) continue;
      for (const m of read(file).matchAll(/\(([0-9a-f]{7,40})\)/g)) hashes.add(m[1]);
    }
    const base = defaultBranchRef();
    // The commits on main in one read, rather than a git call per hash. Two
    // spawns per citation ran past vitest's 5s default once the whole
    // workspace suite was competing for the machine, and this asks the same
    // question: a citation is an abbreviated sha, so it is on main when it
    // prefixes one of these.
    const onBase = git(['rev-list', base]).split('\n').filter(Boolean);
    // M2: assert the gate read something, or an unreadable ref would fail
    // every citation and read as thirteen bad hashes.
    expect(onBase.length, `read no commits out of ${base}`).toBeGreaterThan(0);
    const offenders = [...hashes]
      .filter((h) => !onBase.some((full) => full.startsWith(h)))
      .map((h) => (gitOk(['cat-file', '-e', `${h}^{commit}`]) ? `${h} (only on a branch)` : `${h} (no such commit)`));
    expect(
      offenders,
      `knowledge files cite commits that ${base} does not have: ${offenders.join(', ')}. ` +
        'Cite the squash commit that landed the work, or the issue number for work being landed.',
    ).toEqual([]);
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

});
