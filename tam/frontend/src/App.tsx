import { useEffect, useState } from "react";
import { Menu, LiveRegion, useProfile, errMsg } from "@agile-suite/core";
import { Health, EventsOn, isDemoUrl } from "./api";
import type { HealthInfo, Profile, Settings } from "./api";
import { VIEWS, useView } from "./nav";
import { useModal } from "./modals";
import { Placeholder } from "./components/Placeholder";
import { BacklogView } from "./components/BacklogView";
import { ProfilesModal } from "./components/ProfilesModal";
import { AboutModal } from "./components/AboutModal";
import { PendingChangesModal } from "./components/PendingChangesModal";
import { useSync } from "./contexts/SyncContext";
import { useSyncState } from "./queries/issues";
import { usePendingChanges } from "./queries/pending";
import { formatWhen } from "./lib/format";

// App is the shell: topbar, nav rail, the active view, and the status bar.
// The topbar, profile controls, and status bar mirror XTM's App.tsx/App.css
// so the two windows read as one product; the nav rail is TAM's own element
// (XTM switches views with topbar tabs instead of a rail).
export default function App() {
  const {
    profiles,
    activeId,
    setActiveId,
    activeProfile,
    theme,
    setTheme,
    reload,
    error: profileError,
  } = useProfile<Profile, Settings>();
  const { view, setView } = useView();
  const { isOpen, openModal, closeModal } = useModal();
  const { progress, syncError, canSync, canSwitchProfile, runSync } = useSync();
  const syncState = useSyncState(activeId);
  const pending = usePendingChanges(activeId);
  const pendingCount = pending.data?.length ?? 0;
  const [health, setHealth] = useState<HealthInfo | null>(null);

  useEffect(() => {
    Health()
      .then((h) => {
        setHealth(h);
        if (h.ok) void reload();
      })
      .catch((e) =>
        setHealth({ ok: false, error: errMsg(e), dbPath: "", sharedPath: "", logPath: "" }),
      );
  }, [reload]);

  useEffect(() => {
    const offProfiles = EventsOn("menu:profiles", () => openModal("profiles"));
    const offAbout = EventsOn("menu:about", () => openModal("about"));
    return () => {
      offProfiles();
      offAbout();
    };
  }, [openModal]);

  const current = VIEWS.find((v) => v.id === view) ?? VIEWS[0];
  const demo = isDemoUrl(activeProfile?.jiraUrl);
  const startupFailed = health !== null && !health.ok;

  return (
    <div className="app">
      <header className="topbar">
        <div className="topbar-zone topbar-left">
          <span className="brand">Task Activity Manager</span>
          {demo && <span className="demo-chip">DEMO</span>}
          <label className="sr-only" htmlFor="profile-select">Profile</label>
          <select
            id="profile-select"
            className="profile-select"
            value={activeId}
            onChange={(e) => setActiveId(e.target.value)}
            disabled={!canSwitchProfile}
          >
            {profiles.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name} ({p.projectKey})
              </option>
            ))}
            {profiles.length === 0 && <option value="">No profile</option>}
          </select>
          <button className="topbar-btn" onClick={() => openModal("profiles")}>Manage</button>
        </div>
        <div className="topbar-zone topbar-right">
          <Menu
            label="Sync"
            align="right"
            triggerClassName="topbar-btn topbar-btn-primary"
            items={[
              { key: "sync", label: "Sync changes", onClick: () => void runSync(false), disabled: !canSync },
              { key: "full", label: "Full sync", title: "Clears the cached issues and fetches everything", onClick: () => void runSync(true), disabled: !canSync },
            ]}
          />
          <Menu
            label="Theme"
            align="right"
            items={["light", "dark", "system"].map((t) => ({
              key: t,
              label: t[0].toUpperCase() + t.slice(1),
              checked: theme === t,
              onClick: () => void setTheme(t),
            }))}
          />
          <Menu
            label="Help"
            align="right"
            items={[{ key: "about", label: "About", onClick: () => openModal("about") }]}
          />
        </div>
      </header>

      <div className="app-body">
        <nav className="nav-rail" aria-label="Views">
          {VIEWS.map((v) => (
            <button
              key={v.id}
              className={`nav-item${v.id === view ? " nav-item-active" : ""}`}
              aria-current={v.id === view ? "page" : undefined}
              onClick={() => setView(v.id)}
            >
              {v.label}
            </button>
          ))}
          <div className="nav-divider" />
          <div className="nav-section">Suite</div>
          <button className="nav-item" disabled title="The launcher arrives in Phase 6">
            Tests (XTM)
          </button>
          <div className="nav-hint">opens Xray Test Manager</div>
        </nav>

        <main className="main">
          {startupFailed ? (
            <div className="startup-error" role="alert">
              <h2>The local store could not be opened</h2>
              <p>{health.error}</p>
              {health.logPath && <p>Log: {health.logPath}</p>}
            </div>
          ) : (
            <>
              <div className="view-head">
                <h2 id="view-title">{current.label}</h2>
                {activeProfile && (
                  <span className="muted">
                    {activeProfile.name} · {activeProfile.projectKey}
                  </span>
                )}
              </div>
              {current.id === "backlog" ? <BacklogView /> : <Placeholder view={current} />}
            </>
          )}
        </main>
      </div>

      <footer className="app-statusbar">
        <span className={`dot ${health?.ok ? "dot-ok" : "dot-warn"}`} aria-hidden="true" />
        <span>{health?.ok ? "Local store ready · tam.db" : "Starting up"}</span>
        {!startupFailed && profileError ? (
          <span className="error-text">Profiles could not be loaded: {profileError}</span>
        ) : activeProfile ? (
          <span data-testid="sync-summary">
            {syncState.data
              ? syncState.data.lastSynced
                ? `${syncState.data.issueCount.toLocaleString()} issues, last synced ${formatWhen(syncState.data.lastSynced)}`
                : "Not synced yet"
              : ""}
          </span>
        ) : (
          <span className="muted">Profiles shared with XTM · agile-suite/profiles.db</span>
        )}
        {progress && (
          <span className="chip chip-sync" role="status">
            {progress.total > 0
              ? `Syncing: ${progress.fetched} of ${progress.total}`
              : progress.stage || "Syncing"}
          </span>
        )}
        {pendingCount > 0 && (
          <button type="button" className="chip chip-pending" onClick={() => openModal("pending")}>
            {`${pendingCount} pending ${pendingCount === 1 ? "change" : "changes"}: Commit`}
          </button>
        )}
        {(syncError || syncState.data?.lastError) && !progress && (
          <span className="error-text" data-testid="sync-error">
            Last sync failed: {syncError || syncState.data?.lastError}
          </span>
        )}
        <span className="muted statusbar-right">Theme: {theme}</span>
      </footer>

      {/* LiveRegion's assertive channel is itself a role="alert" node, so it
          stands down while the startup error owns that role. Nothing calls
          announce() on this path anyway: no feature code runs without a
          store. */}
      {!startupFailed && <LiveRegion />}
      {isOpen("profiles") && <ProfilesModal onClose={closeModal} />}
      {isOpen("about") && <AboutModal onClose={closeModal} />}
      {isOpen("pending") && <PendingChangesModal onClose={closeModal} />}
    </div>
  );
}
