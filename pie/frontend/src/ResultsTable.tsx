import { useState } from "react";
import type { query } from "../wailsjs/go/models";
import { projectLabel } from "./linked";
import { Pager, usePaged } from "./Pager";

type Field = "project" | "status" | "priority";
const FILTERS: { field: Field; label: string }[] = [
  { field: "project", label: "Project" },
  { field: "status", label: "Status" },
  { field: "priority", label: "Priority" },
];

// ResultsTable lists the issues the query matched, one row each, a page at a
// time. Its filters narrow the run on screen and never ask Jira.
export function ResultsTable({ issues }: { issues: query.Issue[] }) {
  const [chosen, setChosen] = useState<Record<Field, string>>({ project: "", status: "", priority: "" });
  const values = (f: Field) => [...new Set(issues.map((i) => i[f]))].filter(Boolean).sort();
  // A value the run no longer holds would hide every row, so it reads as All.
  const active = (f: Field) => (values(f).includes(chosen[f]) ? chosen[f] : "");
  const shown = issues.filter((i) => FILTERS.every(({ field }) => active(field) === "" || i[field] === active(field)));
  const names = new Map(issues.map((i) => [i.project, i.projectName]));
  const { rows, pager } = usePaged(shown);

  return (
    <>
      <div className="pie-filters">
        {FILTERS.map(({ field, label }) => (
          <label key={field}>
            {label}
            <select
              className="detail-input pie-filter-select"
              value={active(field)}
              onChange={(e) => setChosen({ ...chosen, [field]: e.target.value })}
            >
              <option value="">All</option>
              {values(field).map((v) => (
                <option key={v} value={v}>
                  {field === "project" ? projectLabel(v, names.get(v)) : v}
                </option>
              ))}
            </select>
          </label>
        ))}
        {shown.length < issues.length && <span className="muted">{`Showing ${shown.length} of ${issues.length} issues`}</span>}
      </div>
      <div className="pie-results">
        <table className="pie-table" aria-label="Query results">
          <thead>
            <tr>
              <th scope="col">Key</th>
              <th scope="col">Project</th>
              <th scope="col">Type</th>
              <th scope="col">Summary</th>
              <th scope="col">Status</th>
              <th scope="col">Priority</th>
              <th scope="col">Updated</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((issue) => (
              <tr key={issue.key}>
                <td className="mono">{issue.key}</td>
                <td>{projectLabel(issue.project, issue.projectName)}</td>
                <td>{issue.type}</td>
                <td>{issue.summary}</td>
                <td>{issue.status}</td>
                <td>{issue.priority}</td>
                <td>{issue.updated.slice(0, 10)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <Pager label="Results" {...pager} />
    </>
  );
}
