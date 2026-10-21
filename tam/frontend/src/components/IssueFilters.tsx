import { useMemo } from "react";
import { MultiSelect, useProfile } from "@agile-suite/core";
import { ASSIGNEE_NONE } from "../api";
import type { Profile, Settings } from "../api";
import { useFilterOptions } from "../queries/issues";
import { useProjectTypes } from "../queries/people";
import { typeChips } from "../lib/issueFilter";
import type { IssueFilter } from "../lib/issueFilter";

interface Props {
  value: IssueFilter;
  onChange: (next: IssueFilter) => void;
}

// IssueFilters is the three controls every view that lists issues narrows by:
// type, status and assignee. One component for all of them, so the five views
// cannot drift apart on labels, order or behaviour, which is what #321 found
// them doing.
//
// It owns no state. Each view keeps its own IssueFilter, because the filters
// belong to the view the user set them on, and resets it on a profile switch
// along with its own page and selection.
//
// The sprint filter is not here: Boards and Sprints pick a sprint to decide
// what to draw at all, so for them it is not a filter over a list but the
// subject of the view.
export function IssueFilters({ value, onChange }: Props) {
  const { activeId } = useProfile<Profile, Settings>();
  // The two reads are here rather than passed in because all five views
  // would otherwise plumb the same two queries and the same memo through to
  // the same place. They are one query each for the whole app, under one key,
  // so a second view mounting costs nothing.
  const projectTypes = useProjectTypes(activeId);
  const options = useFilterOptions(activeId);
  const chips = useMemo(() => typeChips(projectTypes.data), [projectTypes.data]);
  return (
    <>
      {/* One control rather than one chip per type. A project with five types
          spent five controls on one filter, and the action buttons were what
          fell to the second row (#323). Still several types at once, because
          "stories and bugs" is a real question to ask of a backlog. */}
      <MultiSelect
        options={chips.map((t) => ({ value: t.id, label: t.label }))}
        selected={value.types}
        onChange={(types) => onChange({ ...value, types })}
        allLabel="All types"
        title="Filter by issue type"
      />
      <select
        className="detail-input filter-status"
        aria-label="Status"
        value={value.status}
        onChange={(e) => onChange({ ...value, status: e.target.value })}
      >
        <option value="">All statuses</option>
        {(options.data?.statuses ?? []).map((s) => (
          <option key={s} value={s}>{s}</option>
        ))}
      </select>
      <select
        className="detail-input filter-assignee"
        aria-label="Assignee"
        value={value.assignee}
        onChange={(e) => onChange({ ...value, assignee: e.target.value })}
      >
        <option value="">All assignees</option>
        {/* Nobody is a choice of its own, not the absence of one. It is offered
            whether or not the cache holds an unassigned row: a project with
            none still answers the question, with an empty view. */}
        <option value={ASSIGNEE_NONE}>Unassigned</option>
        {(options.data?.assignees ?? []).map((a) => (
          <option key={a} value={a}>{a}</option>
        ))}
      </select>
    </>
  );
}
