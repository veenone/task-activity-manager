import { useCallback, useEffect, useState } from "react";
import "./App.css";
import {
  ListProfiles,
  SyncProfile,
  GetSyncState,
  EventsOn,
  errMsg,
} from "./api";
import type { Profile, SyncState, SyncProgress } from "./api";
import { ProfileForm } from "./components/ProfileForm";
import { TestTable } from "./components/TestTable";
import { TestDetail } from "./components/TestDetail";

function App() {
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [activeId, setActiveId] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);

  const [syncState, setSyncState] = useState<SyncState | null>(null);
  const [progress, setProgress] = useState<SyncProgress | null>(null);
  const [syncError, setSyncError] = useState("");
  const [syncing, setSyncing] = useState(false);

  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);

  // Load profiles once on startup.
  useEffect(() => {
    ListProfiles()
      .then((ps) => {
        setProfiles(ps);
        if (ps.length > 0) setActiveId(ps[0].id);
      })
      .catch((e) => console.error("load profiles:", errMsg(e)))
      .finally(() => setLoading(false));
  }, []);

  // Subscribe to sync progress events for the lifetime of the app.
  useEffect(() => {
    return EventsOn("sync:progress", (p: SyncProgress) => setProgress(p));
  }, []);

  // Refresh the sync summary when the active profile changes or a sync finishes.
  const loadSyncState = useCallback(() => {
    if (!activeId) {
      setSyncState(null);
      return;
    }
    GetSyncState(activeId)
      .then(setSyncState)
      .catch((e) => console.error("sync state:", errMsg(e)));
  }, [activeId]);

  useEffect(() => {
    loadSyncState();
  }, [loadSyncState, refreshKey]);

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

  if (loading) {
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
        <TestTable
          profileId={activeId}
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
