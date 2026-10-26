import { ASSIGNEE_NONE, ISSUE_TYPES } from "../api";
import type { Issue, IssueType } from "../api";

// IssueFilter is what the filter bar holds for every view that lists issues.
// Type is a set because the bar draws one toggle per type and reads best with
// several on; status and assignee are one choice each, the way XTM's own bar
// has them.
//
// assignee is the select's raw value: "" for every assignee, ASSIGNEE_NONE for
// unassigned, or a display name. queryFields below is what turns that into the
// list the store takes.
export interface IssueFilter {
  types: string[];
  status: string;
  assignee: string;
}

// NO_FILTER is the bar showing everything. It is also what a profile switch
// resets to, so it is frozen: a view that mutated a shared default would carry
// one profile's filter into the next.
export const NO_FILTER: Readonly<IssueFilter> = Object.freeze({ types: [], status: "", assignee: "" });

// isFiltered says whether the user has narrowed anything, which is what lets an
// empty view say "nothing matches this filter" rather than "nothing is cached".
export function isFiltered(f: IssueFilter): boolean {
  return f.types.length > 0 || f.status !== "" || f.assignee !== "";
}

// queryFields is the bar's state as IssueQuery and TreeQuery carry it. Both
// take lists, so a single choice becomes a one-entry list and no choice becomes
// an empty one.
//
// The assignee mapping is the one place ASSIGNEE_NONE is read: the store knows
// only that an empty name means unassigned, and the select needed a value for
// that which was not the empty string its All option already uses.
export function queryFields(f: IssueFilter): { types: string[]; statuses: string[]; assignees: string[] } {
  return {
    types: f.types,
    statuses: f.status === "" ? [] : [f.status],
    assignees: f.assignee === "" ? [] : [f.assignee === ASSIGNEE_NONE ? "" : f.assignee],
  };
}

// matchesFilter is the same filter applied to a row in hand, for the two views
// that draw data they already hold: the board composes itself from one read and
// the sprint list from another, so narrowing them in SQL would mean refetching
// to answer a question the browser can already answer.
//
// It has to agree with what the store does, or the same filter would mean two
// things depending on which tab the user is on: an AND across the fields, an OR
// within one, and an empty assignee matching the Unassigned choice.
export function matchesFilter(issue: Issue, f: IssueFilter): boolean {
  if (f.types.length > 0 && !f.types.includes(issue.type)) return false;
  if (f.status !== "" && issue.status !== f.status) return false;
  if (f.assignee === "") return true;
  return issue.assignee === (f.assignee === ASSIGNEE_NONE ? "" : f.assignee);
}

// keptKeys is which rows survive a filter. Two predicates, because a filter
// bar and a search box ask different questions of a family (#353):
//
// anchor says which rows select the family they belong to. narrow says which
// rows inside a selected family are drawn, and defaults to anchor for the
// callers that have one question.
//
// A subtask is drawn only when it matches narrow. Its parent is drawn either
// way, because the Family rows contract would otherwise leave the subtask
// indented under a row that is not there, and the parent is the only honest
// root to give it. Siblings that match nothing stay out, which is what
// separates this from keeping the family whole: a filter on Story used to
// return every subtask under every story.
//
// The predicates are functions rather than an IssueFilter because the board
// ANDs its own text box into the first one. Both callers get one family rule
// this way, which is the point: the board and the sprint list would
// otherwise each hold a copy of it.
export function keptKeys(
  issues: Issue[],
  anchor: (i: Issue) => boolean,
  narrow: (i: Issue) => boolean = anchor,
): Set<string> {
  const families = new Set(issues.filter(anchor).map((i) => (i.type === "subtask" ? i.parentKey : i.key)));
  return new Set(
    issues
      .filter((i) => (i.type === "subtask" ? families.has(i.parentKey) && narrow(i) : families.has(i.key)))
      .map((i) => i.key),
  );
}

// filterIssues is keptKeys applied to a list in hand, for the Sprints view.
// An unset filter returns the list by identity, so a render with no filter
// rebuilds nothing.
export function filterIssues(issues: Issue[], f: IssueFilter): Issue[] {
  if (!isFiltered(f)) return issues;
  const keep = keptKeys(issues, (i) => matchesFilter(i, f));
  return issues.filter((i) => keep.has(i.key));
}

// TypeChip is one toggle on the type filter: the value the store holds in the
// type column, the short label the chip shows, the project's own name as the
// tooltip, and whether TAM models the type (which decides the chip's colour).
export interface TypeChip {
  id: string;
  label: string;
  title: string;
  modelled: boolean;
}

// typeChips is what the bar offers: one chip per type the project has, in the
// order Jira lists them, keyed by the value the sync stored in the type column.
// A type TAM models is that logical type, under TAM's own short label; a type it
// does not is the project's own name, on the neutral chip, the rule #67 set for
// the New issue dialog.
//
// Two Jira types can mean the same logical type ("Task" and "Todo" both do),
// and two chips filtering for the same rows is a filter bar lying about having
// two filters, so the first one wins. A project whose types have never been
// synced falls back to the six TAM models, which is what the bar offered before
// it could know better.
export function typeChips(projectTypes: { name: string; logical: string }[] | undefined): TypeChip[] {
  if (!projectTypes || projectTypes.length === 0) {
    return ISSUE_TYPES.map((t) => ({ id: t.id, label: t.short, title: t.label, modelled: true }));
  }
  const chips = new Map<string, TypeChip>();
  for (const t of projectTypes) {
    const id = t.logical || t.name;
    const short = ISSUE_TYPES.find((x) => x.id === (t.logical as IssueType))?.short;
    if (!chips.has(id)) chips.set(id, { id, label: short ?? t.name, title: t.name, modelled: !!short });
  }
  return [...chips.values()];
}
