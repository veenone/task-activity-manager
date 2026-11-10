import { useState } from "react";
import type { query } from "../wailsjs/go/models";
import { IssueDetail } from "./IssueDetail";
import { projectLabel } from "./linked";
import { ProjectInput } from "./ProjectInput";
import { ExportButton } from "./ExportButton";
import { resultsSheet } from "./exportSheets";
import { Pager, usePaged } from "./Pager";
import { useSort } from "./sort";

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
  const [search, setSearch] = useState("");
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const [openKey, setOpenKey] = useState<string | null>(null);
  const values = (f: Field) => [...new Set(issues.map((i) => i[f]))].filter(Boolean).sort();
  // A value the run no longer holds would hide every row, so it reads as All.
  const active = (f: Field) => (values(f).includes(chosen[f]) ? chosen[f] : "");
  const hasCases = caseKeys.size > 0;
  const names = new Map(issues.map((i) => [i.project, i.projectName]));
  const label = (p: string) => projectLabel(p, names.get(p));
  // Search reads what the row shows: its key, summary and project (#454).
  const words = search.trim().toLowerCase();
  const found = (i: query.Issue) => words === "" || [i.key, i.summary, label(i.project)].some((t) => t.toLowerCase().includes(words));
  const { sorted: shown, th } = useSort(
    issues.filter(
      (i) =>
        FILTERS.every(({ field }) => active(field) === "" || i[field] === active(field)) &&
        (!hasCases || withCase === "" || caseKeys.has(i.key) === (withCase === "with")) &&
        found(i),
    ),
    {
      key: (i) => i.key,
      project: (i) => label(i.project),
      type: (i) => i.type,
      summary: (i) => i.summary,
      status: (i) => i.status,
      priority: (i) => i.priority,
      clientVisibility: (i) => i.clientVisibility,
      case: (i) => (caseKeys.has(i.key) ? "Case" : ""),
      updated: (i) => i.updated,
    },
  );
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
        <label>
          Search
          <input
            type="search"
            className="detail-input pie-filter-select"
            placeholder="Key, summary or project"
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              pager.onPage(0);
            }}
          />
        </label>
        {values("project").length > 0 && (
          <ProjectInput
            label="Project"
            allText="All"
            projects={values("project")}
            projectLabel={label}
            value={active("project")}
            onChange={(p) => {
              setChosen({ ...chosen, project: p });
              pager.onPage(0);
            }}
          />
        )}
        {FILTERS.filter(({ field }) => field !== "project" && values(field).length > 0).map(({ field, label }) => (
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
        ))}
        <ExportButton
          name="pie-results"
          sheets={() => [
            resultsSheet(shown, label, { visibility, caseKeys: hasCases ? caseKeys : null }),
          ]}
        />
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
                {th("key", "Key")}
                {th("project", "Project")}
                {th("type", "Type")}
                {th("summary", "Summary")}
                {th("status", "Status")}
                {th("priority", "Priority")}
                {visibility && th("clientVisibility", "Client visibility")}
                {hasCases && th("case", "Case")}
                {th("updated", "Updated")}
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
