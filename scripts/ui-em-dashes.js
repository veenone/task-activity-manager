const fs = require("node:fs");

const files = fs
  .readFileSync(0, "utf8")
  .split("\n")
  .map((file) => file.replace(/\r$/, ""))
  .filter((file) => file && !file.includes(".test."));

let count = 0;
for (const file of files) {
  for (const line of fs.readFileSync(file, "utf8").split(/\r?\n/)) {
    if (!line.includes("—")) continue;
    // Comments, JSX ones included, are not UI.
    if (/^\s*(?:\/\/|\*|\/\*|\{\/\*)/.test(line)) continue;
    count += 1;
  }
}

process.stdout.write(`${count}\n`);
