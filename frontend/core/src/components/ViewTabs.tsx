export interface ViewTab<V extends string> {
  id: V;
  label: string;
  // The onboarding tour's hook for this tab; tab-<id> when not given.
  tour?: string;
}

interface Props<V extends string> {
  views: readonly ViewTab<V>[];
  current: V;
  onSelect: (id: V) => void;
}

// ViewTabs is the row of view tabs under the topbar, in its own band. The
// View menu and the optional rail reach the same places; this is the one
// that is always visible, so it is the one that says where you are.
export function ViewTabs<V extends string>({ views, current, onSelect }: Props<V>) {
  return (
    <nav className="view-tabs-bar" aria-label="Views">
      <div className="view-tabs" data-tour="views">
        {views.map((v) => (
          <button
            key={v.id}
            type="button"
            data-tour={v.tour ?? `tab-${v.id}`}
            className={`view-tab${v.id === current ? " view-tab-active" : ""}`}
            aria-current={v.id === current ? "page" : undefined}
            onClick={() => onSelect(v.id)}
          >
            {v.label}
          </button>
        ))}
      </div>
    </nav>
  );
}
