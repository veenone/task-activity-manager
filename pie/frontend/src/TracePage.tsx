import { useState } from "react";
import { StatusBadge } from "@agile-suite/core";
import type { query } from "../wailsjs/go/models";
import { OutOfStep, ageText, allProjects, byKey, projectLabel, projectNames, tone } from "./linked";
import { Pager, usePaged } from "./Pager";

// A set rarely spans many projects, so its groups page in smaller steps.
const GROUP_SIZES = [5, 10, 25];

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
  const names = projectNames(result);
  const sets = result.sets.filter(
    (s) => (project === "" || s.projects.includes(project)) && (!onlyOutOfStep || s.outOfStep),
  );
  const open = sets.find((s) => focus !== null && s.keys.includes(focus)) ?? sets[0];
  // Trace mounts when the map opens a key, so the list starts on that key's page.
  const { rows, pager } = usePaged(sets, Math.max(0, sets.indexOf(open)));

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
                  {projectLabel(p, names.get(p))}
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
          {rows.map((s) => (
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
        <Pager label="Linked sets" {...pager} />
        {sets.length === 0 && <p className="muted">No linked sets to show.</p>}
      </div>

      {open && <SetDetail key={open.keys[0]} set={open} issues={issues} names={names} focus={focus} />}
    </div>
  );
}

// SetDetail is keyed by its set, so opening another set starts at its first page.
function SetDetail({
  set,
  issues,
  names,
  focus,
}: {
  set: query.Set;
  issues: Map<string, query.Issue>;
  names: Map<string, string>;
  focus: string | null;
}) {
  const { rows, pager } = usePaged(set.projects, 0, GROUP_SIZES);
  return (
    <section className="pie-trace-detail" aria-labelledby="pie-trace-heading">
      <h2 id="pie-trace-heading">Linked set of {set.keys[0]}</h2>
      {rows.map((p) => (
        <div key={p} role="group" aria-label={`Project ${projectLabel(p, names.get(p))}`} className="pie-trace-project">
          <h3>{projectLabel(p, names.get(p))}</h3>
          {set.keys
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
                    .filter((l) => set.keys.includes(l.key) && issues.get(l.key)?.project !== p)
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
      <Pager label="Projects in the set" {...pager} />
    </section>
  );
}
