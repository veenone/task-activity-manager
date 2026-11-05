import { useEffect, useState } from "react";
import { useConfirm } from "@agile-suite/core";
import { ListProfiles, DeleteProfile, RunQuery, StartupError } from "../wailsjs/go/main/App";
import type { profile, query } from "../wailsjs/go/models";
import { AddProfileDialog, errorText } from "./AddProfileDialog";

function countText(res: query.Result): string {
  if (res.truncated) return `Showing the first ${res.issues.length} of ${res.total} issues`;
  return res.total === 1 ? "1 issue" : `${res.total} issues`;
}

export default function App() {
  const { confirm } = useConfirm();
  const [startupError, setStartupError] = useState("");
  const [profiles, setProfiles] = useState<profile.Profile[] | null>(null);
  const [profileId, setProfileId] = useState("");
  const [adding, setAdding] = useState(false);
  const [jql, setJql] = useState("");
  const [running, setRunning] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<query.Result | null>(null);

  async function loadProfiles(select?: string) {
    const list = await ListProfiles();
    setProfiles(list);
    setProfileId((current) => select ?? (list.some((p) => p.id === current) ? current : (list[0]?.id ?? "")));
  }

  useEffect(() => {
    StartupError()
      .then((problem) => {
        setStartupError(problem);
        return problem ? undefined : loadProfiles();
      })
      .catch((e) => setError(errorText(e)));
  }, []);

  async function run() {
    setRunning(true);
    setError("");
    try {
      setResult(await RunQuery(profileId, jql));
    } catch (e) {
      // The last good rows stay: a refused query says nothing about them.
      setError(errorText(e));
    } finally {
      setRunning(false);
    }
  }

  async function deleteProfile() {
    const target = profiles?.find((p) => p.id === profileId);
    if (!target) return;
    const yes = await confirm({
      title: `Delete ${target.name}?`,
      message: "This removes the profile and its saved token from this computer.",
      confirmLabel: "Delete",
      danger: true,
    });
    if (!yes) return;
    try {
      await DeleteProfile(target.id);
      await loadProfiles();
      setResult(null);
    } catch (e) {
      setError(errorText(e));
    }
  }

  if (startupError) {
    return (
      <div className="pie-main">
        <p role="alert" className="error-text">
          Project Issue Explorer could not start: {startupError}
        </p>
      </div>
    );
  }

  const hasProfile = profileId !== "";

  return (
    <div className="app">
      <header className="topbar">
        <span className="brand">Project Issue Explorer</span>
        <div className="pie-topbar-actions">
          {hasProfile && (
            <>
              <label className="sr-only" htmlFor="pie-profile">
                Profile
              </label>
              <select
                id="pie-profile"
                className="profile-select"
                value={profileId}
                disabled={running}
                onChange={(e) => {
                  setProfileId(e.target.value);
                  setResult(null);
                  setError("");
                }}
              >
                {profiles?.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </select>
              <button type="button" className="btn btn-ghost" disabled={running} onClick={() => void deleteProfile()}>
                Delete profile
              </button>
            </>
          )}
          <button type="button" className="btn btn-ghost" onClick={() => setAdding(true)}>
            Add profile
          </button>
        </div>
      </header>

      <main className="pie-main">
        {profiles !== null && !hasProfile && <p className="muted">Add a profile to run a query.</p>}
        {hasProfile && (
          <form
            className="pie-query"
            onSubmit={(e) => {
              e.preventDefault();
              void run();
            }}
          >
            <label className="field-label" htmlFor="pie-jql">
              JQL
            </label>
            <textarea
              id="pie-jql"
              className="detail-input mono"
              rows={4}
              spellCheck={false}
              value={jql}
              onChange={(e) => setJql(e.target.value)}
            />
            <div className="form-actions">
              <button type="submit" className="btn btn-primary" disabled={running || jql.trim() === ""}>
                Run
              </button>
              {running && <span className="muted">Running...</span>}
              {!running && result && <span className="muted">{countText(result)}</span>}
            </div>
          </form>
        )}

        {error && (
          <p role="alert" className="error-text">
            {error}
          </p>
        )}

        {result && result.issues.length > 0 && (
          <div className="pie-results">
            <table className="pie-table" aria-label="Query results">
              <thead>
                <tr>
                  <th scope="col">Key</th>
                  <th scope="col">Project</th>
                  <th scope="col">Type</th>
                  <th scope="col">Summary</th>
                  <th scope="col">Status</th>
                  <th scope="col">Priority</th>
                  <th scope="col">Updated</th>
                </tr>
              </thead>
              <tbody>
                {result.issues.map((issue) => (
                  <tr key={issue.key}>
                    <td className="mono">{issue.key}</td>
                    <td>{issue.project}</td>
                    <td>{issue.type}</td>
                    <td>{issue.summary}</td>
                    <td>{issue.status}</td>
                    <td>{issue.priority}</td>
                    <td>{issue.updated.slice(0, 10)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </main>

      {adding && (
        <AddProfileDialog
          onClose={() => setAdding(false)}
          onSaved={(saved) => {
            setAdding(false);
            void loadProfiles(saved.id);
          }}
        />
      )}
    </div>
  );
}
