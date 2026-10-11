const fs = require("node:fs");

const overlayMarkup = /className\s*=\s*["'][^"']*(?:modal|overlay|backdrop)/;
const namedImport = /import\s+(?:type\s+)?\{([^}]*)\}\s+from\s+["']([^"']+)["']/gs;

function importsSharedModal(source) {
  for (const match of source.matchAll(namedImport)) {
    const importsModal = match[1].split(",").some((part) => {
      const imported = part.trim().replace(/^type\s+/, "").split(/\s+as\s+/)[0];
      return imported === "Modal";
    });
    const moduleName = match[2];
    const isModalModule =
      moduleName === "@agile-suite/core" ||
      (moduleName.startsWith(".") && /(^|\/)Modal$/.test(moduleName));
    if (importsModal && isModalModule) return true;
  }
  return false;
}

function isBespokeModal(source) {
  const code = source
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/^\s*\/\/.*$/gm, "");
  return overlayMarkup.test(code) && !importsSharedModal(code);
}

if (require.main === module) {
  const files = fs
    .readFileSync(0, "utf8")
    .split("\n")
    .map((file) => file.replace(/\r$/, ""))
    .filter(Boolean);
  const count = files.filter((file) =>
    isBespokeModal(fs.readFileSync(file, "utf8")),
  ).length;
  process.stdout.write(`${count}\n`);
}

module.exports = { isBespokeModal };
