import { useState } from "react";
import type { query } from "../wailsjs/go/models";
import { BarChart } from "./BarChart";
import { projectLabel, projectNames } from "./linked";
import { ExportButton } from "./ExportButton";
import { statisticsSheets } from "./exportSheets";
import { Pager, usePaged } from "./Pager";
import { useSort } from "./sort";
import { ProjectInput } from "./ProjectInput";
import { statistics } from "./stats";

const RANGES = [
  { days: 0, label: "All time" },
  { days: 90, label: "Last 90 days" },
  { days: 365, label: "Last 12 months" },
  { days: 730, label: "Last 2 years" },
];

const dayText = (n: number | null) => (n === null ? "-" : `${n} days`);

export function Card({ value, label }: { value: number | string; label: string }) {
  return (
    <li className="pie-stat">
      <strong>{value}</strong> <span>{label}</span>
    </li>
  );
}

// Statistics counts the run on screen. productField is the profile's product
// base field; without one, product bases are left out rather than shown as 0.
export function Statistics({ result, productField }: { result: query.Result; productField: string }) {
  const [chosen, setProject] = useState("");
  const [withinDays, setWithinDays] = useState(0);
  const projects = [...new Set(result.issues.filter((i) => i.matched).map((i) => i.project))].sort();
  // A filter on a project the new run does not hold would zero every number.
  const project = projects.includes(chosen) ? chosen : "";
  const s = statistics(result, { project, withinDays }, Date.now());
  const names = projectNames(result);
  const perProject = useSort(s.projects, {
    project: (p) => projectLabel(p.project, names.get(p.project)),
    bases: (p) => p.bases.join(", "),
    issues: (p) => p.issues,
    open: (p) => p.open,
    age: (p) => p.medianAge ?? "",
  });
  const byStatus = useSort(s.finished, {
    status: (f) => f.status,
    issues: (f) => f.issues,
    dated: (f) => f.dated,
    days: (f) => f.medianDays ?? "",
  });
  const { rows, pager } = usePaged(perProject.sorted);
  // The field is read when a query runs; a run from before it was set has none.
  const bases = productField !== "" && result.issues.some((i) => (i.bases ?? []).length > 0);

  return (
    <section className="pie-stats" aria-label="Statistics">
      <div className="pie-filters">
        <ProjectInput
          label="Project"
          projects={projects}
          projectLabel={(p) => projectLabel(p, names.get(p))}
          value={project}
          onChange={setProject}
        />
        <label>
          Created
          <select
            className="detail-input pie-filter-select"
            value={withinDays}
            onChange={(e) => setWithinDays(Number(e.target.value))}
          >
            {RANGES.map((r) => (
              <option key={r.days} value={r.days}>
                {r.label}
              </option>
            ))}
          </select>
        </label>
        <span className="muted">{s.issues === 1 ? "1 issue" : `${s.issues} issues`} the query matched</span>
        <ExportButton name="pie-statistics" sheets={() => statisticsSheets({ ...s, projects: perProject.sorted, finished: byStatus.sorted }, (p) => projectLabel(p, names.get(p)))} />
      </div>

      <ul className="pie-stat-cards" aria-label="Headline numbers">
        <Card value={s.open} label="not finished" />
        <Card value={dayText(s.medianAge)} label="median age, not finished" />
        <Card value={s.multiProject} label="reach more than one project" />
        {bases && <Card value={s.multiBase} label="reach more than one product base" />}
        <Card value={s.outOfStep} label="linked sets out of step" />
      </ul>
      {!bases && (
        <p className="muted">
          {productField === ""
            ? "Name the product base field under Jira fields, then run the query again, to count product bases."
            : "Run the query again to read the product base field."}
        </p>
      )}

      <div className="pie-charts">
        <BarChart title="Issues created per month" bars={s.created} />
        <BarChart title="Age of unfinished issues, days" bars={s.ages} />
        <BarChart title="Projects each issue reaches" bars={s.reach} />
      </div>

      <table className="pie-table" aria-label="Per project">
        <thead>
          <tr>
            {perProject.th("project", "Project")}
            {bases && perProject.th("bases", "Product base")}
            {perProject.th("issues", "Issues")}
            {perProject.th("open", "Not finished")}
            {perProject.th("age", "Median age, not finished")}
          </tr>
        </thead>
        <tbody>
          {rows.map((p) => (
            <tr key={p.project}>
              <td>{projectLabel(p.project, names.get(p.project))}</td>
              {bases && <td>{p.bases.join(", ")}</td>}
              <td>{p.issues}</td>
              <td>{p.open}</td>
              <td>{dayText(p.medianAge)}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <Pager label="Per project" {...pager} />

      <table className="pie-table" aria-label="Finished, by status">
        <thead>
          <tr>
            {byStatus.th("status", "Status")}
            {byStatus.th("issues", "Issues")}
            {byStatus.th("dated", "With a resolution date")}
            {byStatus.th("days", "Median time to resolve")}
          </tr>
        </thead>
        <tbody>
          {byStatus.sorted.map((f) => (
            <tr key={f.status}>
              <td>{f.status}</td>
              <td>{f.issues}</td>
              <td>{f.dated}</td>
              <td>{dayText(f.medianDays)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}
