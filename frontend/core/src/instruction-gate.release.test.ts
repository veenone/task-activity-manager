/**
 * Instruction gate: what a release ships and how it is labelled.
 *
 * Split out of instruction-gate.test.ts for the same reason the store gate
 * was: that file sits at the 400-line limit C2 sets. These two cases share
 * only `read` and the list of buildable apps.
 *
 * These checks exist because what they check failed silently. A version
 * resource a reader cannot read still builds, so the first report comes from
 * whoever opened the properties dialog months later.
 */
import { describe, expect, it } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';

const repoRoot = path.resolve(__dirname, '../../..');
/** Go module roots; the ones with a wails.json are the buildable apps. */
const GO_DIRS = ['core', 'tam', 'xtm'];

function read(rel: string): string {
  return fs.readFileSync(path.join(repoRoot, rel), 'utf8');
}

function apps(): string[] {
  const found = GO_DIRS.filter((d) => fs.existsSync(path.join(repoRoot, d, 'wails.json')));
  expect(found.length, 'found no app modules with a wails.json').toBeGreaterThan(0);
  return found;
}

describe('instruction gate: release packaging', () => {
  // build/windows/info.json is the whole version resource: a language to
  // file the strings under, and the strings. Both apps shipped it without a
  // FileVersion string and under 0000, the neutral language, and the
  // properties dialog came up empty: no product name, no company, no
  // copyright, no version. The resource itself was well formed, and
  // VerQueryValue read every string straight out of it. What reads it in
  // practice does not stop there. Explorer and .NET take a missing
  // FileVersion for a failed lookup, then retry under 040904B0, 040904E4 and
  // 04090000 in turn, overwriting what they already found, so the last miss
  // is what the dialog shows. One absent string empties the other six.
  //
  // Verified on a build of each app: with FileVersion present and the strings
  // under 0409, FileVersionInfo answers the product name, company, copyright,
  // description and comments, and reports the language as English (United
  // States) rather than Language Neutral.
  it('every app writes a version resource its readers can read', () => {
    const broken = apps().filter((app) => {
      const langs = JSON.parse(read(`${app}/build/windows/info.json`)).info ?? {};
      const neutral = Object.keys(langs).some((lang) => Number.parseInt(lang, 16) === 0);
      const noFileVersion = Object.values(langs).some(
        (table) => !(table as Record<string, string>).FileVersion,
      );
      return neutral || noFileVersion;
    });
    expect(
      broken,
      `version strings filed under the neutral language, or with no FileVersion to find them by: ${broken.join(', ')}`,
    ).toEqual([]);
  });
});
