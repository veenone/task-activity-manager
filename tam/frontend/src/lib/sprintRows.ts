import type { SprintDetail } from "../api";
import { UNASSIGNED_SPRINT_STATE } from "../api";
import { groupByAssignee } from "./sprintGroups";
import { drawnParents, familyPlace, visibleFamilyIssues } from "./issueFamilies";

// The Sprints tree's row model: what the list draws, in the order it draws
// it. It is the keyboard model and the selection's reading order both, and
// it is pure, so it lives beside the other list helpers rather than inside
// the component that renders it.

// SCOPE_PREFIX namespaces a sprint's row id so it can never collide with an
// issue key, which matters more here than it looks: the selection's order
// array holds issue keys only, and lib/boardSelection's extend slices that
// array blindly, so an id that could pass for a key would end up checked and
// then in a bulk move.
const SCOPE_PREFIX = "sprint:";
const UNASSIGNED_ROW_ID = `${SCOPE_PREFIX}unassigned`;

// rowIdOf names one node of the list. The board's own unassigned work has no
// sprint id to be named by, so it gets a name of its own.
export function rowIdOf(detail: SprintDetail): string {
  return detail.state === UNASSIGNED_SPRINT_STATE ? UNASSIGNED_ROW_ID : `${SCOPE_PREFIX}${detail.id}`;
}

export interface TreeRow {
  id: string;
  kind: "sprint" | "issue";
  // scope is the sprint row the row belongs to, and its own id for a sprint
  // row. A shift gesture is measured inside one scope and nowhere else.
  scope: string;
  parentKey?: string;
}

// visibleRows flattens the list into the order it is drawn in, which is the
// keyboard model: each sprint, then its cards when it is open. The assignee
// separators are not rows here because they are not tree items: they are
// labels drawn between cards, so the tree stays two levels deep and nothing
// lands focus on a band heading that does nothing when activated.
export function visibleRows(details: SprintDetail[], expanded: ReadonlySet<string>, collapsed: ReadonlySet<string>): TreeRow[] {
  const rows: TreeRow[] = [];
  for (const detail of details) {
    const id = rowIdOf(detail);
    rows.push({ id, kind: "sprint", scope: id });
    if (!expanded.has(id)) continue;
    for (const group of groupByAssignee(detail.issues)) {
      const parents = drawnParents(group.issues);
      for (const issue of visibleFamilyIssues(group.issues, collapsed)) rows.push({ id: issue.key, kind: "issue", scope: id, parentKey: familyPlace(issue, parents) === "child" ? issue.parentKey : undefined });
    }
  }
  return rows;
}

// issueOrder is the selection's reading order, per scope: the cards of one
// sprint in the order they are drawn. A shift gesture extends inside one of
// these lists, which is what keeps a drag from the top of Sprint 12 to the
// bottom of Sprint 14 from checking three sprints' work at once. It cannot
// happen on a board, which draws one sprint at a time, and it is one drag
// away here.
export function issueOrder(details: SprintDetail[], collapsed: ReadonlySet<string> = new Set()): Map<string, string[]> {
  const out = new Map<string, string[]>();
  for (const detail of details) {
    out.set(rowIdOf(detail), groupByAssignee(detail.issues).flatMap((g) => visibleFamilyIssues(g.issues, collapsed).map((i) => i.key)));
  }
  return out;
}
