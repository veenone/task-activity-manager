import { useState } from "react";
import type { query } from "../wailsjs/go/models";
import { OutOfStep, allProjects, byKey, projectLabel, projectNames } from "./linked";
import { Pager, usePaged } from "./Pager";

// ProjectMap puts every linked set against every project it touches, so a
// status that moved in one project and not in another shows in one row.
export function ProjectMap({ result, onOpen }: { result: query.Result; onOpen: (key: string) => void }) {
  const [onlyOutOfStep, setOnlyOutOfStep] = useState(false);
  const issues = byKey(result);
  const projects = allProjects(result.sets);
  const names = projectNames(result);
  const sets = result.sets.filter((s) => !onlyOutOfStep || s.outOfStep);
  const { rows, pager } = usePaged(sets);

  return (
    <div className="pie-linked">
      <label className="pie-check">
        <input type="checkbox" checked={onlyOutOfStep} onChange={(e) => setOnlyOutOfStep(e.target.checked)} />
        Out of step only
      </label>
      <div className="pie-results">
        <table className="pie-table pie-map" aria-label="Project map">
          <thead>
            <tr>
              <th scope="col">Linked set</th>
              {projects.map((p) => (
                <th key={p} scope="col">
                  {projectLabel(p, names.get(p))}
                </th>
              ))}
              <th scope="col">State</th>
            </tr>
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
    </div>
  );
}
