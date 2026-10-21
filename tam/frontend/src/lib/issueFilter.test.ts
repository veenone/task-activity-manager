import { describe, it, expect } from "vitest";
import { ASSIGNEE_NONE } from "../api";
import type { Issue } from "../api";
import { NO_FILTER, filterIssues, isFiltered, matchesFilter, queryFields, typeChips } from "./issueFilter";

function issue(over: Partial<Issue>): Issue {
  return {
    key: "PLAT-1", id: "1", project: "PLAT", type: "story", summary: "A row", status: "To Do",
    assignee: "", reporter: "", priority: "", labels: [], sprintId: "", sprintName: "",
    parentKey: "", storyPoints: null, rank: "", created: "", updated: "", ...over,
  };
}

describe("queryFields", () => {
  // The bar holds one choice per field; the store takes lists. An unset field
  // has to become an empty list and not a list holding "", because the store
  // reads an empty entry as a real value.
  it("sends nothing for the fields the user left alone", () => {
    expect(queryFields(NO_FILTER)).toEqual({ types: [], statuses: [], assignees: [] });
  });

  it("sends one entry per choice", () => {
    expect(queryFields({ types: ["bug", "task"], status: "To Do", assignee: "S. Kim" })).toEqual({
      types: ["bug", "task"],
      statuses: ["To Do"],
      assignees: ["S. Kim"],
    });
  });

  // This is the whole reason ASSIGNEE_NONE exists: the select needed a value
  // for nobody that was not the empty string its All option uses, and this is
  // the one place it is turned back into the empty name the store matches.
  it("turns the Unassigned choice into the empty name", () => {
    expect(queryFields({ ...NO_FILTER, assignee: ASSIGNEE_NONE }).assignees).toEqual([""]);
  });
});

describe("isFiltered", () => {
  it("is false only when nothing is set", () => {
    expect(isFiltered(NO_FILTER)).toBe(false);
    expect(isFiltered({ ...NO_FILTER, types: ["bug"] })).toBe(true);
    expect(isFiltered({ ...NO_FILTER, status: "Done" })).toBe(true);
    expect(isFiltered({ ...NO_FILTER, assignee: ASSIGNEE_NONE })).toBe(true);
  });
});

describe("matchesFilter", () => {
  const bug = issue({ key: "PLAT-9", type: "bug", status: "In Progress", assignee: "S. Kim" });

  // The two views that filter in the browser have to mean the same thing by a
  // filter as the two that filter in SQL, or the same choice would show
  // different work depending on which tab the user made it on.
  it("keeps everything when nothing is set", () => {
    expect(matchesFilter(bug, NO_FILTER)).toBe(true);
  });

  it("is an AND across the fields", () => {
    expect(matchesFilter(bug, { types: ["bug"], status: "In Progress", assignee: "S. Kim" })).toBe(true);
    expect(matchesFilter(bug, { types: ["bug"], status: "Done", assignee: "S. Kim" })).toBe(false);
  });

  it("is an OR inside the type field", () => {
    expect(matchesFilter(bug, { ...NO_FILTER, types: ["task", "bug"] })).toBe(true);
    expect(matchesFilter(bug, { ...NO_FILTER, types: ["task", "story"] })).toBe(false);
  });

  it("matches the Unassigned choice against a row with no assignee", () => {
    const nobody = issue({ assignee: "" });
    expect(matchesFilter(nobody, { ...NO_FILTER, assignee: ASSIGNEE_NONE })).toBe(true);
    expect(matchesFilter(bug, { ...NO_FILTER, assignee: ASSIGNEE_NONE })).toBe(false);
  });
});

describe("typeChips", () => {
  // A profile that has never synced has no type list to read, so the bar
  // offers the six types TAM models rather than offering nothing.
  it("falls back to the types TAM models when the project's are unknown", () => {
    expect(typeChips(undefined).map((c) => c.id)).toContain("bug");
    expect(typeChips([]).every((c) => c.modelled)).toBe(true);
  });

  // Two Jira types can mean the same logical type, and two chips selecting the
  // same rows is a bar claiming two filters it does not have.
  it("gives two Jira types that mean the same logical type one chip", () => {
    const chips = typeChips([
      { name: "Task", logical: "task" },
      { name: "Todo", logical: "task" },
    ]);
    expect(chips).toHaveLength(1);
    expect(chips[0].title).toBe("Task");
  });

  // A type TAM has no concept of keeps the project's own name and the neutral
  // chip, the rule #67 set for the New issue dialog.
  it("keeps a type TAM does not model under the project's own name", () => {
    const [chip] = typeChips([{ name: "Improvement", logical: "" }]);
    expect(chip).toMatchObject({ id: "Improvement", label: "Improvement", modelled: false });
  });
});

describe("filterIssues", () => {
  const parent = issue({ key: "PLAT-1", type: "story", assignee: "R. Anand" });
  const sub = issue({ key: "PLAT-2", type: "subtask", parentKey: "PLAT-1", assignee: "S. Kim" });
  const other = issue({ key: "PLAT-3", type: "bug", assignee: "M. Ortiz" });
  const rows = [parent, sub, other];

  // The Family rows contract: a subtask is drawn indented under its parent, so
  // keeping the subtask without the parent would indent a row under nothing.
  it("keeps the parent of a matching subtask", () => {
    expect(filterIssues(rows, { ...NO_FILTER, assignee: "S. Kim" }).map((i) => i.key)).toEqual(["PLAT-1", "PLAT-2"]);
  });

  it("keeps the subtasks of a matching parent", () => {
    expect(filterIssues(rows, { ...NO_FILTER, assignee: "R. Anand" }).map((i) => i.key)).toEqual(["PLAT-1", "PLAT-2"]);
  });

  it("drops a family nothing in it matches", () => {
    expect(filterIssues(rows, { ...NO_FILTER, assignee: "M. Ortiz" }).map((i) => i.key)).toEqual(["PLAT-3"]);
  });

  // By identity, so a render with no filter rebuilds no lists.
  it("returns the list untouched when nothing is set", () => {
    expect(filterIssues(rows, NO_FILTER)).toBe(rows);
  });
});
