import { describe, it, expect } from "vitest";
import { fieldGroups } from "./customFieldScopes";
import type { CustomFieldValue } from "../../api";

function field(fieldId: string, scope: string): CustomFieldValue {
  return { fieldId, name: fieldId, type: "", value: "", scope } as CustomFieldValue;
}

describe("fieldGroups", () => {
  it("separates the fields on the issue from the ones on a run", () => {
    // A Zephyr profile has all three. Rendering them as one list invites an
    // edit to a field that is not on the Jira issue, which the store refuses
    // and which could only ever fail at commit.
    const groups = fieldGroups([
      field("customfield_10102", "issue"),
      field("1", "execution"),
      field("2", "step"),
    ]);
    expect(groups.map((g) => [g.label, g.fields.length, g.editable])).toEqual([
      ["On the test", 1, true],
      ["On an execution", 1, false],
      ["On a step", 1, false],
    ]);
  });

  it("leaves out a scope this profile has no fields for", () => {
    // An Xray or Kiwi profile has the issue fields alone. With one group the
    // panel labels nothing, so those profiles look exactly as they did.
    const groups = fieldGroups([field("customfield_10102", "issue")]);
    expect(groups.map((g) => g.label)).toEqual(["On the test"]);
  });

  it("treats a field with no scope as a field on the issue", () => {
    // Definitions cached before the scope column existed come back with an
    // empty one. Dropping them would empty an Xray profile's custom fields
    // until the next sync.
    const groups = fieldGroups([field("customfield_10102", "")]);
    expect(groups.map((g) => [g.label, g.fields.length])).toEqual([["On the test", 1]]);
  });

  it("keeps a scope it does not know about rather than hiding the field", () => {
    const groups = fieldGroups([field("9", "cycle")]);
    expect(groups.map((g) => [g.label, g.editable])).toEqual([["Elsewhere", false]]);
  });

  it("reports nothing for a profile with no custom fields", () => {
    expect(fieldGroups([])).toEqual([]);
  });
});
