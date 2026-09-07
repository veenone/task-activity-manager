import { useEffect, useState } from "react";
import { Menu, LiveRegion, useProfile, errMsg } from "@agile-suite/core";
import { Health, EventsOn, SetNavRailVisible, isDemoUrl } from "./api";
import type { HealthInfo, Profile, Settings } from "./api";
import { VIEWS, useView } from "./nav";
import type { View } from "./nav";
import { useModal } from "./modals";
import { Placeholder } from "./components/Placeholder";
import { BacklogView } from "./components/BacklogView";
import { EpicsView } from "./components/EpicsView";
import { BoardsView } from "./components/BoardsView";
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
  // Views are switched from the native View menu, the way XTM's are, so the
  // rail is a second and optional way to reach the same places. Its state
  // lives in the shared settings so the menu's tick and the rail agree
  // across restarts; the menu owns the toggle and tells us through an event.
  const [navRail, setNavRail] = useState(false);

  useEffect(() => {
    Health()
      .then((h) => {
        setHealth(h);
        if (h.ok) void reload().then((s) => setNavRail(s?.showNavRail ?? false));
      })
      .catch((e) =>
        setHealth({ ok: false, error: errMsg(e), dbPath: "", sharedPath: "", logPath: "" }),
      );
  }, [reload]);

  useEffect(() => {
    const offProfiles = EventsOn("menu:profiles", () => openModal("profiles"));
    const offAbout = EventsOn("menu:about", () => openModal("about"));
    // The View menu is TAM's primary navigation. It sends the view id the
    // menu was built with, which is why menuViews in main.go has to stay in
    // step with VIEWS in nav.ts.
    const offView = EventsOn("menu:view", (id: string) => {
      if (VIEWS.some((v) => v.id === id)) setView(id as View);
    });
    const offRail = EventsOn("menu:nav-rail", (visible: boolean) => setNavRail(visible));
    const offSync = EventsOn("menu:sync", () => void runSync(false));
    const offFullSync = EventsOn("menu:full-sync", () => void runSync(true));
    return () => {
      offProfiles();
      offAbout();
      offView();
      offRail();
      offSync();
      offFullSync();
    };
  }, [openModal, setView, runSync]);

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
          {pendingCount > 0 && (
            <button
              type="button"
              className="btn-pending"
              onClick={() => openModal("pending")}
              title="Show uncommitted changes"
            >
              <span className="pending-dot" aria-hidden="true" />
              {`${pendingCount} pending`}
            </button>
          )}
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

      {/* The view tabs XTM carries under its topbar. The View menu and the
          optional rail reach the same places; this is the one that is always
          visible, so it is the one that says where you are. */}
      <nav className="view-tabs-bar" aria-label="Views">
        <div className="view-tabs">
          {VIEWS.map((v) => (
            <button
              key={v.id}
              type="button"
              className={`view-tab${v.id === view ? " view-tab-active" : ""}`}
              aria-current={v.id === view ? "page" : undefined}
              onClick={() => setView(v.id)}
            >
              {v.label}
            </button>
          ))}
        </div>
      </nav>

      <div className="app-body">
        {navRail && (
          <nav className="nav-rail" aria-label="Navigation rail">
            <div className="nav-rail-head">
              <span className="nav-section">Views</span>
              <button
                type="button"
                className="btn btn-ghost nav-rail-hide"
                title="Hide the navigation rail (View menu, Ctrl+B)"
                aria-label="Hide the navigation rail"
                onClick={() => {
                  setNavRail(false);
                  // Persisting through the bound method rebuilds the native
                  // menu, so the View menu's tick follows a rail hidden from
                  // here rather than going stale.
                  void SetNavRailVisible(false).catch(() => {});
                }}
              >
                ✕
              </button>
            </div>
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
        )}

        <main className="main">
          {startupFailed ? (
            <div className="startup-error" role="alert">
              <h2>The local store could not be opened</h2>
              <p>{health.error}</p>
              {health.logPath && <p>Log: {health.logPath}</p>}
            </div>
          ) : (
            // No per-view title bar, the way XTM has none: the active tab
            // already names the view and the topbar's profile select already
            // names the project, so a heading repeating both only cost the
            // content 16px of height. Each view names its own landmark
            // instead of borrowing an id from a heading that is gone.
            current.id === "backlog" ? (
              <BacklogView />
            ) : current.id === "epics" ? (
              <EpicsView />
            ) : current.id === "boards" ? (
              <BoardsView />
            ) : (
              <Placeholder view={current} />
            )
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
