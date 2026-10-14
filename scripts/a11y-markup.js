// a11y-markup counts the markup a screen reader cannot make sense of (#227):
// a <table> with no accessible name, a <th> with no scope, and an icon-only
// button (✕, ×) with no aria-label, whose name would be the glyph. It reads
// file paths from stdin, one per line, and prints one number. With --list it
// prints each finding instead.
const fs = require("fs");

const files = fs.readFileSync(0, "utf8").split(/\r?\n/).filter((f) => f && !f.includes(".test."));
const list = process.argv.includes("--list");
const findings = [];

// openTags yields each opening <name ...> tag with its attribute text. The tag
// ends at the first ">" outside braces and quotes, so an attribute like
// onClick={() => close()} does not end it early. A regex stopping at the first
// ">" did, and missed every button with an arrow-function handler.
function* openTags(src, name) {
  const re = new RegExp(`<${name}\\b`, "g");
  for (let m = re.exec(src); m; m = re.exec(src)) {
    let depth = 0;
    let quote = "";
    let i = m.index + m[0].length;
    for (; i < src.length; i++) {
      const c = src[i];
      if (quote) {
        if (c === quote) quote = "";
      } else if (c === '"' || c === "'" || c === "`") {
        quote = c;
      } else if (c === "{") {
        depth++;
      } else if (c === "}") {
        depth--;
      } else if (c === ">" && depth === 0) {
        break;
      }
    }
    yield { index: m.index, attrs: src.slice(m.index + m[0].length, i), end: i + 1 };
  }
}

for (const file of files) {
  const src = fs.readFileSync(file, "utf8");
  const line = (i) => src.slice(0, i).split("\n").length;
  for (const t of openTags(src, "table")) {
    // A <caption> right inside the table names it as well as an aria-label.
    const named = /aria-label(ledby)?=/.test(t.attrs) || /^\s*<caption\b/.test(src.slice(t.end));
    if (!named) findings.push(`${file}:${line(t.index)} table without a name`);
  }
  for (const t of openTags(src, "th")) {
    if (!/\bscope=/.test(t.attrs)) findings.push(`${file}:${line(t.index)} th without scope`);
  }
  for (const t of openTags(src, "button")) {
    const iconOnly = /^\s*(✕|×|✖)\s*<\/button>/.test(src.slice(t.end));
    if (iconOnly && !/aria-label=/.test(t.attrs)) {
      findings.push(`${file}:${line(t.index)} icon button without a label`);
    }
  }
}

console.log(list ? findings.join("\n") : findings.length);
