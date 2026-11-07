import { useEffect, useState } from "react";
import { Modal, useConfirm, usePrompt } from "@agile-suite/core";
import { AddCaseProjects, DeleteProjectGroup, JiraProjects, ProjectGroups, SaveProjectGroup } from "../wailsjs/go/main/App";
import type { cases, jira } from "../wailsjs/go/models";
import { errorText } from "./AddProfileDialog";
import { projectLabel } from "./linked";

// An account can see hundreds of projects; past this many matches the list
// asks for a narrower search rather than drawing them all.
const SHOWN = 50;

// ProjectPicker adds projects to a case: searched from Jira's project list by
// key or name, or taken from a named group. The analyst can save what is
// chosen as a new group.
export function ProjectPicker({
  profileId,
  caseId,
  onClose,
  onAdded,
}: {
  profileId: string;
  caseId: number;
  onClose: () => void;
  onAdded: (d: cases.Detail) => void;
}) {
  const { confirm } = useConfirm();
  const { prompt } = usePrompt();
  const [projects, setProjects] = useState<jira.Project[] | null>(null);
  const [groups, setGroups] = useState<cases.Group[]>([]);
  const [search, setSearch] = useState("");
  const [chosen, setChosen] = useState<Set<string>>(new Set());
  const [error, setError] = useState("");

  useEffect(() => {
    let current = true;
    JiraProjects(profileId).then(
      (l) => current && setProjects([...l].sort((a, b) => a.key.localeCompare(b.key))),
      (e) => current && setError(errorText(e)),
    );
    ProjectGroups(profileId).then(
      (g) => current && setGroups(g),
      (e) => current && setError(errorText(e)),
    );
    return () => {
      current = false;
    };
  }, [profileId]);

  const needle = search.trim().toLowerCase();
  const matches = (projects ?? []).filter((p) => !needle || p.key.toLowerCase().includes(needle) || p.name.toLowerCase().includes(needle));
  const keys = [...chosen].sort();

  function toggle(key: string, on: boolean) {
    const next = new Set(chosen);
    if (on) next.add(key);
    else next.delete(key);
    setChosen(next);
  }

  async function run(action: () => Promise<void>) {
    setError("");
    try {
      await action();
    } catch (e) {
      setError(errorText(e));
    }
  }

  async function saveGroup() {
    const name = await prompt({ title: "Save as project group", placeholder: "Group name", submitLabel: "Save" });
    if (name === null) return;
    await run(async () => {
      const g = await SaveProjectGroup(profileId, name, keys);
      setGroups([...groups, g].sort((a, b) => a.name.localeCompare(b.name)));
    });
  }

  async function deleteGroup(g: cases.Group) {
    const yes = await confirm({ title: `Delete ${g.name}?`, message: "Cases keep their projects.", confirmLabel: "Delete", danger: true });
    if (yes) await run(async () => {
      await DeleteProjectGroup(profileId, g.id);
      setGroups(groups.filter((x) => x.id !== g.id));
    });
  }

  return (
    <Modal onClose={onClose} labelledBy="pie-project-picker">
      <div className="profile-form">
        <h2 id="pie-project-picker">Add projects</h2>
        {groups.length > 0 && (
          <ul className="pie-filter-list" aria-label="Project groups">
            {groups.map((g) => (
              <li key={g.id} className="pie-group">
                <button
                  type="button"
                  className="pie-map-key"
                  aria-label={`Choose ${g.name} (${g.projects.length} projects)`}
                  onClick={() => setChosen(new Set([...chosen, ...g.projects]))}
                >
                  {g.name} <span className="muted">{g.projects.join(", ")}</span>
                </button>
                <button type="button" className="btn btn-ghost" aria-label={`Delete group ${g.name}`} onClick={() => void deleteGroup(g)}>
                  Delete
                </button>
              </li>
            ))}
          </ul>
        )}
        <label>
          Search projects
          <input value={search} placeholder="Key or name" onChange={(e) => setSearch(e.target.value)} />
        </label>
        {projects === null && !error && <p className="muted">Reading Jira's project list...</p>}
        <ul className="pie-filter-list" aria-label="Projects">
          {matches.slice(0, SHOWN).map((p) => (
            <li key={p.key}>
              <label className="pie-check">
                <input
                  type="checkbox"
                  aria-label={projectLabel(p.key, p.name)}
                  checked={chosen.has(p.key)}
                  onChange={(e) => toggle(p.key, e.target.checked)}
                />
                {projectLabel(p.key, p.name)}
              </label>
            </li>
          ))}
        </ul>
        {matches.length > SHOWN && <p className="muted">{`Showing ${SHOWN} of ${matches.length}. Narrow the search to see the rest.`}</p>}
        {keys.length > 0 && <p className="muted">{`Chosen: ${keys.join(", ")}`}</p>}
        {error && <p className="error-text">{error}</p>}
        <div className="form-actions form-actions-end">
          <button type="button" className="btn btn-ghost" disabled={keys.length === 0} onClick={() => void saveGroup()}>
            Save as group...
          </button>
          <button type="button" className="btn" onClick={onClose}>
            Cancel
          </button>
          <button
            type="button"
            className="btn btn-primary"
            disabled={keys.length === 0}
            onClick={() => void run(async () => onAdded(await AddCaseProjects(profileId, caseId, keys)))}
          >
            {keys.length === 1 ? "Add 1 project" : `Add ${keys.length} projects`}
          </button>
        </div>
      </div>
    </Modal>
  );
}
