import { useEffect, useRef } from "react";
import { ViewTabs } from "@agile-suite/core";
import { EventsOn } from "./api";
import { ProfileForm } from "./components/ProfileForm";
import { LiveRegion } from "./components/LiveRegion";
import "./App.css";
import { useAppState } from "./shell/useAppState";
import { useAppActions } from "./shell/useAppActions";
import { TopBar } from "./shell/TopBar";
import { BulkToolbar } from "./shell/BulkToolbar";
import { NavRail } from "./shell/NavRail";
import { MainView } from "./shell/MainView";
import { DialogHost } from "./shell/DialogHost";
import { StatusBar } from "./shell/StatusBar";

// App is the shell. useAppState and useAppActions in shell/ hold what it knows
// and does; App waits for the backend and a first profile, wires the native
// menu, and lays out the top bar, view host, dialogs and status bar.
function App() {
  const s = useAppState();
  const a = useAppActions(s);
  const { profiles, setNavRail, loadingProfiles, openModal, view, setView, shownViews, health } = s;
  const { runSync, runFullSync, handleCreated } = a;

  // Native menu bar (built in main.go) drives the same actions via events. A ref
  // holds the latest handlers so a single subscription always sees current
  // state, rather than capturing stale closures.
  const menuActions = useRef<Record<string, () => void>>({});
  menuActions.current = {
    "menu:sync": runSync,
    "menu:full-sync": runFullSync,
    "menu:new-profile": () => openModal("form"),
    "menu:import": () => openModal("import"),
    "menu:sync-history": () => openModal("syncHistory"),
    "menu:diagnostics": () => openModal("diagnostics"),
    "menu:about": () => openModal("about"),
  };
  // The View menu sends the id of the view it was built with. Go builds it
  // once with every view, so the ones this profile hides are ignored here.
  const selectFromMenu = useRef<(id: string) => void>(() => {});
  selectFromMenu.current = (id) => {
    const v = shownViews.find((sv) => sv.id === id);
    if (v) setView(v.id);
  };
  useEffect(() => {
    const unsubs = Object.keys(menuActions.current).map((event) =>
      EventsOn(event, () => menuActions.current[event]?.()),
    );
    // Subscribed on its own because it carries the new state: the menu
    // checkbox is the source of truth for the rail, not a toggle of whatever
    // the frontend last had.
    unsubs.push(
      EventsOn("menu:nav-rail", (visible: boolean) => setNavRail(visible)),
      EventsOn("menu:view", (id: string) => selectFromMenu.current(id)),
    );
    return () => unsubs.forEach((u) => u && u());
  }, [setNavRail]);

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
            Try removing the database file and relaunching. If that doesn't
            help, check the log for more detail.
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
      <TopBar s={s} a={a} />

      <ViewTabs views={shownViews} current={view} onSelect={setView} />

      <BulkToolbar s={s} />

      <div className="app-body">
        <NavRail s={s} />

        <MainView s={s} a={a} />
      </div>

      <DialogHost s={s} a={a} />

      <StatusBar s={s} />

      <LiveRegion />
    </div>
  );
}

export default App;
