import { Menu } from "../components/Menu";
import type { AppState } from "./useAppState";
import type { AppActions } from "./useAppActions";

// TopBar is the header: brand, profile switcher and Manage menu on the left,
// the view tabs in the middle, and pending, New Test, More and Sync on the right.
export function TopBar({ s, a }: { s: AppState; a: AppActions }) {
  const { profiles, activeId, setActiveId, theme, showCoverage, chooseTheme, openModal, pulling, view, groupBy, selectedFolder, setSelectedKey, startTour, pendingChanges, isDemo, syncActive } = s;
  const { runSync, runFullSync, toggleCoverage, openNewTest } = a;
  return (
    <header className="topbar">
      <div className="topbar-zone topbar-left">
        <span className="brand">Xray Test Manager</span>
        {isDemo && <span className="demo-chip">DEMO</span>}
        <select
          data-tour="profile"
          className="profile-select"
          value={activeId}
          disabled={syncActive}
          title={
            syncActive
              ? "You can't switch profiles while a sync is running"
              : "Switch active profile"
          }
          onChange={(e) => {
            // Guard against a race with an in-flight sync: the sync writes to
            // the store keyed by the old profile, so switching mid-sync would
            // point the UI at a new profile while stale progress events and
            // reloads are still landing. The disabled attribute prevents it;
            // this early return is belt-and-braces for menu/keyboard paths.
            if (syncActive) return;
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
        <Menu
          label="⚙ Manage"
          title="Manage profiles and connections"
          items={[
            {
              key: "profiles",
              label: "Manage Profiles…",
              onClick: () => openModal("profiles"),
              title:
                "Manage profiles: add, edit, set default, export, or delete",
            },
            {
              key: "connections",
              label: "Connections…",
              onClick: () => openModal("connections"),
              title:
                "Manage this workspace's connections. Add a target " +
                "(e.g. Kiwi) alongside its primary connection.",
            },
            {
              key: "bridge",
              label: "Bridge…",
              onClick: () => openModal("bridge"),
              title:
                "Publish or migrate this workspace's tests from one connection to another",
            },
          ]}
        />
      </div>

      <div data-tour="pending" className="topbar-zone topbar-right">
        {pendingChanges.length > 0 && (
          <button
            className="btn-pending"
            onClick={() => openModal("pending")}
            title="Show uncommitted edits"
          >
            <span className="pending-dot" aria-hidden="true" />
            {pendingChanges.length} pending
          </button>
        )}

        {view === "browse" && (
          <button
            className="btn btn-primary"
            onClick={() =>
              openNewTest(groupBy === "folder" ? selectedFolder : "")
            }
            title="Create a new test case"
          >
            ＋ New Test
          </button>
        )}

        {/* Wrapper carries the tour target: Menu renders its own
            button internally, so the attribute cannot go on it. */}
        <span data-tour="more">
          <Menu
            label="More"
            align="right"
            title="Tools & diagnostics"
            items={[
              {
                key: "importtests",
                label: "Import tests…",
                onClick: () => openModal("import"),
                title: "Import tests from a CSV or XLSX file",
              },
              {
                key: "labels",
                label: "Labels…",
                onClick: () => openModal("labels"),
                title: "Rename, merge or delete a label across synced tests",
              },
              {
                key: "fullsync",
                label: "Full resync (re-pull folders)",
                onClick: runFullSync,
                title:
                  "Force a full re-sync, ignoring the incremental watermark. " +
                  "This re-maps Test Repository folder membership.",
              },
              {
                key: "history",
                label: "Sync history",
                onClick: () => openModal("syncHistory"),
              },
              // Help group. The tour sat between the sync actions and
              // Diagnostics with nothing separating it, which made it easy
              // to miss in an eleven-item menu.
              { key: "help-div", divider: true },
              {
                key: "tour",
                label: "🧭 Take the tour",
                onClick: () => startTour(view),
                title: "Replay this view's walkthrough",
              },
              {
                key: "diag",
                label: "Diagnostics",
                onClick: () => openModal("diagnostics"),
                title: "Logs & diagnostics",
              },
              { key: "td", divider: true },
              {
                key: "t-light",
                label: "Theme: Light",
                checked: theme === "light",
                onClick: () => chooseTheme("light"),
              },
              {
                key: "t-dark",
                label: "Theme: Dark",
                checked: theme === "dark",
                onClick: () => chooseTheme("dark"),
              },
              {
                key: "t-system",
                label: "Theme: System",
                checked: theme === "system",
                onClick: () => chooseTheme("system"),
              },
              { key: "cov-div", divider: true },
              {
                key: "show-coverage",
                label: "Show Coverage tab",
                checked: showCoverage,
                onClick: () => void toggleCoverage(),
                title:
                  "Reveal the Coverage module tab (opt-in; hidden by default)",
              },
            ]}
          />
        </span>

        <button
          data-tour="sync"
          className="btn btn-primary"
          onClick={runSync}
          disabled={pulling}
        >
          {pulling ? "Syncing…" : "Sync"}
        </button>
      </div>
    </header>
  );
}
