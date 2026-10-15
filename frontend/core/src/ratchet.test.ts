import { execFileSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";

const repoRoot = path.resolve(__dirname, "../../..");
const tempDirs: string[] = [];

afterEach(() => {
  for (const dir of tempDirs.splice(0)) {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

function shellQuote(value: string): string {
  return `'${value.replaceAll("'", `'"'"'`)}'`;
}

function counterCommand(name: string): string {
  const line = fs
    .readFileSync(path.join(repoRoot, ".ratchet-counters"), "utf8")
    .split(/\r?\n/)
    .find((candidate) => candidate.startsWith(`${name}\t`));
  if (!line) throw new Error(`${name} counter is missing`);
  return line.slice(line.indexOf("\t") + 1);
}

function runCounter(
  name: string,
  trackedFiles: string[],
  lineEnding: string,
): string {
  const list = path.join(path.dirname(trackedFiles[0]), `${name}-files.txt`);
  const files = trackedFiles
    .map((file) => file.replaceAll("\\", "/"))
    .join(lineEnding);
  fs.writeFileSync(list, `${files}${lineEnding}`);
  const command = [
    `git() { node -e 'process.stdout.write(require("node:fs").readFileSync(process.argv[1]))' ${shellQuote(list.replaceAll("\\", "/"))}; }`,
    counterCommand(name),
  ].join("\n");

  return execFileSync("sh", ["-c", command], {
    cwd: repoRoot,
    encoding: "utf8",
  }).trim();
}

describe("files_over_400 ratchet counter", () => {
  it.each([
    ["LF", "\n"],
    ["CRLF", "\r\n"],
  ])("counts tracked files from a %s-terminated list", (_label, lineEnding) => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "files-over-400-"));
    tempDirs.push(dir);
    const large = path.join(dir, "large.ts");
    const boundary = path.join(dir, "boundary.ts");
    fs.writeFileSync(large, "line\n".repeat(401));
    fs.writeFileSync(boundary, "line\n".repeat(400));

    expect(runCounter("files_over_400", [large, boundary], lineEnding)).toBe(
      "1",
    );
  });
});

describe("ui_em_dashes ratchet counter", () => {
  it.each([
    ["LF", "\n"],
    ["CRLF", "\r\n"],
  ])("counts shipped UI lines from a %s-terminated list", (_label, lineEnding) => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "ui-em-dashes-"));
    tempDirs.push(dir);
    const source = path.join(dir, "View.tsx");
    const test = path.join(dir, "View.test.tsx");
    // A JSX comment is no more visible than a line comment (#260).
    fs.writeFileSync(source, 'const label = "Wait — retry";\n// ignored — comment\n      {/* ignored — JSX comment */}\n');
    fs.writeFileSync(test, 'const label = "Test — copy";\n');

    expect(runCounter("ui_em_dashes", [source, test], lineEnding)).toBe("1");
  });
});

describe("bespoke_modals ratchet counter", () => {
  it.each([
    ["LF", "\n"],
    ["CRLF", "\r\n"],
  ])("counts only real overlays from a %s-terminated list", (_label, lineEnding) => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "bespoke-modals-"));
    tempDirs.push(dir);
    const bespoke = path.join(dir, "Bespoke.tsx");
    const shared = path.join(dir, "Shared.tsx");
    const local = path.join(dir, "Local.tsx");
    fs.writeFileSync(bespoke, '<div className="modal-overlay" />\n');
    fs.writeFileSync(
      shared,
      'import { Modal } from "@agile-suite/core";\n<div className="modal-overlay" />\n',
    );
    fs.writeFileSync(
      local,
      'import { Modal } from "./Modal";\n<div className="modal-overlay" />\n',
    );

    expect(
      runCounter("bespoke_modals", [bespoke, shared, local], lineEnding),
    ).toBe("1");
  });
});
