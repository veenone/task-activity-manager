const fs = require("node:fs");

const files = fs
  .readFileSync(0, "utf8")
  .split("\n")
  .map((file) => file.replace(/\r$/, ""))
  .filter((file) => file && !file.includes("wailsjs"));

let count = 0;
for (const file of files) {
  const source = fs.readFileSync(file);
  let lines = 0;
  for (const byte of source) {
    if (byte === 0x0a) lines += 1;
  }
  if (lines > 400) count += 1;
}

process.stdout.write(`${count}\n`);
