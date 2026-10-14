import { useEffect, useRef } from "react";
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
  const { profiles, setNavRail, loadingProfiles, openModal, setView, health } = s;
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
    "menu:view-browse": () => setView("browse"),
    "menu:view-preconditions": () => setView("preconditions"),
    "menu:view-requirements": () => setView("requirements"),
    "menu:view-dashboard": () => setView("dashboard"),
    "menu:view-traceability": () => setView("traceability"),
    "menu:view-plans": () => setView("plans"),
    "menu:view-components": () => setView("components"),
    "menu:view-duplicates": () => setView("duplicates"),
    "menu:view-gapanalysis": () => setView("gapanalysis"),
    "menu:view-testcalls": () => setView("testcalls"),
    "menu:view-coverage": () => setView("coverage"),
    "menu:sync-history": () => openModal("syncHistory"),
    "menu:diagnostics": () => openModal("diagnostics"),
    "menu:about": () => openModal("about"),
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
