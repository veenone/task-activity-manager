import { useEffect, useState } from "react";
import { Baselines } from "../wailsjs/go/main/App";
import type { cases, query } from "../wailsjs/go/models";
import { errorText } from "./ProfileForm";
import { BaselinesDialog } from "./BaselinesDialog";
import { allProjects, baselineGroups, byKey, projectLabel, projectNames } from "./linked";
import { filterSets, type MapFilter } from "./mapData";
import { ExportButton } from "./ExportButton";
import { mapSheet, setSheet } from "./exportSheets";
import { MapFlow } from "./MapFlow";
import { BaselineView } from "./BaselineView";
import { MapTree } from "./MapTree";
import { MapCoverage } from "./MapCoverage";
import { ProjectInput } from "./ProjectInput";
import { Card } from "./Statistics";

const VIEWS = ["Flow", "Tree", "Baseline", "Coverage"] as const;
const NO_FILTER: MapFilter = { baseline: "", project: "", key: "" };

// ProjectMap puts every linked set against every project it touches, so a
// status that moved in one project and not in another shows in one row. The
// project columns follow the analyst's baselines when there are any.
export function ProjectMap({ profileId, result, onOpen }: { profileId: string; result: query.Result; onOpen: (key: string) => void }) {
  const [onlyOutOfStep, setOnlyOutOfStep] = useState(false);
  const [filter, setFilter] = useState<MapFilter>(NO_FILTER);
  // clears remounts the project box, so Clear filters also empties text that named no project.
  const [clears, setClears] = useState(0);
  const [view, setView] = useState<(typeof VIEWS)[number]>("Flow");
  const [baselines, setBaselines] = useState<cases.Baseline[]>([]);
  const [editing, setEditing] = useState(false);
  const [error, setError] = useState("");
  const issues = byKey(result);
  const names = projectNames(result);
  const groups = baselineGroups(allProjects(result.sets), baselines);
  const projects = groups.flatMap((g) => g.projects);
  // The filters narrow every view; the count cards stay on the whole run.
  // A baseline or project that a new run or a baselines edit took away counts
  // as no choice, so the map never empties under a select that shows none.
  const baseline = groups.some((g) => (g.baseline ?? "-") === filter.baseline) ? filter.baseline : "";
  const pickable = baseline === "" ? projects : (groups.find((g) => (g.baseline ?? "-") === baseline)?.projects ?? []);
  const live: MapFilter = { baseline, project: pickable.includes(filter.project) ? filter.project : "", key: filter.key };
  const sets = filterSets(result.sets, groups, live).filter((s) => !onlyOutOfStep || s.outOfStep);
  const filtered = onlyOutOfStep || live.baseline !== "" || live.project !== "" || live.key.trim() !== "";
  // Flow and Tree draw only the projects the shown sets reach.
  const shown = baselineGroups(allProjects(sets), baselines);
  const inSets = new Set(result.sets.flatMap((s) => s.keys));
  const needFix = result.sets
    .filter((s) => s.outOfStep)
    .flatMap((s) => s.keys)
    .filter((k) => issues.get(k)?.statusCategory !== "done");
  const unplaced = groups.find((g) => g.baseline === null)?.projects.length ?? 0;

  useEffect(() => {
    let current = true;
    Baselines(profileId).then(
      (b) => current && setBaselines(b),
      (e) => current && setError(errorText(e)),
    );
    return () => {
      current = false;
    };
  }, [profileId]);

  return (
    <div className="pie-linked">
      <div className="pie-filters">
        <label className="pie-check">
          <input type="checkbox" checked={onlyOutOfStep} onChange={(e) => setOnlyOutOfStep(e.target.checked)} />
          Out of step only
        </label>
        <label>
          Baseline
          <select
            className="detail-input pie-filter-select"
            value={live.baseline}
            onChange={(e) => setFilter({ ...live, baseline: e.target.value, project: "" })}
          >
            <option value="">All baselines</option>
            {groups.map((g) => (
              <option key={g.baseline ?? "-"} value={g.baseline ?? "-"}>
                {g.baseline === null ? "No baseline" : `Baseline ${projectLabel(g.baseline, names.get(g.baseline))}`}
              </option>
            ))}
          </select>
        </label>
        <ProjectInput
          key={clears}
          label="Project"
          projects={pickable}
          projectLabel={(p) => projectLabel(p, names.get(p))}
          value={live.project}
          onChange={(p) => setFilter({ ...live, project: p })}
        />
        <label>
          Key
          <input
            className="detail-input pie-filter-select"
            type="search"
            value={filter.key}
            placeholder="Part of a key"
            onChange={(e) => setFilter({ ...filter, key: e.target.value })}
          />
        </label>
        {filtered && (
          <>
            <span className="muted">{`Showing ${sets.length} of ${result.sets.length} linked sets`}</span>
            <button
              type="button"
              className="btn btn-ghost"
              onClick={() => {
                setFilter(NO_FILTER);
                setClears((n) => n + 1);
                setOnlyOutOfStep(false);
              }}
            >
              Clear filters
            </button>
          </>
        )}
        <button type="button" className="btn btn-ghost" onClick={() => setEditing(true)}>
          Baselines
        </button>
        <ExportButton
          name="pie-project-map"
          sheets={() => {
            const label = (p: string) => projectLabel(p, names.get(p));
            return [mapSheet(sets, groups, issues, label), setSheet(sets, issues, label)];
          }}
        />
        <div className="view-tabs pie-map-views" role="group" aria-label="Map view">
          {VIEWS.map((v) => (
            <button
              key={v}
              type="button"
              className={`view-tab${v === view ? " view-tab-active" : ""}`}
              aria-pressed={v === view}
              onClick={() => setView(v)}
            >
              {v}
            </button>
          ))}
        </div>
      </div>
      <ul className="pie-stat-cards" aria-label="Map counts">
        <Card value={`${result.sets.filter((s) => s.outOfStep).length} of ${result.sets.length}`} label="linked sets out of step" />
        <Card value={needFix.length} label="issues still to fix" />
        <Card value={inSets.size} label={`issues across ${projects.length} projects`} />
        <Card
          value={groups.length - (unplaced > 0 ? 1 : 0)}
          label={`baselines, and ${unplaced === 1 ? "1 project" : `${unplaced} projects`} under none`}
        />
      </ul>
      {error && (
        <p role="alert" className="error-text">
          {error}
        </p>
      )}
      {view === "Flow" && <MapFlow sets={sets} issues={issues} groups={shown} names={names} onOpen={onOpen} />}
      {view === "Tree" && <MapTree sets={sets} issues={issues} groups={shown} names={names} onOpen={onOpen} />}
      {view === "Baseline" && (
        <BaselineView result={result} baselines={baselines} onOpen={onOpen} />
      )}
      {view === "Coverage" && <MapCoverage sets={sets} groups={groups} issues={issues} names={names} onOpen={onOpen} />}
      {sets.length === 0 && <p className="muted">No linked sets to show.</p>}
      {editing && <BaselinesDialog profileId={profileId} baselines={baselines} onChange={setBaselines} onClose={() => setEditing(false)} />}
    </div>
  );
}
