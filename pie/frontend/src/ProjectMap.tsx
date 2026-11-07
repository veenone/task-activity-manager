import { useEffect, useState } from "react";
import { Baselines } from "../wailsjs/go/main/App";
import type { cases, query } from "../wailsjs/go/models";
import { errorText } from "./AddProfileDialog";
import { BaselinesDialog } from "./BaselinesDialog";
import { OutOfStep, allProjects, baselineGroups, byKey, projectLabel, projectNames } from "./linked";
import { MapFlow } from "./MapFlow";
import { MapTree } from "./MapTree";
import { Pager, usePaged } from "./Pager";
import { Card } from "./Statistics";

const VIEWS = ["Flow", "Tree", "Table"] as const;

// ProjectMap puts every linked set against every project it touches, so a
// status that moved in one project and not in another shows in one row. The
// project columns follow the analyst's baselines when there are any.
export function ProjectMap({ profileId, result, onOpen }: { profileId: string; result: query.Result; onOpen: (key: string) => void }) {
  const [onlyOutOfStep, setOnlyOutOfStep] = useState(false);
  const [view, setView] = useState<(typeof VIEWS)[number]>("Flow");
  const [baselines, setBaselines] = useState<cases.Baseline[]>([]);
  const [editing, setEditing] = useState(false);
  const [error, setError] = useState("");
  const issues = byKey(result);
  const names = projectNames(result);
  const groups = baselineGroups(allProjects(result.sets), baselines);
  const projects = groups.flatMap((g) => g.projects);
  const grouped = groups.some((g) => g.baseline !== null);
  const sets = result.sets.filter((s) => !onlyOutOfStep || s.outOfStep);
  const { rows, pager } = usePaged(sets);
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

  const span = grouped ? 2 : undefined;
  return (
    <div className="pie-linked">
      <div className="pie-filters">
        <label className="pie-check">
          <input type="checkbox" checked={onlyOutOfStep} onChange={(e) => setOnlyOutOfStep(e.target.checked)} />
          Out of step only
        </label>
        <button type="button" className="btn btn-ghost" onClick={() => setEditing(true)}>
          Baselines
        </button>
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
      {view === "Table" && (
        <>
          <div className="pie-results">
            <table className="pie-table pie-map" aria-label="Project map">
              <thead>
                <tr>
                  <th scope="col" rowSpan={span}>
                    Linked set
                  </th>
                  {grouped
                    ? groups.map((g) => (
                        <th key={g.baseline ?? ""} scope="colgroup" colSpan={g.projects.length}>
                          {g.baseline === null ? "No baseline" : `Baseline ${projectLabel(g.baseline, names.get(g.baseline))}`}
                        </th>
                      ))
                    : projects.map((p) => (
                        <th key={p} scope="col">
                          {projectLabel(p, names.get(p))}
                        </th>
                      ))}
                  <th scope="col" rowSpan={span}>
                    State
                  </th>
                </tr>
                {grouped && (
                  <tr>
                    {projects.map((p) => (
                      <th key={p} scope="col">
                        {projectLabel(p, names.get(p))}
                      </th>
                    ))}
                  </tr>
                )}
              </thead>
              <tbody>
                {rows.map((s) => (
                  <tr key={s.keys[0]}>
                    <th scope="row" className="mono">
                      {s.keys[0]}
                    </th>
                    {projects.map((p) => {
                      const here = s.keys.filter((k) => issues.get(k)?.project === p);
                      return (
                        <td key={p}>
                          {here.length === 0
                            ? "-"
                            : here.map((k) => (
                                <button key={k} type="button" className="pie-map-key" onClick={() => onOpen(k)}>
                                  <span className="mono">{k}</span> {issues.get(k)?.status}
                                </button>
                              ))}
                        </td>
                      );
                    })}
                    <td>
                      <OutOfStep outOfStep={s.outOfStep} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <Pager label="Project map" {...pager} />
        </>
      )}
      {sets.length === 0 && <p className="muted">No linked sets to show.</p>}
      {editing && <BaselinesDialog profileId={profileId} baselines={baselines} onChange={setBaselines} onClose={() => setEditing(false)} />}
    </div>
  );
}
