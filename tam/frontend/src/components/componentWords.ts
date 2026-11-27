import { assigneeLabel } from "@agile-suite/core";
import { ENTITY_COMPONENT_CREATE, ENTITY_COMPONENT_EDIT } from "../api";
import type { ComponentDelete, ComponentFields, PendingChange } from "../api";

// The Pending changes dialog's words for component changes (#605), beside
// releaseWords.ts.

function parse<T>(text: string): T | null {
  try {
    return JSON.parse(text) as T;
  } catch {
    return null;
  }
}

function lead(f: ComponentFields): string {
  return f.leadDisplayName || f.leadName;
}

// componentChangeName is the component's name: an edit's before value, the
// name every other change carries.
export function componentChangeName(row: PendingChange): string {
  const from = row.entityType === ENTITY_COMPONENT_EDIT ? row.beforeVal : row.afterVal;
  return parse<{ name?: string }>(from)?.name || `component ${row.entityKey}`;
}

// componentChangeLine reads one component change in words.
export function componentChangeLine(row: PendingChange, project: string): string {
  const name = componentChangeName(row);
  if (row.entityType === ENTITY_COMPONENT_CREATE) {
    const d = parse<ComponentFields>(row.afterVal);
    if (!d) return "A draft component that could not be read. Discard it and draft it again.";
    const bits = [`New component in ${project}`];
    if (lead(d)) bits.push(`lead ${lead(d)}`);
    if (d.assigneeType) bits.push(`default assignee ${assigneeLabel(d.assigneeType)}`);
    return bits.join(", ");
  }
  if (row.entityType !== ENTITY_COMPONENT_EDIT) {
    const d = parse<ComponentDelete>(row.afterVal);
    return d?.moveToName ? `Delete ${name}, its issues move to ${d.moveToName}` : `Delete ${name}`;
  }
  const was = parse<ComponentFields>(row.beforeVal);
  const now = parse<ComponentFields>(row.afterVal);
  if (!was || !now) return `Edit component ${name}: the change could not be read. Discard it and edit the component again.`;
  const changed: string[] = [];
  if (now.name !== was.name) changed.push(`name to ${now.name}`);
  if (now.description !== was.description) changed.push(now.description ? "description changed" : "description removed");
  if (now.leadName !== was.leadName) changed.push(now.leadName ? `lead to ${lead(now)}` : "lead removed");
  if ((now.assigneeType || "PROJECT_DEFAULT") !== (was.assigneeType || "PROJECT_DEFAULT")) {
    changed.push(`default assignee to ${assigneeLabel(now.assigneeType)}`);
  }
  return `Edit component ${name}: ${changed.length ? changed.join(", ") : "nothing changed"}`;
}

// componentChangeNote says what Commit does with a component change.
export function componentChangeNote(row: PendingChange): string {
  switch (row.entityType) {
    case ENTITY_COMPONENT_CREATE: return "Commit creates it in Jira before the issue changes that name it.";
    case ENTITY_COMPONENT_EDIT: return "Saved locally. Commit sends the change to Jira.";
  }
  return "Commit deletes it in Jira after the issue changes. The issues themselves are not deleted.";
}
