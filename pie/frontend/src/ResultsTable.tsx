import { useState } from "react";
import type { query } from "../wailsjs/go/models";
import { IssueDetail } from "./IssueDetail";
import { projectLabel } from "./linked";
import { Pager, usePaged } from "./Pager";

type Field = "project" | "status" | "priority" | "clientVisibility";
const FILTERS: { field: Field; label: string }[] = [
  { field: "project", label: "Project" },
  { field: "status", label: "Status" },
  { field: "priority", label: "Priority" },
  { field: "clientVisibility", label: "Client visibility" },
];

// ResultsTable lists the issues the query matched, one row each, a page at a
// time. Its filters narrow the run on screen and never ask Jira. A filter
// whose field the run holds no value for is left out, as is its column.
export function ResultsTable({ issues }: { issues: query.Issue[] }) {
  const [chosen, setChosen] = useState<Record<Field, string>>({ project: "", status: "", priority: "", clientVisibility: "" });
  const [openKey, setOpenKey] = useState<string | null>(null);
  const values = (f: Field) => [...new Set(issues.map((i) => i[f]))].filter(Boolean).sort();
  // A value the run no longer holds would hide every row, so it reads as All.
  const active = (f: Field) => (values(f).includes(chosen[f]) ? chosen[f] : "");
  const shown = issues.filter((i) => FILTERS.every(({ field }) => active(field) === "" || i[field] === active(field)));
  const names = new Map(issues.map((i) => [i.project, i.projectName]));
  const visibility = values("clientVisibility").length > 0;
  const { rows, pager } = usePaged(shown);
  const open = shown.find((i) => i.key === openKey);

  return (
    <>
      <div className="pie-filters">
        {FILTERS.filter(({ field }) => values(field).length > 0).map(({ field, label }) => (
          <label key={field}>
            {label}
            <select
              className="detail-input pie-filter-select"
              value={active(field)}
              onChange={(e) => {
                setChosen({ ...chosen, [field]: e.target.value });
                pager.onPage(0);
              }}
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
        <span className="muted" role="status">
          {shown.length < issues.length ? `Showing ${shown.length} of ${issues.length} issues` : ""}
        </span>
      </div>
      <div className="pie-results-split">
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
                {visibility && <th scope="col">Client visibility</th>}
                <th scope="col">Updated</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((issue) => (
                <tr key={issue.key} className={issue.key === openKey ? "pie-row-open" : undefined}>
                  <td>
                    <button type="button" className="pie-map-key mono" onClick={() => setOpenKey(issue.key)}>
                      {issue.key}
                    </button>
                  </td>
                  <td>{projectLabel(issue.project, issue.projectName)}</td>
                  <td>{issue.type}</td>
                  <td>{issue.summary}</td>
                  <td>{issue.status}</td>
                  <td>{issue.priority}</td>
                  {visibility && <td>{issue.clientVisibility}</td>}
                  <td>{issue.updated.slice(0, 10)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {open && <IssueDetail issue={open} onClose={() => setOpenKey(null)} />}
      </div>
      <Pager label="Results" {...pager} />
    </>
  );
}
