import { type CSSProperties, useState } from "react";
import type { query } from "../wailsjs/go/models";
import { coverage, leastCovered, percent } from "./coverage";
import { projectLabel } from "./linked";
import { FIX_STATES } from "./mapData";
import { Pager, usePaged } from "./Pager";

type Group = { baseline: string | null; projects: string[] };

const SORTS = ["Run order", "Least covered first"] as const;
const label = (id: string) => FIX_STATES.find((f) => f.id === id)?.label ?? id;

// MapCoverage replaces the Project map's Table (#473): a row per linked set,
// a column per project grouped under its baseline. A red gap is a project of
// a baseline the set reaches that has no issue in it, the fix still missing.
export function MapCoverage({
  sets,
  groups,
  issues,
  names,
  onOpen,
}: {
  sets: query.Set[];
  groups: Group[];
  issues: Map<string, query.Issue>;
  names: Map<string, string>;
  onOpen: (key: string) => void;
}) {
  const [gapsOnly, setGapsOnly] = useState(false);
  const [sort, setSort] = useState<(typeof SORTS)[number]>("Run order");
  const name = (p: string) => projectLabel(p, names.get(p));
  const grouped = groups.some((g) => g.baseline !== null);
  const { rows: all, columns } = coverage(sets, groups, issues);
  const kept = all.filter((r) => !gapsOnly || r.cells.includes("gap"));
  const { rows, pager } = usePaged(sort === "Run order" ? kept : leastCovered(kept));
  const span = grouped ? 2 : undefined;

  return (
    <>
      <div className="pie-cov-bar">
        <ul className="pie-cov-legend" aria-label="Cell key">
          <li>
            <span className="pie-cov-key pie-cov-done">done</span>
          </li>
          <li>
            <span className="pie-cov-key pie-cov-open">open</span>
          </li>
          <li>
            <span className="pie-cov-gap">gap</span>
          </li>
          <li className="muted">· not in this baseline</li>
        </ul>
        <label className="pie-check">
          <input type="checkbox" checked={gapsOnly} onChange={(e) => setGapsOnly(e.target.checked)} />
          Gaps only
        </label>
        <label>
          Sort
          <select className="detail-input pie-filter-select" value={sort} onChange={(e) => setSort(e.target.value as (typeof SORTS)[number])}>
            {SORTS.map((s) => (
              <option key={s}>{s}</option>
            ))}
          </select>
        </label>
      </div>
      {!grouped && <p className="muted">Gaps show once projects are under a baseline. Use Baselines to set them.</p>}
      <div className="pie-results">
        <table className="pie-table pie-map pie-cov-table" aria-label="Coverage">
          <thead>
            <tr>
              <th scope="col" rowSpan={span}>
                Linked set
              </th>
              {grouped
                ? groups.map((g) => (
                    <th key={g.baseline ?? ""} scope="colgroup" colSpan={g.projects.length}>
                      {g.baseline === null ? "No baseline" : `Baseline ${name(g.baseline)}`}
                    </th>
                  ))
                : columns.map((c) => (
                    <th key={c.project} scope="col">
                      {name(c.project)}
                    </th>
                  ))}
              <th scope="col" rowSpan={span}>
                Covered
              </th>
              <th scope="col" rowSpan={span}>
                State
              </th>
            </tr>
            {grouped && (
              <tr>
                {columns.map((c) => (
                  <th key={c.project} scope="col">
                    {name(c.project)}
                  </th>
                ))}
              </tr>
            )}
            {grouped && (
              <tr className="pie-cov-share">
                <th scope="row">Coverage</th>
                {columns.map((c) => (
                  <td key={c.project}>
                    {c.share !== null && (
                      <>
                        {percent(c.share)}
                        <span className="pie-cov-meter" style={{ "--share": percent(c.share) } as CSSProperties} aria-hidden="true" />
                      </>
                    )}
                  </td>
                ))}
                <td />
                <td />
              </tr>
            )}
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.set.keys[0]}>
                <th scope="row" className="mono">
                  {r.set.keys[0]}
                </th>
                {r.cells.map((c, i) => (
                  <td key={columns[i].project} className="pie-cov-cell">
                    {c === "gap" ? (
                      <span className="pie-cov-gap">gap</span>
                    ) : c === "none" ? (
                      <span className="muted">·</span>
                    ) : (
                      c.keys.map((k) => {
                        const issue = issues.get(k);
                        return (
                          <button
                            key={k}
                            type="button"
                            className={`pie-cov-key mono ${issue?.statusCategory === "done" ? "pie-cov-done" : "pie-cov-open"}`}
                            aria-label={`${k} ${issue?.status ?? ""}`.trim()}
                            title={issue?.status}
                            onClick={() => onOpen(k)}
                          >
                            {k}
                          </button>
                        );
                      })
                    )}
                  </td>
                ))}
                <td>{`${r.covered} of ${r.of}`}</td>
                <td className={`pie-cov-state-${r.state}`}>{label(r.state)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <Pager label="Coverage" {...pager} />
    </>
  );
}
