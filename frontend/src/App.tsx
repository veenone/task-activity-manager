import { useCallback, useEffect, useState } from "react";
import "./App.css";
import {
  Health,
  ListProfiles,
  SyncProfile,
  GetSyncState,
  ListFolders,
  EventsOn,
  errMsg,
} from "./api";
import type {
  HealthInfo,
  Profile,
  SyncState,
  SyncProgress,
  Folder,
} from "./api";
import { ProfileForm } from "./components/ProfileForm";
import { TestTable } from "./components/TestTable";
import { TestDetail } from "./components/TestDetail";
import { FolderTree } from "./components/FolderTree";

function App() {
  const [health, setHealth] = useState<HealthInfo | null>(null);

  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [activeId, setActiveId] = useState<string>("");
  const [loadingProfiles, setLoadingProfiles] = useState(false);
  const [showForm, setShowForm] = useState(false);

  const [syncState, setSyncState] = useState<SyncState | null>(null);
  const [progress, setProgress] = useState<SyncProgress | null>(null);
  const [syncError, setSyncError] = useState("");
  const [syncing, setSyncing] = useState(false);

  const [folders, setFolders] = useState<Folder[]>([]);
  const [selectedFolder, setSelectedFolder] = useState<string>("");

  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);

  // First: check whether the backend started up cleanly. If not, render a
  // diagnostic screen so the user sees the actual failure instead of a
  // blank window or a cryptic nil-pointer panic.
  useEffect(() => {
    Health()
      .then(setHealth)
      .catch((e) =>
        setHealth({
          ok: false,
          error: `Health check itself failed: ${errMsg(e)}`,
          dbPath: "",
          logPath: "",
        }),
      );
  }, []);

  // Load profiles once the backend reports healthy.
  useEffect(() => {
    if (!health || !health.ok) return;
    setLoadingProfiles(true);
    ListProfiles()
      .then((ps) => {
        setProfiles(ps);
        if (ps.length > 0) setActiveId(ps[0].id);
      })
      .catch((e) => console.error("load profiles:", errMsg(e)))
      .finally(() => setLoadingProfiles(false));
  }, [health]);

  // Subscribe to sync progress events for the lifetime of the app.
  useEffect(() => {
    return EventsOn("sync:progress", (p: SyncProgress) => setProgress(p));
  }, []);

  // Refresh the sync summary and folder tree when the active profile changes
  // or a sync finishes.
  const loadProfileData = useCallback(() => {
    if (!activeId) {
      setSyncState(null);
      setFolders([]);
      return;
    }
    GetSyncState(activeId)
      .then(setSyncState)
      .catch((e) => console.error("sync state:", errMsg(e)));
    ListFolders(activeId)
      .then(setFolders)
      .catch((e) => console.error("list folders:", errMsg(e)));
  }, [activeId]);

  useEffect(() => {
    loadProfileData();
  }, [loadProfileData, refreshKey]);

  // Clear folder selection when the profile changes.
  useEffect(() => {
    setSelectedFolder("");
  }, [activeId]);

  async function runSync() {
    if (!activeId || syncing) return;
    setSyncing(true);
    setSyncError("");
    setProgress({ fetched: 0, total: 0, done: false });
    try {
      await SyncProfile(activeId);
      setRefreshKey((k) => k + 1);
    } catch (e) {
      setSyncError(errMsg(e));
    } finally {
      setSyncing(false);
      setProgress(null);
    }
  }

  function handleCreated(p: Profile) {
    setProfiles((prev) => [...prev, p]);
    setActiveId(p.id);
    setShowForm(false);
    setSelectedKey(null);
  }

  const activeProfile = profiles.find((p) => p.id === activeId);
  const isDemo =
    !!activeProfile &&
    /^(demo$|demo:|mock:)/i.test(activeProfile.jiraUrl.trim());

  // Health check hasn't returned yet.
  if (!health) {
    return <div className="centered muted">Loading…</div>;
  }

  // Startup failed — show the actual error and the log path so the user can
  // diagnose without a console window.
  if (!health.ok) {
    return (
      <div className="centered">
        <div className="onboard">
          <h2>Backend failed to start</h2>
          <pre className="backend-error">{health.error || "(no error message reported)"}</pre>
          {health.dbPath && (
            <p className="muted">
              Database path:{" "}
              <code>{health.dbPath}</code>
            </p>
          )}
          {health.logPath && (
            <p className="muted">
              Full log:{" "}
              <code>{health.logPath}</code>
            </p>
          )}
          <p className="muted">
            Try removing the database file and relaunching, or check the log
            for a more detailed error.
          </p>
        </div>
      </div>
    );
  }

  if (loadingProfiles) {
    return <div className="centered muted">Loading…</div>;
  }

  // First-run onboarding — no profiles configured yet (FR-12.1).
  if (profiles.length === 0) {
    return (
      <div className="centered">
        <div className="onboard">
          <h1>Xray Test Manager</h1>
          <p className="muted">
            Connect to your Jira Data Center project to get started.
          </p>
          <ProfileForm onCreated={handleCreated} />
        </div>
      </div>
    );
  }

  return (
    <div className="app">
      <header className="topbar">
        <span className="brand">Xray Test Manager</span>
        {isDemo && <span className="demo-chip">DEMO</span>}
        <select
          className="profile-select"
          value={activeId}
          onChange={(e) => {
            setActiveId(e.target.value);
            setSelectedKey(null);
          }}
        >
          {profiles.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name} ({p.projectKey})
            </option>
          ))}
        </select>
        <button className="btn" onClick={() => setShowForm(true)}>
          + Profile
        </button>

        <div className="spacer" />

        {progress && !progress.done && <SyncBar progress={progress} />}
        {syncError && (
          <span className="error-text">Sync failed: {syncError}</span>
        )}
        {!progress && !syncError && syncState && (
          <span className="muted sync-info">
            {syncState.testCount.toLocaleString()} tests
            {syncState.lastSyncedAt
              ? ` · synced ${new Date(syncState.lastSyncedAt).toLocaleString()}`
              : " · never synced"}
          </span>
        )}
        <button
          className="btn btn-primary"
          onClick={runSync}
          disabled={syncing}
        >
          {syncing ? "Syncing…" : "Sync"}
        </button>
      </header>

      <main className="content">
        {folders.length > 0 && (
          <FolderTree
            folders={folders}
            selected={selectedFolder}
            onSelect={(id) => {
              setSelectedFolder(id);
              setSelectedKey(null);
            }}
          />
        )}
        <TestTable
          profileId={activeId}
          folderId={selectedFolder}
          refreshKey={refreshKey}
          selectedKey={selectedKey}
          onSelect={setSelectedKey}
        />
        {selectedKey && (
          <TestDetail
            profileId={activeId}
            testKey={selectedKey}
            onClose={() => setSelectedKey(null)}
          />
        )}
      </main>

      {showForm && (
        <div className="modal-overlay" onClick={() => setShowForm(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <ProfileForm
              onCreated={handleCreated}
              onCancel={() => setShowForm(false)}
            />
          </div>
        </div>
      )}
    </div>
  );
}

function SyncBar({ progress }: { progress: SyncProgress }) {
  const pct =
    progress.total > 0
      ? Math.round((progress.fetched / progress.total) * 100)
      : 0;
  return (
    <div className="syncbar">
      <div className="syncbar-track">
        <div className="syncbar-fill" style={{ width: `${pct}%` }} />
      </div>
      <span className="muted">
        {progress.fetched.toLocaleString()} /{" "}
        {progress.total > 0 ? progress.total.toLocaleString() : "…"}
      </span>
    </div>
  );
}

export default App;
