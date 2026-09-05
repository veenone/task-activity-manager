import type { ViewInfo } from "../nav";

// Placeholder stands in for a view that a later phase delivers. The copy
// names the phase so nobody mistakes the empty state for a bug.
export function Placeholder({ view }: { view: ViewInfo }) {
  return (
    <section className="placeholder" aria-labelledby="view-title">
      <div className="placeholder-card">
        <div className="placeholder-glyph" aria-hidden="true" />
        <h2 className="placeholder-title">
          The {view.label} view arrives in {view.phase}
        </h2>
        <p className="placeholder-blurb">{view.blurb}</p>
        <p className="placeholder-blurb">
          This build proves the shell, the shared profiles, and the demo profile.
        </p>
      </div>
    </section>
  );
}
