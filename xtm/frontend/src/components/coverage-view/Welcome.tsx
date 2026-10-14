import { CoverageMap } from "../CoverageMap";
import type { CoverageViewProps, CoverageState } from "./useCoverageState";
import type { CoverageActions } from "./useCoverageActions";

// Welcome is what shows with nothing selected: the Coverage Map tab, or how
// to build a model with the template and demo loaders.
export function Welcome({ s, a, p }: { s: CoverageState; a: CoverageActions; p: CoverageViewProps }) {
  const { tab, setTab, busy, canon } = s;
  const { downloadTemplate, loadDemoExample, loadPkcsReference } = a;
  const { isDemo, demoVariant } = p;
  return (
    <>
      <nav className="cov-tabs">
        <button
          className={`cov-tab${tab === "map" ? " cov-tab-active" : ""}`}
          onClick={() => setTab("map")}
        >
          Coverage Map
        </button>
      </nav>
      {tab === "map" ? (
        <CoverageMap isDemo={isDemo} demoVariant={demoVariant} />
      ) : (
        <div className="cov-welcome">
          <h2>Parameter-level coverage</h2>
          <p>
            This tab is a local workspace, and <strong>sync does not populate it</strong>. Sync only loads your
            tests and requirements; you build the coverage model here from them.
          </p>
          <ol>
            <li>
              In the left panel, name a function (e.g. <code>C_Sign</code>) and click <strong>Add</strong>.
            </li>
            <li>
              Select it, then <strong>Import template…</strong> to load a parameter workbook, or use
              <strong> Add group</strong> to build it by hand.
            </li>
            <li>
              Use <strong>Map…</strong> on each value to attach the tests that exercise it. The coverage % and
              gap list update live.
            </li>
            <li>
              Use <strong>Members</strong> to link the customer requirements that reuse this function.
            </li>
          </ol>
          <div className="cov-welcome-actions">
            <button className="btn" disabled={busy} onClick={() => void downloadTemplate()}>
              Download blank template…
            </button>
            {isDemo && (
              <button className="btn btn-primary" disabled={busy} onClick={() => void loadDemoExample()}>
                Load demo example (Login)
              </button>
            )}
            {isDemo && demoVariant && (
              <button className="btn btn-primary" disabled={busy} onClick={() => void loadPkcsReference()}>
                {demoVariant === "euicc" ? "Load eUICC coverage" : "Load PKCS#11 coverage"}
              </button>
            )}
          </div>
          {canon.length > 0 && <p className="cov-muted">Or pick an existing functional requirement on the left.</p>}
        </div>
      )}
    </>
  );
}
