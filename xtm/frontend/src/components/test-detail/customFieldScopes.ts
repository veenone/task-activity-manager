import type { CustomFieldValue } from "../../api";

// Custom fields arrive from more than one namespace, and only one of them is
// editable here.
//
// A custom field edit commits as a Jira issue field update, so a field that
// lives on an execution or on a test step cannot be edited from a test: the
// store refuses it, and an input that silently fails to save is worse than no
// input. A Zephyr Essential profile has all three; Xray and Kiwi have the
// issue fields alone and see no change (#371 Phase 3).
//
// Mirrors backend.FieldScope* in internal/backend/dto.go.
const SCOPES: Array<{ scope: string; label: string; editable: boolean }> = [
  { scope: "issue", label: "On the test", editable: true },
  { scope: "execution", label: "On an execution", editable: false },
  { scope: "step", label: "On a step", editable: false },
];

const OTHER = { label: "Elsewhere", editable: false };

export interface FieldGroup {
  label: string;
  editable: boolean;
  fields: CustomFieldValue[];
}

// fieldGroups splits the fields into the sections the panel renders, in the
// order above, leaving out a section this profile has no fields for.
//
// A field with no scope is a field on the issue: that is what every definition
// cached before the scope existed meant, and dropping those would empty an
// Xray profile's custom fields until its next sync. A scope this build does
// not know keeps its field in a section of its own rather than hiding it,
// because a field the backend reported and the panel never shows is invisible
// to the user and to us.
export function fieldGroups(fields: CustomFieldValue[]): FieldGroup[] {
  const known = new Set(SCOPES.map((s) => s.scope));
  const groups: FieldGroup[] = [];
  for (const s of SCOPES) {
    const owned = fields.filter((f) => (f.scope || "issue") === s.scope);
    if (owned.length > 0) groups.push({ label: s.label, editable: s.editable, fields: owned });
  }
  const rest = fields.filter((f) => f.scope && !known.has(f.scope));
  if (rest.length > 0) groups.push({ ...OTHER, fields: rest });
  return groups;
}
