// a11y-markup counts the markup a screen reader cannot make sense of (#227):
// a <table> with no accessible name, a <th> with no scope, and an icon-only
// button (✕, ×) with no aria-label, whose name would be the glyph. It reads
// file paths from stdin, one per line, and prints one number. With --list it
// prints each finding instead.
const fs = require("fs");

const files = fs.readFileSync(0, "utf8").split(/\r?\n/).filter((f) => f && !f.includes(".test."));
const list = process.argv.includes("--list");
const findings = [];

for (const file of files) {
  const src = fs.readFileSync(file, "utf8");
  const line = (i) => src.slice(0, i).split("\n").length;
  for (const m of src.matchAll(/<table\b([^>]*)>/g)) {
    // A <caption> right inside the table names it as well as an aria-label.
    const rest = src.slice(m.index + m[0].length);
    const named = /aria-label(ledby)?=/.test(m[1]) || /^\s*<caption\b/.test(rest);
    if (!named) findings.push(`${file}:${line(m.index)} table without a name`);
  }
  for (const m of src.matchAll(/<th\b([^>]*)>/g)) {
    if (!/\bscope=/.test(m[1])) findings.push(`${file}:${line(m.index)} th without scope`);
  }
  for (const m of src.matchAll(/<button\b([^>]*)>\s*(✕|×|✖)\s*<\/button>/g)) {
    if (!/aria-label=/.test(m[1])) findings.push(`${file}:${line(m.index)} icon button without a label`);
  }
}

console.log(list ? findings.join("\n") : findings.length);
