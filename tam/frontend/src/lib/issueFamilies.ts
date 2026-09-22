import type { RowPlace } from "@agile-suite/core";
import type { Issue } from "../api";

// drawnParents names the issues in a set that a subtask can actually sit
// under: an epic is a level up and a subtask cannot hold another one.
export function drawnParents(issues: Issue[]): Set<string> {
  return new Set(issues.filter((i) => i.type !== "subtask" && i.type !== "epic").map((i) => i.key));
}

// familyPlace says where one row sits in the set being drawn, which is what
// the row lead needs and what every one of these lists used to get wrong in
// the same way: a subtask whose parent is not in the set was drawn as a
// root, indistinguishable from a top-level story. Each of these lists is
// capped, paged, or grouped, so the parent falling outside the set is not an
// edge case; it is what the backlog's second page looks like.
export function familyPlace(issue: Issue, drawn: ReadonlySet<string>): RowPlace {
  if (issue.type !== "subtask" || !issue.parentKey) return "root";
  return drawn.has(issue.parentKey) ? "child" : "detached";
}

// Group only direct subtasks; an issue's epic link is a different level.
// Preserve root and sibling order, and leave uncached parents' children visible.
export function issueFamilies(issues: Issue[]): { parent: Issue; children: Issue[] }[] {
  const parents = drawnParents(issues);
  const children = new Map<string, Issue[]>();
  for (const issue of issues) {
    if (issue.type === "subtask" && parents.has(issue.parentKey)) {
      const siblings = children.get(issue.parentKey) ?? [];
      siblings.push(issue);
      children.set(issue.parentKey, siblings);
    }
  }
  return issues.filter((i) => i.type !== "subtask" || !parents.has(i.parentKey))
    .map((parent) => ({ parent, children: children.get(parent.key) ?? [] }));
}

export function orderFamilies(issues: Issue[]): Issue[] {
  return issueFamilies(issues).flatMap(({ parent, children }) => [parent, ...children]);
}

export function subtaskCounts(issues: Issue[]): Map<string, number> {
  return new Map(issueFamilies(issues).filter((f) => f.children.length > 0).map((f) => [f.parent.key, f.children.length]));
}

export function visibleFamilyIssues(issues: Issue[], collapsed: ReadonlySet<string>): Issue[] {
  const counts = subtaskCounts(issues);
  return issues.filter((i) => i.type !== "subtask" || !counts.has(i.parentKey) || !collapsed.has(i.parentKey));
}
