import type { Issue } from "../api";

// UNASSIGNED_LABEL is what the separator over the cards nobody owns prints.
// It is the same word the Boards view's assignee swimlane uses for the same
// absence, and deliberately not "Board backlog", which boardrepo gives the
// node holding work that is in no sprint: a sprint can hold unassigned cards
// and the board backlog can hold assigned ones, so the two must not share a
// word.
export const UNASSIGNED_LABEL = "Unassigned";

// AssigneeGroup is one separator and the cards under it.
export interface AssigneeGroup {
  // id is the assignee exactly as the cache holds it, "" for nobody. It is
  // what a group's rows are keyed and scoped by, so two people whose display
  // names collide are still two groups.
  id: string;
  label: string;
  issues: Issue[];
  points: number;
}

// groupByAssignee partitions one sprint's cards into the bands the tree
// draws them under: most work first, and the cards nobody owns last.
//
// The order is spelled out rather than left to the Map, and that is the
// whole reason this function exists. A Map iterates in insertion order,
// which here is the order the cards came back in, which is board_issue's
// own position order; a sync that reorders a board would then reshuffle the
// separators under every sprint, so a reader who learned where their own
// name sits would have to find it again after every refresh. Points first
// says which band is carrying the sprint, the count breaks a tie between two
// bands with no estimates at all, and the name breaks the last one so the
// order is total rather than merely mostly decided.
export function groupByAssignee(issues: Issue[]): AssigneeGroup[] {
  const byAssignee = new Map<string, AssigneeGroup>();
  for (const issue of issues) {
    let group = byAssignee.get(issue.assignee);
    if (!group) {
      group = { id: issue.assignee, label: issue.assignee || UNASSIGNED_LABEL, issues: [], points: 0 };
      byAssignee.set(issue.assignee, group);
    }
    group.issues.push(issue);
    group.points += issue.storyPoints ?? 0;
  }
  return [...byAssignee.values()].sort(compare);
}

function compare(a: AssigneeGroup, b: AssigneeGroup): number {
  // Nobody's cards read last however much work they carry: the band is a
  // gap in the sprint's ownership, not a person's share of it.
  const aNobody = a.id === "";
  const bNobody = b.id === "";
  if (aNobody !== bNobody) return aNobody ? 1 : -1;
  if (a.points !== b.points) return b.points - a.points;
  if (a.issues.length !== b.issues.length) return b.issues.length - a.issues.length;
  return a.label.localeCompare(b.label);
}
