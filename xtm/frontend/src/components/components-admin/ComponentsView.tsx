import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useProfile } from "../../contexts/ProfileContext";
import { useComponentCounts, useProjectComponents } from "../../queries/components";
import { keys } from "../../queries/keys";
import {
  ComponentIssueCount,
  CreateComponent,
  DeleteComponent,
  UpdateComponent,
  errMsg,
} from "../../api";
import type { ComponentInput, ProjectComponent } from "../../api";
import { ComponentFormModal } from "./ComponentFormModal";
import { DeleteComponentModal } from "./DeleteComponentModal";

type Dialog =
  | { kind: "none" }
  | { kind: "new" }
  | { kind: "edit"; component: ProjectComponent }
  | { kind: "delete"; component: ProjectComponent; issueCount: number };

// ComponentsView lists the project's Jira components and creates, edits and
// deletes them. Writes go straight to Jira; afterwards the list, the local
// test counts and the rest of the profile's data reload.
export function ComponentsView({ onChanged }: { onChanged: () => void }) {
  const { activeId: profileId } = useProfile();
  const qc = useQueryClient();
  const list = useProjectComponents(profileId);
  const counts = useComponentCounts(profileId);
  const [dialog, setDialog] = useState<Dialog>({ kind: "none" });
  const [error, setError] = useState("");

  const components = list.data ?? [];

  async function reload() {
    await qc.invalidateQueries({ queryKey: keys.components(profileId) });
    onChanged();
  }

  async function write(run: () => Promise<unknown>) {
    try {
      await run();
      setDialog({ kind: "none" });
      setError("");
    } finally {
      await reload();
    }
  }

  async function openDelete(c: ProjectComponent) {
    setError("");
    try {
      const issueCount = await ComponentIssueCount(profileId, c.id);
      setDialog({ kind: "delete", component: c, issueCount });
    } catch (e) {
      setError(errMsg(e));
    }
  }

  const others = (c: ProjectComponent) => components.filter((o) => o.id !== c.id);

  return (
    <div className="components-view">
      <div className="components-head">
        <h2>Components</h2>
        <button className="btn btn-primary" onClick={() => setDialog({ kind: "new" })}>
          New component
        </button>
      </div>
      {(error || list.error) && <div className="error-text">{error || errMsg(list.error)}</div>}
      {list.isLoading ? (
        <p className="muted">Loading components…</p>
      ) : components.length === 0 ? (
        <p className="muted">This project has no components yet.</p>
      ) : (
        <table className="components-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Description</th>
              <th>Lead</th>
              <th>Tests</th>
              <th>
                <span className="sr-only">Actions</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {components.map((c) => (
              <tr key={c.id}>
                <td>{c.name}</td>
                <td>{c.description}</td>
                <td>{c.leadDisplayName || c.leadName}</td>
                <td>{counts.data?.get(c.name) ?? 0}</td>
                <td className="components-actions">
                  <button
                    className="btn btn-ghost"
                    aria-label={`Edit ${c.name}`}
                    onClick={() => setDialog({ kind: "edit", component: c })}
                  >
                    Edit
                  </button>
                  <button className="btn btn-ghost" aria-label={`Delete ${c.name}`} onClick={() => openDelete(c)}>
                    Delete
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {dialog.kind === "new" && (
        <ComponentFormModal
          takenNames={components.map((c) => c.name)}
          onSubmit={(input: ComponentInput) => write(() => CreateComponent(profileId, input))}
          onCancel={() => setDialog({ kind: "none" })}
        />
      )}
      {dialog.kind === "edit" && (
        <ComponentFormModal
          initial={dialog.component}
          takenNames={others(dialog.component).map((c) => c.name)}
          onSubmit={(input: ComponentInput) =>
            write(() => UpdateComponent(profileId, dialog.component.id, input))
          }
          onCancel={() => setDialog({ kind: "none" })}
        />
      )}
      {dialog.kind === "delete" && (
        <DeleteComponentModal
          component={dialog.component}
          others={others(dialog.component)}
          issueCount={dialog.issueCount}
          onConfirm={(moveTo) => write(() => DeleteComponent(profileId, dialog.component.id, moveTo))}
          onCancel={() => setDialog({ kind: "none" })}
        />
      )}
    </div>
  );
}
