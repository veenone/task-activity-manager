import { ENTITY_COMPONENT_DELETE, ENTITY_COMPONENT_EDIT } from "../api";
import type { ComponentSummary, PendingChange } from "../api";

// componentWaiting says, per component id, what a pending change will do to
// it, the way releaseWaiting does for releases. A draft says so itself.
export function componentWaiting(rows: PendingChange[]): Map<number, string> {
  const out = new Map<number, string>();
  for (const row of rows) {
    const id = Number(row.entityKey);
    if (row.entityType === ENTITY_COMPONENT_DELETE) out.set(id, "Deleting on Commit");
    else if (row.entityType === ENTITY_COMPONENT_EDIT && !out.has(id)) out.set(id, "Edited, not committed");
  }
  return out;
}

// COMPONENT_SORTS are the list's orders, XTM's with issues in place of tests.
export const COMPONENT_SORTS = [
  { value: "name", label: "Name" },
  { value: "issues", label: "Issues" },
  { value: "lead", label: "Lead" },
] as const;

export type ComponentSort = (typeof COMPONENT_SORTS)[number]["value"];

const leadOf = (c: ComponentSummary) => c.leadDisplayName || c.leadName;

// filterComponents keeps the components whose name, description or lead
// holds the filter text, ignoring case, in the chosen order.
export function filterComponents(all: ComponentSummary[], filter: string, sort: ComponentSort): ComponentSummary[] {
  const q = filter.trim().toLowerCase();
  const kept = q
    ? all.filter((c) => c.name.toLowerCase().includes(q) || c.description.toLowerCase().includes(q) || leadOf(c).toLowerCase().includes(q))
    : all;
  return [...kept].sort((a, b) => {
    if (sort === "issues") return b.issues - a.issues || a.name.localeCompare(b.name);
    if (sort === "lead") return leadOf(a).localeCompare(leadOf(b)) || a.name.localeCompare(b.name);
    return a.name.localeCompare(b.name);
  });
}
