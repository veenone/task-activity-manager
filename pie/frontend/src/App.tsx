import { useEffect, useState } from "react";
import { ViewTabs, useConfirm, usePrompt } from "@agile-suite/core";
import {
  DeleteProfile,
  DeleteQuery,
  LastRun,
  ListProfiles,
  ListQueries,
  Fields,
  RunQuery,
  SaveQuery,
  StartupError,
} from "../wailsjs/go/main/App";
import type { profile, query, saved } from "../wailsjs/go/models";
import { AddProfileDialog, errorText, isDemo } from "./AddProfileDialog";
import { CasesPage } from "./CasesPage";
import { FieldsDialog } from "./FieldsDialog";
import { JiraFilterPicker } from "./JiraFilterPicker";
import { ProjectMap } from "./ProjectMap";
import { ResultsTable } from "./ResultsTable";
import { RunNotes } from "./RunNotes";
import { Statistics } from "./Statistics";
import { TracePage } from "./TracePage";
import { useCases } from "./useCases";

type View = "results" | "trace" | "map" | "stats" | "cases";
const VIEWS = [
  { id: "results", label: "Results" },
  { id: "trace", label: "Trace" },
  { id: "map", label: "Project map" },
  { id: "stats", label: "Statistics" },
  { id: "cases", label: "Cases" },
] as const;

function countText(res: query.Result, matched: number): string {
  if (res.truncated) return `Showing the first ${matched} of ${res.total} issues`;
  return res.total === 1 ? "1 issue" : `${res.total} issues`;
}

// runAtText gives a stored run's time in the analyst's own time zone.
function runAtText(iso: string): string {
  const d = new Date(iso);
  const two = (n: number) => String(n).padStart(2, "0");
  return `Last run ${d.getFullYear()}-${two(d.getMonth() + 1)}-${two(d.getDate())} ${two(d.getHours())}:${two(d.getMinutes())}`;
}

export default function App() {
  const { confirm } = useConfirm();
  const { prompt } = usePrompt();
  const [startupError, setStartupError] = useState("");
  const [profiles, setProfiles] = useState<profile.Profile[] | null>(null);
  const [profileId, setProfileId] = useState("");
  const [adding, setAdding] = useState(false);
  const [jql, setJql] = useState("");
  const [running, setRunning] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<query.Result | null>(null);
  const [queries, setQueries] = useState<saved.Query[]>([]);
  // 0 is JQL that has not been saved; its runs are not stored.
  const [queryId, setQueryId] = useState(0);
  const [runAt, setRunAt] = useState("");
  const [change, setChange] = useState<{ added: string[]; dropped: string[] } | null>(null);
  const [view, setView] = useState<View>("results");
  const cases = useCases(profileId, setError);
  // null until the stored names arrive, so the dialog cannot save blanks over them.
  const [fields, setFields] = useState<saved.Fields | null>(null);
  const [editingFields, setEditingFields] = useState(false);
  // The key whose linked set Trace opens; the map sets it when a key is chosen.
  const [focus, setFocus] = useState<string | null>(null);

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

  useEffect(() => {
    setQueryId(0);
    setRunAt("");
    setChange(null);
    setQueries([]);
    setFields(null);
    if (!profileId) return;
    // A slow answer for a profile no longer selected must not land.
    let current = true;
    ListQueries(profileId).then(
      (list) => current && setQueries(list),
      (e) => current && setError(errorText(e)),
    );
    Fields(profileId).then(
      (f) => current && setFields(f),
      (e) => current && setError(errorText(e)),
    );
    return () => {
      current = false;
    };
  }, [profileId]);

  function showRun(r: saved.Run) {
    setResult(r.result);
    setRunAt(r.runAt);
    setChange(r.added && r.dropped ? { added: r.added, dropped: r.dropped } : null);
    setFocus(null);
  }

  async function openQuery(id: number) {
    setQueryId(id);
    setError("");
    setChange(null);
    if (id === 0) return;
    setJql(queries.find((q) => q.id === id)?.jql ?? "");
    // The rows on screen belong to whatever was open before.
    setResult(null);
    setRunAt("");
    try {
      const r = await LastRun(profileId, id);
      if (r.runAt) showRun(r);
    } catch (e) {
      setError(errorText(e));
    }
  }

  async function saveQuery() {
    const name = await prompt({ title: "Save query as", placeholder: "Name", submitLabel: "Save" });
    if (name === null) return;
    try {
      const q = await SaveQuery(profileId, name, jql);
      setQueries(await ListQueries(profileId));
      setQueryId(q.id);
      setRunAt("");
      setChange(null);
    } catch (e) {
      setError(errorText(e));
    }
  }

  async function deleteQuery() {
    const target = queries.find((q) => q.id === queryId);
    if (!target) return;
    const yes = await confirm({
      title: `Delete ${target.name}?`,
      message: "This removes the saved query and its last run from this computer.",
      confirmLabel: "Delete",
      danger: true,
    });
    if (!yes) return;
    try {
      await DeleteQuery(profileId, target.id);
      setQueries(await ListQueries(profileId));
      setQueryId(0);
      setRunAt("");
      setChange(null);
    } catch (e) {
      setError(errorText(e));
    }
  }

  async function run() {
    setRunning(true);
    setError("");
    try {
      let r = await RunQuery(profileId, queryId, jql, false);
      if (r.result.needsConfirm) {
        const yes = await confirm({
          title: "Fetch a large result?",
          message: `This query matches ${r.result.total} issues. PIE fetches at most 5000 of them.`,
          confirmLabel: "Fetch",
          danger: false,
        });
        if (!yes) return; // nothing was fetched, so the last rows stay
        r = await RunQuery(profileId, queryId, jql, true);
      }
      showRun(r);
      // A run stores the JQL it ran on the saved query.
      if (queryId !== 0) setQueries(await ListQueries(profileId));
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
  const selected = profiles?.find((p) => p.id === profileId);
  // Issues reached only through a link belong to Trace and the map, not to the
  // rows the analyst asked Jira for.
  const matched = result?.issues.filter((i) => i.matched) ?? [];

  return (
    <div className="app">
      <header className="topbar">
        <span className="brand">Project Issue Explorer</span>
        {selected && isDemo(selected.jiraUrl) && <span className="demo-chip">DEMO</span>}
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
              <button type="button" className="btn btn-ghost" disabled={running || fields === null} onClick={() => setEditingFields(true)}>
                Jira fields
              </button>
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
            <div className="pie-saved">
              <label className="field-label" htmlFor="pie-saved-query">
                Saved query
              </label>
              <select
                id="pie-saved-query"
                className="detail-input"
                value={queryId}
                disabled={running}
                onChange={(e) => void openQuery(Number(e.target.value))}
              >
                <option value={0}>Unsaved query</option>
                {queries.map((q) => (
                  <option key={q.id} value={q.id}>
                    {q.name}
                  </option>
                ))}
              </select>
              <button
                type="button"
                className="btn btn-ghost"
                disabled={running || jql.trim() === ""}
                onClick={() => void saveQuery()}
              >
                Save as...
              </button>
              {queryId !== 0 && (
                <button type="button" className="btn btn-ghost" disabled={running} onClick={() => void deleteQuery()}>
                  Delete query
                </button>
              )}
              <JiraFilterPicker
                profileId={profileId}
                disabled={running}
                onPick={(id) => {
                  // A filter reference follows the filter as its owner edits it.
                  setJql(`filter = ${id}`);
                  void openQuery(0);
                }}
              />
            </div>
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
              {!running && result && <span className="muted">{countText(result, matched.length)}</span>}
              {!running && queryId !== 0 && runAt && <span className="muted">{runAtText(runAt)}</span>}
            </div>
          </form>
        )}

        <RunNotes change={change} error={error} result={result} caseKeys={cases.keys} />

        {hasProfile && <ViewTabs views={VIEWS} current={view} onSelect={setView} />}
        {/* Cases are PIE's own records; every other view needs a run. */}
        {view === "cases" && <CasesPage profileId={profileId} cases={cases.list} result={result} onChanged={cases.reload} />}
        {!result && view !== "cases" && hasProfile && <p className="muted">Run a query to see this view.</p>}

        {result && view === "trace" && <TracePage result={result} focus={focus} onFocus={setFocus} />}
        {result && view === "map" && (
          <ProjectMap
            result={result}
            onOpen={(key) => {
              setFocus(key);
              setView("trace");
            }}
          />
        )}

        {result && view === "stats" && <Statistics result={result} productField={fields?.productBase ?? ""} />}
        {result && view === "results" && matched.length > 0 && (
          <ResultsTable issues={matched} caseKeys={cases.keys} onOpenCases={cases.open} />
        )}
      </main>

      {editingFields && fields && (
        <FieldsDialog
          profileId={profileId}
          fields={fields}
          onClose={() => setEditingFields(false)}
          onSaved={(f) => {
            setEditingFields(false);
            setFields(f);
          }}
        />
      )}
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
