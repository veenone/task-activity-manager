import { useState } from "react";
import { StatusBadge } from "@agile-suite/core";
import type { query } from "../wailsjs/go/models";
import { OutOfStep, ageText, allProjects, byKey, tone } from "./linked";

// TracePage lists the linked sets and shows one of them project by project.
// focus is the key whose set is open; with none, the first set shown is.
export function TracePage({
  result,
  focus,
  onFocus,
}: {
  result: query.Result;
  focus: string | null;
  onFocus: (key: string) => void;
}) {
  const [project, setProject] = useState("");
  const [onlyOutOfStep, setOnlyOutOfStep] = useState(false);
  const issues = byKey(result);
  const sets = result.sets.filter(
    (s) => (project === "" || s.projects.includes(project)) && (!onlyOutOfStep || s.outOfStep),
  );
  const open = sets.find((s) => focus !== null && s.keys.includes(focus)) ?? sets[0];

  return (
    <div className="pie-linked pie-trace">
      <div className="pie-trace-side">
        <div className="pie-filters">
          <label>
            Project
            <select value={project} onChange={(e) => setProject(e.target.value)}>
              <option value="">All projects</option>
              {allProjects(result.sets).map((p) => (
                <option key={p} value={p}>
                  {p}
                </option>
              ))}
            </select>
          </label>
          <label className="pie-check">
            <input type="checkbox" checked={onlyOutOfStep} onChange={(e) => setOnlyOutOfStep(e.target.checked)} />
            Out of step only
          </label>
        </div>
        <ul className="pie-set-list" aria-label="Linked sets">
          {sets.map((s) => (
            <li key={s.keys[0]}>
              <button
                type="button"
                className={`pie-set${s === open ? " pie-set-open" : ""}`}
                aria-current={s === open ? "true" : undefined}
                onClick={() => onFocus(s.keys[0])}
              >
                <span className="mono">{s.keys[0]}</span>
                <span className="muted">
                  {s.keys.length} issues in {s.projects.length} projects
                </span>
                <OutOfStep outOfStep={s.outOfStep} />
              </button>
            </li>
          ))}
        </ul>
        {sets.length === 0 && <p className="muted">No linked sets to show.</p>}
      </div>

      {open && (
        <section className="pie-trace-detail" aria-labelledby="pie-trace-heading">
          <h2 id="pie-trace-heading">Linked set of {open.keys[0]}</h2>
          {open.projects.map((p) => (
            <div key={p} role="group" aria-label={`Project ${p}`} className="pie-trace-project">
              <h3>{p}</h3>
              {open.keys
                .map((k) => issues.get(k))
                .filter((i): i is query.Issue => i?.project === p)
                .map((i) => (
                  <div key={i.key} className={`pie-trace-issue${i.key === focus ? " pie-trace-focus" : ""}`}>
                    <div className="pie-trace-line">
                      <span className="mono">{i.key}</span>
                      <StatusBadge tone={tone(i.statusCategory)} label={i.status} />
                      <span className="muted">{ageText(i.created)}</span>
                      {!i.matched && <span className="pie-not-matched">Not in the query</span>}
                    </div>
                    <div>{i.summary}</div>
                    <ul className="pie-trace-links">
                      {(i.links ?? [])
                        .filter((l) => open.keys.includes(l.key) && issues.get(l.key)?.project !== p)
                        .map((l) => (
                          <li key={`${l.words} ${l.key}`}>
                            {l.words} {l.key}
                          </li>
                        ))}
                    </ul>
                  </div>
                ))}
            </div>
          ))}
        </section>
      )}
    </div>
  );
}
