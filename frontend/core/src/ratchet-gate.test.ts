import { execFileSync } from "node:child_process";
import { createRequire } from "node:module";
import fs from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

const repoRoot = path.resolve(__dirname, "../../..");
const require = createRequire(import.meta.url);
const { isBespokeModal } = require(path.join(repoRoot, "scripts/bespoke-modals.js")) as {
  isBespokeModal: (source: string) => boolean;
};

function counterCommand(name: string): string {
  const counters = fs.readFileSync(path.join(repoRoot, ".ratchet-counters"), "utf8");
  const line = counters.split(/\r?\n/).find((candidate) => candidate.startsWith(`${name}\t`));
  if (!line) throw new Error(`Missing ratchet counter: ${name}`);
  return line.slice(line.indexOf("\t") + 1);
}

describe("ratchet counters", () => {
  it("does not mistake an unrelated core import for shared Modal use", () => {
    expect(
      isBespokeModal(`
        import { useEscapeKey } from "@agile-suite/core";
        export const Panel = () => <div className="columns-backdrop" />;
      `),
    ).toBe(true);
  });

  it.each([
    'import { Modal, useEscapeKey } from "@agile-suite/core";',
    'import { Modal } from "./Modal";',
  ])("exempts overlay markup backed by a Modal import", (modalImport) => {
    expect(isBespokeModal(`${modalImport}\n<Modal className="modal example" />`)).toBe(false);
  });

  it("counts no app file building its own overlay", () => {
    const count = execFileSync("sh", ["-c", counterCommand("bespoke_modals")], {
      cwd: repoRoot,
      encoding: "utf8",
    }).trim();

    expect(Number(count)).toBe(0);
  }, 30_000);
});
