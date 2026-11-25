import { useEffect, useRef, useState } from "react";
import { LiveRegion, SyncRibbon, ViewTabs, clearViewState, isModalOpen, useProfile, useTour, errMsg } from "@agile-suite/core";
import { Health, EventsOn, SetTourSeenVersion, isDemoUrl } from "./api";
import type { HealthInfo, Profile, Settings, SyncProgress } from "./api";
import { VIEWS, useView } from "./nav";
import { WorklogView } from "./components/WorklogView";
import { TOURS, TOUR_VERSION, shouldOfferTour } from "./tour/steps";
import type { View } from "./nav";
import { useModal } from "./modals";
import { useGuarded } from "./lib/unsavedEdit";
import { BacklogView } from "./components/BacklogView";
import { AssignedToMeView } from "./components/AssignedToMeView";
import { EpicsView } from "./components/EpicsView";
import { BoardsView } from "./components/BoardsView";
import { SprintsView } from "./components/SprintsView";
import { ReportsView } from "./components/ReportsView";
import { DashboardsView } from "./components/DashboardsView";
import { RitualsView } from "./components/RitualsView";
import { DependenciesView } from "./components/DependenciesView";
import { FirstRun } from "./components/FirstRun";
import { NavRail } from "./components/NavRail";
import { PermissionNote } from "./components/PermissionNote";
import { TopBarActions } from "./components/TopBarActions";
import { ProfilesModal } from "./components/ProfilesModal";
import { AboutModal } from "./components/AboutModal";
import { DiagnosticsModal } from "./components/DiagnosticsModal";
import { SettingsModal } from "./components/SettingsModal";
import { PendingChangesModal } from "./components/PendingChangesModal";
import { useSync } from "./contexts/SyncContext";
import { useFullSync } from "./contexts/useFullSync";
import { useSyncState } from "./queries/issues";
import { usePendingChanges } from "./queries/pending";
import { formatWhen } from "./lib/format";

// cachedCount is what the status bar says about the cache: the rows it
// holds, and what the project holds when that is more. A sync narrowed by
// the profile's scope JQL, or by a type the backend keeps out, caches part
// of a project, and "38 issues" on its own read as a project of 38 when it
// held 2,943 (#68). The two are equal after a plain full sync, and the
// second half is left off then rather than repeating the number.
function cachedCount({ issueCount, projectTotal }: { issueCount: number; projectTotal: number }): string {
  const cached = issueCount.toLocaleString();
  return projectTotal > issueCount ? `${cached} of ${projectTotal.toLocaleString()}` : cached;
}

// App is the shell: topbar, nav rail, the active view, and the status bar.
// The topbar, profile controls, and status bar mirror XTM's App.tsx/App.css
// so the two windows read as one product; the nav rail is TAM's own element
// (XTM switches views with topbar tabs instead of a rail).
// SyncBar is XTM's status-bar progress bar, class for class: a fixed track
// with a filled proportion, and the stage beside it. A count only exists for
// a phase that knows its total (the issue pull, and now the boards pass,
// which counts boards), so the track is only drawn when there is a
// proportion to draw; everything else says what it is doing and trails off.
function SyncBar({ progress }: { progress: SyncProgress }) {
  const hasCount = progress.total > 0;
  const pct = hasCount ? Math.round((progress.fetched / progress.total) * 100) : 0;
  const stage = progress.stage || (progress.phase === "boards" ? "Boards" : "Syncing");
  return (
    <div className="syncbar" role="status">
      {hasCount && (
        <div
          className="syncbar-track"
          role="progressbar"
          aria-valuemin={0}
          aria-valuemax={progress.total}
          aria-valuenow={progress.fetched}
          aria-label={stage}
        >
          <div className="syncbar-fill" style={{ width: `${pct}%` }} />
        </div>
      )}
      <span className="muted">
        {stage}
        {hasCount ? `: ${progress.fetched.toLocaleString()} / ${progress.total.toLocaleString()}` : "…"}
      </span>
    </div>
  );
}

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
  // A view or profile switch unmounts the issue panel, so both ask first
  // when it holds an unsaved edit (#521).
  const switchView = useGuarded(setView);
  const switchProfile = useGuarded(setActiveId);
  const { isOpen, openModal, closeModal } = useModal();
  const { progress, syncError, canSync, canSwitchProfile, runSync, runRefresh } = useSync();
  const runFullSync = useFullSync(runSync, canSync);
  const syncState = useSyncState(activeId);
  const pending = usePendingChanges(activeId);
  const pendingCount = pending.data?.length ?? 0;
  const [health, setHealth] = useState<HealthInfo | null>(null);
  // Views are switched from the native View menu, the way XTM's are, so the
  // rail is a second and optional way to reach the same places. Its state
  // lives in the shared settings so the menu's tick and the rail agree
  // across restarts; the menu owns the toggle and tells us through an event.
  const [navRail, setNavRail] = useState(false);
  // Whether the first profile read has answered. Until it has, an empty
  // list is not yet a new install.
  const [profilesRead, setProfilesRead] = useState(false);
  // Which version of the onboarding tour this user has already been
  // through. It starts at the current one so a slow settings read cannot
  // flash the tour at somebody who has seen it; the read below lowers it
  // when they have not.
  const [tourSeenVersion, setTourSeenVersion] = useState(TOUR_VERSION);
  const { start: startTour } = useTour({
    tours: TOURS,
    version: TOUR_VERSION,
    markSeen: SetTourSeenVersion,
    onFinish: () => setTourSeenVersion(TOUR_VERSION),
  });

  useEffect(() => {
    Health()
      .then((h) => {
        setHealth(h);
        if (h.ok) {
          void reload().then((s) => {
            setProfilesRead(true);
            setNavRail(s?.showNavRail ?? false);
            setTourSeenVersion(s?.tourSeenVersion ?? 0);
          });
        }
      })
      .catch((e) =>
        setHealth({ ok: false, error: errMsg(e), dbPath: "", sharedPath: "", logPath: "" }),
      );
  }, [reload]);

  // A profile switch drops the old profile's view state, the way XTM's
  // does, so returning to it starts every view clean. The views mounted at
  // the time reset themselves; this reaches the ones that were not (#524).
  const viewStateFor = useRef(activeId);
  useEffect(() => {
    if (viewStateFor.current && viewStateFor.current !== activeId) clearViewState(viewStateFor.current);
    viewStateFor.current = activeId;
  }, [activeId]);

  // The first-run tour starts itself once the profile has synced, which is
  // the first moment there is anything to be shown around. offeredTour
  // keeps it to once a session: the seen version is written when the tour
  // ends, but a tour closed before that write lands must not reopen on the
  // next render.
  const offeredTour = useRef(false);
  const hasSynced = (syncState.data?.lastSynced ?? "") !== "";
  useEffect(() => {
    if (offeredTour.current) return;
    if (!shouldOfferTour(tourSeenVersion, hasSynced)) return;
    offeredTour.current = true;
    startTour("start", () => setView("backlog"));
  }, [tourSeenVersion, hasSynced, startTour, setView]);

  useEffect(() => {
    const offProfiles = EventsOn("menu:profiles", () => openModal("profiles"));
    const offAbout = EventsOn("menu:about", () => openModal("about"));
    const offDiagnostics = EventsOn("menu:diagnostics", () => openModal("diagnostics"));
    const offSettings = EventsOn("menu:settings", () => openModal("settings"));
    // The View menu is TAM's primary navigation. It sends the view id the
    // menu was built with, which is why menuViews in main.go has to stay in
    // step with VIEWS in nav.ts. Its accelerators fire with a dialog open,
    // so the switch waits until none is (#528).
    const offView = EventsOn("menu:view", (id: string) => {
      if (!isModalOpen() && VIEWS.some((v) => v.id === id)) switchView(id as View);
    });
    const offRail = EventsOn("menu:nav-rail", (visible: boolean) => setNavRail(visible));
    const offSync = EventsOn("menu:sync", () => void runSync(false));
    const offFullSync = EventsOn("menu:full-sync", () => void runFullSync());
    return () => {
      offProfiles();
      offAbout();
      offDiagnostics();
      offSettings();
      offView();
      offRail();
      offSync();
      offFullSync();
    };
  }, [openModal, switchView, runSync, runFullSync]);

  const current = VIEWS.find((v) => v.id === view) ?? VIEWS[0];
  const demo = isDemoUrl(activeProfile?.jiraUrl);
  const startupFailed = health !== null && !health.ok;
  const noProfile = profilesRead && !profileError && profiles.length === 0;
  // The run that just failed, or the one the store remembers from a previous
  // launch. A running sync supersedes both, so the bar reports what is
  // happening now.
  const lastSyncError = syncError || syncState.data?.lastError || "";
  const syncFailed = !!lastSyncError && !progress;

  return (
    <div className="app">
      <header className="topbar">
        <div className="topbar-zone topbar-left">
          <span className="brand">Task Activity Manager</span>
          {demo && <span className="demo-chip">DEMO</span>}
          <label className="sr-only" htmlFor="profile-select">Profile</label>
          <select
            id="profile-select"
            data-tour="profile"
            className="profile-select"
            value={activeId}
            onChange={(e) => switchProfile(e.target.value)}
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
        <TopBarActions
          pendingCount={pendingCount}
          canSync={canSync}
          theme={theme}
          onPending={() => openModal("pending")}
          onRefresh={() => void runRefresh()}
          onSync={(full) => void (full ? runFullSync() : runSync(false))}
          onTheme={(t) => void setTheme(t)}
          onTour={() => startTour(view)}
          onFirstRunTour={() => startTour("start", () => setView("backlog"))}
          onDiagnostics={() => openModal("diagnostics")}
          onAbout={() => openModal("about")}
        />
      </header>

      {!noProfile && <ViewTabs views={VIEWS} current={view} onSelect={switchView} />}

      <div className="app-body">
        {navRail && <NavRail view={view} onSelect={switchView} onHide={() => setNavRail(false)} />}

        <main className="main">
          {!startupFailed && !noProfile && current.gated && <PermissionNote profileId={activeId} />}
          {startupFailed ? (
            <div className="startup-error" role="alert">
              <h2>The local store could not be opened</h2>
              <p>{health.error}</p>
              {health.logPath && <p>Log: {health.logPath}</p>}
            </div>
          ) : !profilesRead ? (
            <p className="muted">Reading the profiles</p>
          ) : noProfile ? (
            <FirstRun onCreate={() => openModal("profiles")} />
          ) : (
            // No per-view title bar, the way XTM has none: the active tab
            // already names the view and the topbar's profile select already
            // names the project, so a heading repeating both only cost the
            // content 16px of height. Each view names its own landmark
            // instead of borrowing an id from a heading that is gone.
            current.id === "backlog" ? (
              <BacklogView />
            ) : current.id === "assigned" ? (
              <AssignedToMeView />
            ) : current.id === "worklog" ? (
              <WorklogView />
            ) : current.id === "epics" ? (
              <EpicsView />
            ) : current.id === "boards" ? (
              <BoardsView />
            ) : current.id === "sprints" ? (
              <SprintsView />
            ) : current.id === "reports" ? (
              <ReportsView onOpenBoards={() => setView("boards")} />
            ) : current.id === "dashboards" ? (
              <DashboardsView />
            ) : current.id === "dependencies" ? (
              <DependenciesView />
            ) : (
              <RitualsView />
            )
          )}
        </main>
      </div>

      {/* A failed sync takes the whole bar, the way XTM's does: beside the
          store state, the issue count and the theme it read as one more
          value rather than the one thing here to act on (#315). */}
      <footer className={syncFailed ? "app-statusbar statusbar-failed" : "app-statusbar"}>
        {syncFailed ? (
          <SyncRibbon message={lastSyncError} />
        ) : (
          <>
            <span className={`dot ${health?.ok ? "dot-ok" : "dot-warn"}`} aria-hidden="true" />
            <span>{health?.ok ? "Local store ready" : "Starting up"}</span>
            {!startupFailed && profileError ? (
              <span className="error-text">Profiles could not be loaded: {profileError}</span>
            ) : activeProfile ? (
              <span data-testid="sync-summary">
                {syncState.data
                  ? syncState.data.lastSynced
                    ? `${cachedCount(syncState.data)} issues, last synced ${formatWhen(syncState.data.lastSynced)}`
                    : "Not synced yet"
                  : ""}
              </span>
            ) : (
              <span className="muted">Profiles are shared with Xray Test Manager</span>
            )}
            {progress && <SyncBar progress={progress} />}
            <span className="muted statusbar-right">Theme: {theme}</span>
          </>
        )}
      </footer>

      {/* LiveRegion's assertive channel is itself a role="alert" node, so it
          stands down while the startup error owns that role. Nothing calls
          announce() on this path anyway: no feature code runs without a
          store. */}
      {!startupFailed && <LiveRegion />}
      {isOpen("profiles") && <ProfilesModal onClose={closeModal} />}
      {isOpen("about") && <AboutModal onClose={closeModal} />}
      {isOpen("diagnostics") && <DiagnosticsModal onClose={closeModal} />}
      {isOpen("settings") && <SettingsModal onClose={closeModal} />}
      {isOpen("pending") && <PendingChangesModal onClose={closeModal} />}
    </div>
  );
}
