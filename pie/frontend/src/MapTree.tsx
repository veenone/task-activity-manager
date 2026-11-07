import { useState } from "react";
import type { query } from "../wailsjs/go/models";
import { OutOfStep, projectLabel } from "./linked";

type Group = { baseline: string | null; projects: string[] };

// MapTree branches each baseline into its projects and each project into the
// issues its linked sets hold there. Choosing a set marks its issues wherever
// they sit, which the table shows as one row.
export function MapTree({
  sets,
  issues,
  groups,
  names,
  onOpen,
}: {
  sets: query.Set[];
  issues: Map<string, query.Issue>;
  groups: Group[];
  names: Map<string, string>;
  onOpen: (key: string) => void;
}) {
  const [chosen, setChosen] = useState("");
  const label = (p: string) => projectLabel(p, names.get(p));
  const lit = new Set(sets.find((s) => s.keys[0] === chosen)?.keys ?? []);
  const keysIn = (p: string) => [...new Set(sets.flatMap((s) => s.keys))].filter((k) => issues.get(k)?.project === p).sort();

  return (
    <div className="pie-tree-view">
      <div className="pie-tree-scroll">
        <ul className="pie-tree" aria-label="Project tree">
          {groups.map((g) => (
            <li key={g.baseline ?? ""}>
              <span className="pie-tree-node">{g.baseline === null ? "No baseline" : `Baseline ${label(g.baseline)}`}</span>
              <ul>
                {g.projects.map((p) => {
                  const keys = keysIn(p);
                  const fixed = keys.filter((k) => issues.get(k)?.statusCategory === "done").length;
                  return (
                    <li key={p}>
                      <span className="pie-tree-node">
                        {label(p)}
                        <span className="muted">{`${fixed} of ${keys.length} fixed`}</span>
                        <progress value={fixed} max={Math.max(1, keys.length)} aria-hidden="true" />
                      </span>
                      <ul>
                        {keys.map((k) => (
                          <li key={k} className="pie-tree-issue" aria-current={lit.has(k) ? "true" : undefined}>
                            <button type="button" className="pie-map-key" onClick={() => onOpen(k)}>
                              <span className="mono">{k}</span> {issues.get(k)?.status}
                            </button>
                          </li>
                        ))}
                      </ul>
                    </li>
                  );
                })}
              </ul>
            </li>
          ))}
        </ul>
      </div>
      <section className="pie-flow-side">
        <h3>Linked sets</h3>
        <ul aria-label="Linked sets">
          {sets.map((s) => (
            <li key={s.keys[0]}>
              <button
                type="button"
                className="pie-set"
                aria-pressed={chosen === s.keys[0]}
                onClick={() => setChosen(chosen === s.keys[0] ? "" : s.keys[0])}
              >
                <span className="mono">{s.keys[0]}</span> <OutOfStep outOfStep={s.outOfStep} />
              </button>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}
