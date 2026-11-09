import { useState } from "react";
import type { query } from "../wailsjs/go/models";
import { IssueDetail } from "./IssueDetail";
import { projectLabel } from "./linked";
import { ProjectInput } from "./ProjectInput";
import { Pager, usePaged } from "./Pager";

type Field = "project" | "status" | "priority" | "clientVisibility";
const FILTERS: { field: Field; label: string }[] = [
  { field: "project", label: "Project" },
  { field: "status", label: "Status" },
  { field: "priority", label: "Priority" },
  { field: "clientVisibility", label: "Client visibility" },
];
type CaseFilter = "" | "with" | "without";

// ResultsTable lists the issues the query matched, one row each, a page at a
// time. Its filters narrow the run on screen and never ask Jira. A filter
// whose field the run holds no value for is left out, as is its column. The
// analyst picks rows and opens them as cases; caseKeys are the issues that
// already have one.
export function ResultsTable({
  issues,
  caseKeys,
  onOpenCases,
}: {
  issues: query.Issue[];
  caseKeys: Set<string>;
  // onOpenCases resolves true once the cases are stored; on false the
  // selection stays so the analyst can try again.
  onOpenCases: (keys: string[]) => Promise<boolean>;
}) {
  const [chosen, setChosen] = useState<Record<Field, string>>({ project: "", status: "", priority: "", clientVisibility: "" });
  const [withCase, setWithCase] = useState<CaseFilter>("");
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const [openKey, setOpenKey] = useState<string | null>(null);
  const values = (f: Field) => [...new Set(issues.map((i) => i[f]))].filter(Boolean).sort();
  // A value the run no longer holds would hide every row, so it reads as All.
  const active = (f: Field) => (values(f).includes(chosen[f]) ? chosen[f] : "");
  const hasCases = caseKeys.size > 0;
  const shown = issues.filter(
    (i) =>
      FILTERS.every(({ field }) => active(field) === "" || i[field] === active(field)) &&
      (!hasCases || withCase === "" || caseKeys.has(i.key) === (withCase === "with")),
  );
  const names = new Map(issues.map((i) => [i.project, i.projectName]));
  const visibility = values("clientVisibility").length > 0;
  const { rows, pager } = usePaged(shown);
  const open = shown.find((i) => i.key === openKey);
  // Only rows still shown count, so a filter cannot open a case out of sight.
  const selected = shown.filter((i) => picked.has(i.key)).map((i) => i.key);
  const pageAll = rows.length > 0 && rows.every((i) => picked.has(i.key));

  function toggle(keys: string[], on: boolean) {
    const next = new Set(picked);
    for (const k of keys) {
      if (on) next.add(k);
      else next.delete(k);
    }
    setPicked(next);
  }

  return (
    <>
      <div className="pie-filters">
        {FILTERS.filter(({ field }) => values(field).length > 0).map(({ field, label }) =>
          field === "project" ? (
            <ProjectInput
              key={field}
              label={label}
              allText="All"
              projects={values(field)}
              projectLabel={(p) => projectLabel(p, names.get(p))}
              value={active(field)}
              onChange={(p) => {
                setChosen({ ...chosen, [field]: p });
                pager.onPage(0);
              }}
            />
          ) : (
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
                  {v}
                </option>
              ))}
            </select>
          </label>
          ),
        )}
        {hasCases && (
          <label>
            Case
            <select
              className="detail-input pie-filter-select"
              value={withCase}
              onChange={(e) => {
                setWithCase(e.target.value as CaseFilter);
                pager.onPage(0);
              }}
            >
              <option value="">All</option>
              <option value="with">With a case</option>
              <option value="without">Without a case</option>
            </select>
          </label>
        )}
        {selected.length > 0 && (
          <button
            type="button"
            className="btn btn-primary"
            onClick={() => {
              void onOpenCases(selected).then((done) => done && setPicked(new Set()));
            }}
          >
            {`Open ${selected.length} as cases`}
          </button>
        )}
        <span className="muted" role="status">
          {shown.length < issues.length ? `Showing ${shown.length} of ${issues.length} issues` : ""}
        </span>
      </div>
      <div className="pie-results-split">
        <div className="pie-results">
          <table className="pie-table" aria-label="Query results">
            <thead>
              <tr>
                <th scope="col">
                  <input
                    type="checkbox"
                    aria-label="Select all on this page"
                    checked={pageAll}
                    onChange={(e) => toggle(rows.map((i) => i.key), e.target.checked)}
                  />
                </th>
                <th scope="col">Key</th>
                <th scope="col">Project</th>
                <th scope="col">Type</th>
                <th scope="col">Summary</th>
                <th scope="col">Status</th>
                <th scope="col">Priority</th>
                {visibility && <th scope="col">Client visibility</th>}
                {hasCases && <th scope="col">Case</th>}
                <th scope="col">Updated</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((issue) => (
                <tr key={issue.key} className={issue.key === openKey ? "pie-row-open" : undefined}>
                  <td>
                    <input
                      type="checkbox"
                      aria-label={`Select ${issue.key}`}
                      checked={picked.has(issue.key)}
                      onChange={(e) => toggle([issue.key], e.target.checked)}
                    />
                  </td>
                  <td>
                    <button type="button" className="pie-map-key mono" onClick={() => setOpenKey(issue.key)}>
                      {issue.key}
                    </button>
                  </td>
                  <td>{projectLabel(issue.project, issue.projectName)}</td>
                  <td>{issue.type}</td>
                  <td className="pie-summary">{issue.summary}</td>
                  <td>{issue.status}</td>
                  <td>{issue.priority}</td>
                  {visibility && <td>{issue.clientVisibility}</td>}
                  {hasCases && <td>{caseKeys.has(issue.key) ? <span className="pie-case">Case</span> : ""}</td>}
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
