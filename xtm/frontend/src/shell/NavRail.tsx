import { SetNavRailVisible } from "../api";
import type { AppState } from "./useAppState";

// NavRail is the optional left rail listing the same views as the tab bar.
export function NavRail({ s }: { s: AppState }) {
  const { navRail, setNavRail, view, setView, shownViews } = s;
  return (
    <>
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
          {shownViews.map((v) => (
            <button
              key={v.id}
              className={`nav-item${v.id === view ? " nav-item-active" : ""}`}
              aria-current={v.id === view ? "page" : undefined}
              onClick={() => setView(v.id)}
            >
              {v.label}
            </button>
          ))}
        </nav>
      )}
    </>
  );
}
