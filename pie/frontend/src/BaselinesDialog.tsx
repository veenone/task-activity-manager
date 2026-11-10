import { useEffect, useState } from "react";
import { Modal, useConfirm } from "@agile-suite/core";
import { AddBaseline, AddDescendants, JiraProjects, RemoveBaseline, RemoveDescendant } from "../wailsjs/go/main/App";
import type { cases, jira } from "../wailsjs/go/models";
import { errorText } from "./ProfileForm";
import { Combo } from "./ProjectInput";
import { projectLabel } from "./linked";

// BaselinesDialog keeps the analyst's mapping of Jira projects: which are
// product baselines, and which projects were derived from each. Go refuses a
// project already in the mapping and says where it sits.
export function BaselinesDialog({
  profileId,
  baselines,
  onChange,
  onClose,
}: {
  profileId: string;
  baselines: cases.Baseline[];
  onChange: (b: cases.Baseline[]) => void;
  onClose: () => void;
}) {
  const { confirm } = useConfirm();
  const [list, setList] = useState(baselines);
  const [projects, setProjects] = useState<jira.Project[]>([]);
  const [newBase, setNewBase] = useState("");
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [error, setError] = useState("");

  useEffect(() => {
    let current = true;
    JiraProjects(profileId).then(
      (l) => current && setProjects([...l].sort((a, b) => a.key.localeCompare(b.key))),
      (e) => current && setError(errorText(e)),
    );
    return () => {
      current = false;
    };
  }, [profileId]);

  const names = new Map(projects.map((p) => [p.key, p.name]));
  const label = (key: string) => projectLabel(key, names.get(key));
  const keyOf = (text: string) => text.trim().toUpperCase();
  // Every Jira project, offered in the boxes' suggestion lists (#454).
  const keys = projects.map((p) => p.key);
  const jiraLabel = (key: string) => projectLabel(key, projects.find((p) => p.key === key)?.name);

  async function change(action: () => Promise<cases.Baseline[]>): Promise<boolean> {
    setError("");
    try {
      const next = await action();
      setList(next);
      onChange(next);
      return true;
    } catch (e) {
      setError(errorText(e));
      return false;
    }
  }

  async function removeBaseline(key: string) {
    const yes = await confirm({
      title: `Remove ${label(key)}?`,
      message: "Its descendants leave the mapping with it. Jira does not change.",
      confirmLabel: "Remove",
      danger: true,
    });
    if (yes) await change(() => RemoveBaseline(profileId, key));
  }

  return (
    <Modal onClose={onClose} labelledBy="pie-baselines">
      <div className="profile-form">
        <h2 id="pie-baselines">Baselines</h2>
        <p className="muted">
          Mark the Jira projects that are product baselines and put the projects derived from each under it. A project sits under one
          baseline at most. The Project map groups its columns this way.
        </p>
        <form
          className="pie-baseline-add"
          onSubmit={(e) => {
            e.preventDefault();
            void change(() => AddBaseline(profileId, keyOf(newBase))).then((ok) => ok && setNewBase(""));
          }}
        >
          <label htmlFor="pie-new-baseline">New baseline</label>
            <Combo
              id="pie-new-baseline"
              name="New baseline"
              className="detail-input"
              text={newBase}
              onText={setNewBase}
              projects={keys}
              label={jiraLabel}
              onPick={setNewBase}
              placeholder=""
            />
          <button type="submit" className="btn btn-ghost" disabled={newBase.trim() === ""}>
            Add baseline
          </button>
        </form>
        {list.map((b) => (
          <fieldset key={b.key} className="pie-baseline" aria-label={label(b.key)}>
            <legend>
              {label(b.key)}
              <button type="button" className="btn btn-ghost" aria-label={`Remove ${label(b.key)}`} onClick={() => void removeBaseline(b.key)}>
                Remove baseline
              </button>
            </legend>
            <ul>
              {b.descendants.map((d) => (
                <li key={d}>
                  {label(d)}
                  <button
                    type="button"
                    className="btn btn-ghost"
                    aria-label={`Remove ${label(d)}`}
                    onClick={() => void change(() => RemoveDescendant(profileId, d))}
                  >
                    Remove
                  </button>
                </li>
              ))}
            </ul>
            <form
              className="pie-baseline-add"
              onSubmit={(e) => {
                e.preventDefault();
                const key = keyOf(drafts[b.key] ?? "");
                void change(() => AddDescendants(profileId, b.key, [key])).then((ok) => ok && setDrafts({ ...drafts, [b.key]: "" }));
              }}
            >
              <Combo
                name={`Add a descendant of ${label(b.key)}`}
                className="detail-input"
                text={drafts[b.key] ?? ""}
                onText={(t) => setDrafts({ ...drafts, [b.key]: t })}
                projects={keys}
                label={jiraLabel}
                onPick={(k) => setDrafts({ ...drafts, [b.key]: k })}
                placeholder=""
              />
              <button type="submit" className="btn btn-ghost" disabled={(drafts[b.key] ?? "").trim() === ""}>
                Add
              </button>
            </form>
          </fieldset>
        ))}
        {error && (
          <p role="alert" className="error-text">
            {error}
          </p>
        )}
        <div className="form-actions">
          <button type="button" className="btn btn-primary" onClick={onClose}>
            Done
          </button>
        </div>
      </div>
    </Modal>
  );
}
