// typeColumn sizes the issue-type column from the type names actually on
// screen, the way lib/keyColumn.ts sizes the key column, and for the same
// reason: every row is its own grid container, so a per-row max-content
// track would size each row to its own chip and the columns would stop
// lining up. The width is measured once per render and handed to the table
// as one custom property.
//
// The tracks it replaces were written when a chip only ever said Task, Bug
// or Sub: 104px in the Backlog grid, 44px in the sprint and Epics trees.
// #68 made the sync fetch every type the project has, so the chip now says
// Improvement, Change Request or Sub Test Execution, and in a 44px track
// that is two letters and an ellipsis. All three grids scroll sideways, so
// a column wide enough to read costs nothing that matters.

// The narrowest the column is allowed to get, so a page of Bug and Task
// still leaves the header legible, and the widest, so one pathological name
// cannot eat the row.
const MIN_CHARS = 6;
const MAX_CHARS = 22;

// Average advance per character as a fraction of the font size, for the
// mixed-case words a Jira type name is made of. Lower than keyColumn's 0.62
// because that measures uppercase keys and digits, and a type name is mostly
// lowercase. 0.55em sits above the blend of a realistic name, so the column
// errs wide rather than clipping.
const EM_PER_CHAR = 0.55;

// The chip's own box around the text: 8px padding each side and a 1px
// border each side, from .chip-type in App.css.
const CHIP_BOX_PX = 18;

// typeColumnWidth returns the CSS width for the type column of a grid
// showing these chip labels. Pass what the chip renders, from
// lib/typeChip.ts typeChipLabel, not the logical type id: "subtask" is four
// characters and "Technical task" is fourteen.
export function typeColumnWidth(labels: string[]): string {
  let longest = 0;
  for (const l of labels) {
    if (l.length > longest) longest = l.length;
  }
  const chars = Math.min(Math.max(longest, MIN_CHARS), MAX_CHARS);
  return `calc(${chars} * ${EM_PER_CHAR}em + ${CHIP_BOX_PX}px)`;
}
