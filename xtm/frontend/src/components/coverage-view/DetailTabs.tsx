import type { CoverageState } from "./useCoverageState";

// DetailTabs is the tab bar of the selected requirement.
export function DetailTabs({ s }: { s: CoverageState }) {
  const { tab, setTab, gaps, reuse } = s;
  return (
    <>
      <nav className="cov-tabs">
        <button
          className={`cov-tab${tab === "guide" ? " cov-tab-active" : ""}`}
          onClick={() => setTab("guide")}
        >
          Guide
        </button>
        <button
          className={`cov-tab${tab === "matrix" ? " cov-tab-active" : ""}`}
          onClick={() => setTab("matrix")}
        >
          Coverage
        </button>
        <button
          className={`cov-tab${tab === "gaps" ? " cov-tab-active" : ""}`}
          onClick={() => setTab("gaps")}
        >
          Gaps ({gaps.length})
        </button>
        <button
          className={`cov-tab${tab === "reuse" ? " cov-tab-active" : ""}`}
          onClick={() => setTab("reuse")}
        >
          Reuse ({reuse.length})
        </button>
        <button
          className={`cov-tab${tab === "versions" ? " cov-tab-active" : ""}`}
          onClick={() => setTab("versions")}
        >
          Versions &amp; CRs
        </button>
        <button
          className={`cov-tab${tab === "map" ? " cov-tab-active" : ""}`}
          onClick={() => setTab("map")}
        >
          Coverage Map
        </button>
      </nav>
    </>
  );
}
