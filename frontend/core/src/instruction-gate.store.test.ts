/**
 * Instruction gate: the Store schema and purge contract.
 *
 * Split out of instruction-gate.test.ts, which had reached the 400-line limit
 * C2 sets. These two cases share only `read`, so they were the cut that cost
 * the least.
 *
 * Both apps are checked. A table the schema keys by profile and no purge list
 * names survives a profile delete, and nothing else reports it: XTM carried 25
 * such tables until #272, because this gate read TAM alone.
 */
import { describe, expect, it } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';

const repoRoot = path.resolve(__dirname, '../../..');

function read(rel: string): string {
  return fs.readFileSync(path.join(repoRoot, rel), 'utf8');
}

describe('instruction gate: store schema and purge', () => {

  it('every profile-keyed table is swept by one of the two purge lists', () => {
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

  // XTM keeps one purge list rather than TAM's two, and has no board to
  // scope rows to. The check is the same one: a table the schema keys by
  // profile and the list does not name survives a profile delete. This went
  // unnoticed for 25 of 40 tables because the gate above reads TAM's schema
  // only, while the contract it enforces reads as though it covered both.
  it('every profile-keyed XTM table is in its purge list', () => {
    const schema = read('xtm/internal/store/store.go');
    const tables = [...schema.matchAll(/CREATE TABLE IF NOT EXISTS (\w+) \(([\s\S]*?)\);/g)]
      .filter(([, , body]) => /profile_id/.test(body))
      .map(([, name]) => name);
    expect(tables.length, 'found no profile-keyed XTM tables to check').toBeGreaterThan(0);

    const purgeList = read('xtm/internal/testrepo/purge.go');
    const missing = tables.filter((table) => !purgeList.includes(`"${table}"`));
    expect(missing, `${missing.join(', ')} in no PurgeProfile`).toEqual([]);
  });
});
