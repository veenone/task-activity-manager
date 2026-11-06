import { useEffect, useState } from "react";
import { Baselines } from "../wailsjs/go/main/App";
import type { cases, query } from "../wailsjs/go/models";
import { errorText } from "./AddProfileDialog";
import { BaselinesDialog } from "./BaselinesDialog";
import { OutOfStep, allProjects, baselineGroups, byKey, projectLabel, projectNames } from "./linked";
import { Pager, usePaged } from "./Pager";

// ProjectMap puts every linked set against every project it touches, so a
// status that moved in one project and not in another shows in one row. The
// project columns follow the analyst's baselines when there are any.
export function ProjectMap({ profileId, result, onOpen }: { profileId: string; result: query.Result; onOpen: (key: string) => void }) {
  const [onlyOutOfStep, setOnlyOutOfStep] = useState(false);
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
      </div>
      {error && (
        <p role="alert" className="error-text">
          {error}
        </p>
      )}
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
      {sets.length === 0 && <p className="muted">No linked sets to show.</p>}
      {editing && <BaselinesDialog profileId={profileId} baselines={baselines} onChange={setBaselines} onClose={() => setEditing(false)} />}
    </div>
  );
}
