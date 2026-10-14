import { VersionBar } from "../VersionBar";
import { CoveragePublishPanel } from "../CoveragePublishPanel";
import type { CoverageState } from "./useCoverageState";
import type { CoverageActions } from "./useCoverageActions";

// DetailHead is the selected requirement's name and headline, its template,
// report and members actions, the version bar, and the publish panel.
export function DetailHead({ s, a }: { s: CoverageState; a: CoverageActions }) {
  const { selected, versions, versionId, setVersionId, busy, setShowMembers, report, reuse, stale, reload, selectedCanon } = s;
  const { importTemplate, exportReport, downloadTemplate } = a;
  return (
    <>
      <header className="cov-detail-head">
        <h2>{selectedCanon?.name}</h2>
        {report && (
          <span className="cov-headline" title="Required parameter values covered">
            {report.percent}% · {report.testedValues}/{report.totalValues}
          </span>
        )}
        <div className="cov-actions">
          <button className="btn" disabled={busy || !versionId} onClick={() => void importTemplate()}>
            Import template…
          </button>
          <button className="btn" disabled={busy || !versionId} onClick={() => void exportReport()}>
            Export report…
          </button>
          <button className="btn" disabled={busy} onClick={() => void downloadTemplate()}>
            Blank template…
          </button>
          <button className="btn" onClick={() => setShowMembers(true)}>
            Members ({reuse.length})
          </button>
        </div>
      </header>

      <VersionBar
        versions={versions}
        value={versionId}
        onChange={setVersionId}
        canonicalId={selected}
        onChanged={() => void reload()}
      />

      {stale.length > 0 && (
        <div className="cov-stale">
          ⚠ {stale.length} mapping{stale.length === 1 ? "" : "s"} reference tests no longer in the local cache (kept, not counted).
        </div>
      )}

      <CoveragePublishPanel versionId={versionId} />
    </>
  );
}
