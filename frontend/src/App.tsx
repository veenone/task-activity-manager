import { useCallback, useEffect, useMemo, useState } from "react";
import "./App.css";
import {
  Health,
  ListProfiles,
  SyncProfile,
  GetSyncState,
  ListFolders,
  ListPendingChanges,
  DiscardPendingChange,
  CommitPendingChanges,
  EventsOn,
  errMsg,
} from "./api";
import type {
  HealthInfo,
  Profile,
  SyncState,
  SyncProgress,
  Folder,
  PendingChange,
  CommitResult,
} from "./api";
import { ProfileForm } from "./components/ProfileForm";
import { TestTable } from "./components/TestTable";
import { TestDetail } from "./components/TestDetail";
import { FolderTree } from "./components/FolderTree";
import { PendingChangesModal } from "./components/PendingChangesModal";
import { BulkEditModal } from "./components/BulkEditModal";

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
  const [detailVersion, setDetailVersion] = useState(0);

  const [pendingChanges, setPendingChanges] = useState<PendingChange[]>([]);
  const [showPending, setShowPending] = useState(false);
  const [committing, setCommitting] = useState(false);
  const [lastCommitResult, setLastCommitResult] = useState<CommitResult | null>(
    null,
  );

  const [selectedSet, setSelectedSet] = useState<Set<string>>(new Set());
  const [showBulkEdit, setShowBulkEdit] = useState(false);

  // First: check whether the backend started up cleanly.
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

  // Pending changes grouped by test key — drives the dirty markers in the
  // grid and the per-field dot in the detail panel.
  const pendingByTestKey = useMemo(() => {
    const m = new Map<string, PendingChange[]>();
    for (const p of pendingChanges) {
      if (p.entityType !== "test_case") continue;
      const arr = m.get(p.entityKey);
      if (arr) arr.push(p);
      else m.set(p.entityKey, [p]);
    }
    return m;
  }, [pendingChanges]);

  const reloadPending = useCallback(() => {
    if (!activeId) {
      setPendingChanges([]);
      return;
    }
    ListPendingChanges(activeId)
      .then(setPendingChanges)
      .catch((e) => console.error("list pending:", errMsg(e)));
  }, [activeId]);

  useEffect(() => {
    reloadPending();
  }, [reloadPending, refreshKey]);

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

  // Clear folder + row selection when the profile changes.
  useEffect(() => {
    setSelectedFolder("");
    setSelectedSet(new Set());
  }, [activeId]);

  function toggleSelect(key: string) {
    setSelectedSet((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  function toggleSelectPage(keys: string[]) {
    setSelectedSet((prev) => {
      const allSelected = keys.every((k) => prev.has(k));
      const next = new Set(prev);
      if (allSelected) {
        for (const k of keys) next.delete(k);
      } else {
        for (const k of keys) next.add(k);
      }
      return next;
    });
  }

  async function runSync() {
    if (!activeId || syncing) return;
    setSyncing(true);
    setSyncError("");
    setProgress({ fetched: 0, total: 0, done: false });
    try {
      await SyncProfile(activeId);
      setRefreshKey((k) => k + 1);
      setDetailVersion((v) => v + 1);
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

  // Called by TestDetail after a successful inline edit. Refreshes the
  // grid (so it shows the new value) and the pending list. Deliberately
  // does NOT bump detailVersion — TestDetail already has the new value in
  // its own local state, and re-fetching mid-edit would risk clobbering
  // a field the user is still typing in.
  function handleEdited() {
    setRefreshKey((k) => k + 1);
    reloadPending();
  }

  // Called when the user discards a pending change from the modal. The
  // backend reverts test_case to before_val, so the detail panel needs to
  // re-fetch too.
  async function handleDiscard(id: number) {
    if (!activeId) return;
    try {
      await DiscardPendingChange(activeId, id);
      setRefreshKey((k) => k + 1);
      setDetailVersion((v) => v + 1);
      reloadPending();
    } catch (e) {
      console.error("discard:", errMsg(e));
    }
  }

  // Called when the user clicks "Commit" in the pending modal. Pushes all
  // pending changes to Jira; per-Test results land in lastCommitResult.
  // Committed pending rows are deleted by the backend; failures stay.
  async function handleCommit() {
    if (!activeId || committing) return;
    setCommitting(true);
    setLastCommitResult(null);
    try {
      const result = await CommitPendingChanges(activeId);
      setLastCommitResult(result);
      setRefreshKey((k) => k + 1);
      setDetailVersion((v) => v + 1);
      reloadPending();
    } catch (e) {
      setLastCommitResult({
        succeeded: [],
        conflicted: [],
        failed: [{ testKey: "", error: errMsg(e) }],
      });
    } finally {
      setCommitting(false);
    }
  }

  function closePendingModal() {
    setShowPending(false);
    setLastCommitResult(null);
  }

  const activeProfile = profiles.find((p) => p.id === activeId);
  const isDemo =
    !!activeProfile &&
    /^(demo$|demo:|mock:)/i.test(activeProfile.jiraUrl.trim());

  if (!health) {
    return <div className="centered muted">Loading…</div>;
  }

  if (!health.ok) {
    return (
      <div className="centered">
        <div className="onboard">
          <h2>Backend failed to start</h2>
          <pre className="backend-error">
            {health.error || "(no error message reported)"}
          </pre>
          {health.dbPath && (
            <p className="muted">
              Database path: <code>{health.dbPath}</code>
            </p>
          )}
          {health.logPath && (
            <p className="muted">
              Full log: <code>{health.logPath}</code>
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

        {pendingChanges.length > 0 && (
          <button
            className="btn btn-pending"
            onClick={() => setShowPending(true)}
            title="Show uncommitted edits"
          >
            Pending {pendingChanges.length}
          </button>
        )}

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

      {selectedSet.size > 0 && (
        <div className="bulk-toolbar">
          <span className="bulk-count">{selectedSet.size} selected</span>
          <button
            className="btn btn-primary"
            onClick={() => setShowBulkEdit(true)}
          >
            Bulk edit…
          </button>
          <button className="btn" onClick={() => setSelectedSet(new Set())}>
            Clear
          </button>
        </div>
      )}

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
          pendingByTestKey={pendingByTestKey}
          selectedSet={selectedSet}
          onSelect={setSelectedKey}
          onToggleSelect={toggleSelect}
          onToggleSelectPage={toggleSelectPage}
        />
        {selectedKey && (
          <TestDetail
            profileId={activeId}
            testKey={selectedKey}
            version={detailVersion}
            pendingForTest={pendingByTestKey.get(selectedKey) ?? []}
            onClose={() => setSelectedKey(null)}
            onEdited={handleEdited}
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

      {showPending && (
        <PendingChangesModal
          changes={pendingChanges}
          onDiscard={handleDiscard}
          onCommit={handleCommit}
          onJumpTo={(key) => {
            setSelectedKey(key);
            closePendingModal();
          }}
          onClose={closePendingModal}
          committing={committing}
          lastResult={lastCommitResult}
        />
      )}

      {showBulkEdit && (
        <BulkEditModal
          profileId={activeId}
          testKeys={[...selectedSet]}
          onComplete={() => {
            setRefreshKey((k) => k + 1);
            setDetailVersion((v) => v + 1);
            reloadPending();
            setSelectedSet(new Set());
            setShowBulkEdit(false);
          }}
          onCancel={() => {
            setRefreshKey((k) => k + 1);
            setDetailVersion((v) => v + 1);
            reloadPending();
            setShowBulkEdit(false);
          }}
        />
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
