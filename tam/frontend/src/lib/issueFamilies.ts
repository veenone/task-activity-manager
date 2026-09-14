import type { Issue } from "../api";

// Group only direct subtasks; an issue's epic link is a different level.
// Preserve root and sibling order, and leave uncached parents' children visible.
export function issueFamilies(issues: Issue[]): { parent: Issue; children: Issue[] }[] {
  const parents = new Set(issues.filter((i) => i.type !== "subtask" && i.type !== "epic").map((i) => i.key));
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
