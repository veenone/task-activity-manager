import { useState } from "react";
import { StatusBadge } from "@agile-suite/core";
import type { query } from "../wailsjs/go/models";
import { OutOfStep, ageText, allProjects, byKey, projectLabel, projectNames, tone } from "./linked";
import { ExportButton } from "./ExportButton";
import { setSheet } from "./exportSheets";
import { ProjectInput } from "./ProjectInput";
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
  const [keyText, setKeyText] = useState("");
  const keyHit = (s: query.Set) => s.keys.some((k) => k.toLowerCase().includes(keyText.trim().toLowerCase()));
  const issues = byKey(result);
  const names = projectNames(result);
  const shownBy = (p: string, only: boolean) =>
    result.sets.filter((s) => (p === "" || s.projects.includes(p)) && (!only || s.outOfStep) && keyHit(s));
  // The open set is the focused one if it is shown, else the first shown.
  const openIndex = (list: query.Set[]) => Math.max(0, list.findIndex((s) => focus !== null && s.keys.includes(focus)));
  const sets = shownBy(project, onlyOutOfStep);
  const open = sets[openIndex(sets)];
  // Trace mounts when the map opens a key, so the list starts on that key's page.
  const { rows, pager } = usePaged(sets, openIndex(sets));

  // A filter change turns the list to the page holding the set left open.
  function refilter(p: string, only: boolean) {
    setProject(p);
    setOnlyOutOfStep(only);
    pager.onPage(Math.floor(openIndex(shownBy(p, only)) / pager.pageSize));
  }

  return (
    <div className="pie-linked pie-trace">
      <div className="pie-trace-side">
        <div className="pie-filters">
          <ProjectInput
            label="Project"
            projects={allProjects(result.sets)}
            projectLabel={(p) => projectLabel(p, names.get(p))}
            value={project}
            onChange={(p) => refilter(p, onlyOutOfStep)}
          />
          <label>
            Key
            <input
              className="detail-input pie-filter-select"
              type="search"
              value={keyText}
              placeholder="Part of a key"
              onChange={(e) => {
                setKeyText(e.target.value);
                pager.onPage(0);
              }}
            />
          </label>
          <label className="pie-check">
            <input type="checkbox" checked={onlyOutOfStep} onChange={(e) => refilter(project, e.target.checked)} />
            Out of step only
          </label>
          <ExportButton name="pie-trace" sheets={() => [setSheet(sets, issues, (p) => projectLabel(p, names.get(p)))]} />
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
  // A key opened from the map may sit in a project past the first page.
  const focused = set.projects.indexOf(issues.get(focus ?? "")?.project ?? "");
  const { rows, pager } = usePaged(set.projects, Math.max(0, focused), GROUP_SIZES);
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
