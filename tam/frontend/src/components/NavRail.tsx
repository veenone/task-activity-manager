import { focusCurrentViewTab } from "@agile-suite/core";
import { SetNavRailVisible } from "../api";
import { VIEWS } from "../nav";
import type { View } from "../nav";

// The rail beside the views. Selecting a view goes through onSelect, so the
// shell's unsaved-edit guard applies here as it does to the tabs.
export function NavRail({ view, onSelect, onHide }: { view: View; onSelect: (v: View) => void; onHide: () => void }) {
  return (
    <nav className="nav-rail" aria-label="Navigation rail">
      <div className="nav-rail-head">
        <span className="nav-section">Views</span>
        <button
          type="button"
          className="btn btn-ghost nav-rail-hide"
          title="Hide the navigation rail (View menu, Ctrl+\)"
          aria-label="Hide the navigation rail"
          onClick={() => {
            onHide();
            // This button goes with the rail, so focus moves to the tab
            // of the view on screen instead of dropping to the body.
            focusCurrentViewTab();
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
          onClick={() => onSelect(v.id)}
        >
          {v.label}
        </button>
      ))}
    </nav>
  );
}
