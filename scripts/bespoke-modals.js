const fs = require("node:fs");

const files = fs
  .readFileSync(0, "utf8")
  .split("\n")
  .map((file) => file.replace(/\r$/, ""))
  .filter(Boolean);

let count = 0;
for (const file of files) {
  const source = fs.readFileSync(file, "utf8");
  if (!/className="[^"]*(?:modal|overlay|backdrop)/.test(source)) continue;
  if (source.includes("@agile-suite/core")) continue;
  if (/from "(?:\.\/|\.\.\/)+Modal"/.test(source)) continue;
  count += 1;
}

process.stdout.write(`${count}\n`);
