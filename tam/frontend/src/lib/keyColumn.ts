// keyColumn sizes the issue-key column from the keys actually on screen.
//
// Both tables used to give the key a fixed pixel track (84px in the Backlog,
// 92px in the Epics tree) sized for a short key like PLAT-412. Jira DC allows
// a project key of ten characters, so a legal key such as PLATFORM-98765 does
// not fit: in the Backlog it wrapped inside a 34px row and the type chip
// painted over it, and in the tree, where white-space is nowrap, it ran
// straight over the summary. Neither cell had an ellipsis, so a clipped key
// was indistinguishable from a short one, which in a tool whose next action
// edits the issue under that key is a correctness problem, not a cosmetic one.
//
// A per-row max-content track does not fix it: every row is its own grid
// container, so each row would size its key column to its own key and the
// columns would stop lining up. The width has to be one number shared by
// every row, so it is measured here once per render and handed to the table
// as a CSS custom property.

// The narrowest the column is allowed to get, so a page of short keys still
// leaves the header legible, and the widest, so one pathological key cannot
// eat the summary.
const MIN_CHARS = 9;
const MAX_CHARS = 22;

// Average advance per character as a fraction of the font size, for the
// uppercase letters, digits, hyphens, and underscores a Jira key is made of.
// Measured in Chromium at 12px Segoe UI: 7.59px per uppercase letter at
// weight 600, 6.49px per digit, 4.83px per hyphen. 0.62em sits above the
// blend of a realistic key, so the column errs wide rather than clipping.
const EM_PER_CHAR = 0.62;

// keyColumnWidth returns the CSS width for the key column of a table showing
// these keys. extraPx is room for anything sharing the cell, e.g. the
// Backlog's pending dot.
export function keyColumnWidth(keys: string[], extraPx = 0): string {
  let longest = 0;
  for (const k of keys) {
    if (k.length > longest) longest = k.length;
  }
  const chars = Math.min(Math.max(longest, MIN_CHARS), MAX_CHARS);
  return `calc(${chars} * ${EM_PER_CHAR}em + ${extraPx}px)`;
}
