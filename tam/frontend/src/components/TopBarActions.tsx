import { Menu } from "@agile-suite/core";

interface Props {
  pendingCount: number;
  // canSync is the shell's lock: a sync, a commit or a refresh holds it, and
  // every control that would take it waits.
  canSync: boolean;
  theme: string;
  onPending: () => void;
  onRefresh: () => void;
  onSync: (full: boolean) => void;
  onTheme: (theme: string) => void;
  onTour: () => void;
  onFirstRunTour: () => void;
  onDiagnostics: () => void;
  onAbout: () => void;
}

const THEMES = ["light", "dark", "system"];

// TopBarActions is the right side of the top bar, in XTM's order: the pending
// count, this app's own Refresh, More, and Sync last (#540). Sync is one click
// for the common case; Full sync, the tours, Diagnostics, About and the theme
// sit under More, where XTM keeps Full resync, its tour, Diagnostics and the
// theme.
export function TopBarActions(p: Props) {
  return (
    <div className="topbar-zone topbar-right">
      {p.pendingCount > 0 && (
        <button type="button" className="btn-pending" onClick={p.onPending} title="Show uncommitted changes">
          <span className="pending-dot" aria-hidden="true" />
          {`${p.pendingCount} pending`}
        </button>
      )}
      {/* Refresh is for the whole window, not for one section: it drops
          the cached issue details and reads the active view again, which
          is how a profile whose details never expire asks Jira for them.
          It takes the same lock a sync does, so it is offered on exactly
          the same terms. */}
      <button
        type="button"
        className="topbar-btn"
        onClick={p.onRefresh}
        disabled={!p.canSync}
        title="Read this view again and fetch the issue details afresh"
      >
        Refresh
      </button>
      {/* The tours point at this menu to say where they live, and Menu
          renders its own button, so the wrapper carries the target. */}
      <span data-tour="help">
        <Menu
          label="More"
          align="right"
          title="Full sync, help and theme"
          items={[
            { key: "full", label: "Full sync", title: "Fetches every issue again and drops the ones Jira no longer returns", onClick: () => p.onSync(true), disabled: !p.canSync },
            { key: "help-div", divider: true },
            { key: "tour", label: "Take the tour", title: "A short walk through this view", onClick: p.onTour },
            { key: "start", label: "Take the first-run tour", title: "Sync, plan, commit: the loop TAM is built around", onClick: p.onFirstRunTour },
            { key: "diagnostics", label: "Diagnostics", title: "Paths, build and the recent log", onClick: p.onDiagnostics },
            { key: "about", label: "About", onClick: p.onAbout },
            { key: "theme-div", divider: true },
            ...THEMES.map((t) => ({
              key: `theme-${t}`,
              label: `Theme: ${t[0].toUpperCase()}${t.slice(1)}`,
              checked: p.theme === t,
              onClick: () => p.onTheme(t),
            })),
          ]}
        />
      </span>
      <button
        type="button"
        data-tour="sync"
        className="topbar-btn topbar-btn-primary"
        onClick={() => p.onSync(false)}
        disabled={!p.canSync}
        title="Fetch what changed in Jira since the last sync"
      >
        Sync
      </button>
    </div>
  );
}
