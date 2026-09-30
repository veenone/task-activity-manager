import { describe, expect, it } from "vitest";
import { agreementItems, agreementRows, effectiveAgreement } from "./doneAgreement";

// The body the Go template writes for a board's agreement, with items typed
// into it. The blank task is what a fresh document carries.
const board = `<p>What this team agrees has to be true before a piece of work counts as done.</p><h2>Items</h2>` +
  `<ac:task-list>` +
  `<ac:task><ac:task-id>1</ac:task-id><ac:task-status>incomplete</ac:task-status><ac:task-body>Reviewed by someone who did not write it</ac:task-body></ac:task>` +
  `<ac:task><ac:task-id>2</ac:task-id><ac:task-status>complete</ac:task-status><ac:task-body>Unit tests pass</ac:task-body></ac:task>` +
  `<ac:task><ac:task-status>incomplete</ac:task-status><ac:task-body></ac:task-body></ac:task>` +
  `</ac:task-list>`;

const additions = `<p><strong>Sprint:</strong> Sprint 12</p><h2>Extra items</h2>` +
  `<ac:task-list>` +
  `<ac:task><ac:task-status>incomplete</ac:task-status><ac:task-body>Load test run against staging</ac:task-body></ac:task>` +
  `<ac:task><ac:task-status>incomplete</ac:task-status><ac:task-body>Unit tests pass</ac:task-body></ac:task>` +
  `</ac:task-list>`;

describe("agreementItems", () => {
  it("reads the items out of a done agreement's task list", () => {
    expect(agreementItems(board)).toEqual([
      "Reviewed by someone who did not write it",
      "Unit tests pass",
    ]);
  });

  it("reads the same items whether the document's own boxes are ticked or not", () => {
    expect(agreementItems(board.replace(/complete/g, "incomplete"))).toEqual(agreementItems(board));
  });

  it("carries no item for a fresh document, whose one task is empty", () => {
    const fresh = `<p>What this team agrees has to be true.</p><h2>Items</h2>` +
      `<ac:task-list><ac:task><ac:task-status>incomplete</ac:task-status><ac:task-body></ac:task-body></ac:task></ac:task-list>`;
    expect(agreementItems(fresh)).toEqual([]);
  });

  it("identifies an item by its words, not by how the editor wrapped them", () => {
    const wrapped = `<ac:task-list><ac:task><ac:task-status>incomplete</ac:task-status><ac:task-body>\n  Unit   tests\n  pass\n</ac:task-body></ac:task></ac:task-list>`;
    expect(agreementItems(wrapped)).toEqual(["Unit tests pass"]);
  });

  it("reads an item's own words and not those of a list nested under it", () => {
    const nested = `<ac:task-list><ac:task><ac:task-status>incomplete</ac:task-status><ac:task-body>Tested` +
      `<ac:task-list><ac:task><ac:task-status>incomplete</ac:task-status><ac:task-body>by hand</ac:task-body></ac:task></ac:task-list>` +
      `</ac:task-body></ac:task></ac:task-list>`;
    expect(agreementItems(nested)).toEqual(["Tested", "by hand"]);
  });

  it("keeps the words of a formatted item, marks and links included", () => {
    const rich = `<ac:task-list><ac:task><ac:task-status>incomplete</ac:task-status><ac:task-body><strong>Docs</strong> updated</ac:task-body></ac:task></ac:task-list>`;
    expect(agreementItems(rich)).toEqual(["Docs updated"]);
  });

  it("answers with no item for a body it cannot read rather than throwing", () => {
    expect(agreementItems("<p>unclosed")).toEqual([]);
    expect(agreementItems("")).toEqual([]);
  });
});

describe("effectiveAgreement", () => {
  it("is the board's items and then the sprint's additions", () => {
    expect(effectiveAgreement(board, additions)).toEqual([
      "Reviewed by someone who did not write it",
      "Unit tests pass",
      "Load test run against staging",
    ]);
  });

  it("is the board's items alone for an issue in no sprint", () => {
    expect(effectiveAgreement(board, "")).toEqual([
      "Reviewed by someone who did not write it",
      "Unit tests pass",
    ]);
  });

  it("names an item once however many times the documents say it", () => {
    expect(effectiveAgreement(additions, additions)).toEqual([
      "Load test run against staging",
      "Unit tests pass",
    ]);
  });
});

describe("agreementRows", () => {
  const items = ["Reviewed by someone else", "Unit tests pass"];

  it("is every item the agreement states, ticked where the store says so", () => {
    expect(agreementRows(items, ["Unit tests pass"])).toEqual([
      { text: "Reviewed by someone else", ticked: false, stated: true },
      { text: "Unit tests pass", ticked: true, stated: true },
    ]);
  });

  it("keeps a tick made against words the agreement no longer states, after the items", () => {
    expect(agreementRows(items, ["Unit tests pass on CI", "Unit tests pass"])).toEqual([
      { text: "Reviewed by someone else", ticked: false, stated: true },
      { text: "Unit tests pass", ticked: true, stated: true },
      { text: "Unit tests pass on CI", ticked: true, stated: false },
    ]);
  });

  it("is the items alone for an issue nobody has ticked anything on", () => {
    expect(agreementRows(items, [])).toEqual([
      { text: "Reviewed by someone else", ticked: false, stated: true },
      { text: "Unit tests pass", ticked: false, stated: true },
    ]);
  });

  it("is the ticks alone once the agreement states nothing", () => {
    expect(agreementRows([], ["Unit tests pass"])).toEqual([
      { text: "Unit tests pass", ticked: true, stated: false },
    ]);
  });
});
