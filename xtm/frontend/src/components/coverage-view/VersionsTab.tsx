import { SetMemberVersion } from "../../api";
import { ChangeRequestsPanel } from "../ChangeRequestsPanel";
import { VersionDashboard } from "../VersionDashboard";
import type { CoverageState } from "./useCoverageState";
import type { CoverageActions } from "./useCoverageActions";

// VersionsTab is change requests, the version dashboard, and each member
// requirement's locked version.
export function VersionsTab({ s, a }: { s: CoverageState; a: CoverageActions }) {
  const { profileId, selected, versions, versionId, busy, reuse, reload, versionName } = s;
  const { applyVersionToAll } = a;
  return (
    <>
      <div className="cov-versions-tab">
        <ChangeRequestsPanel
          canonicalId={selected}
          versions={versions}
          onChanged={() => void reload()}
        />
        <VersionDashboard
          canonicalId={selected}
        />
        <div className="cov-member-locks">
          <div className="cov-member-locks-header">
            <h3 className="cov-section-title">
              Customer version locks
              {versionId
                ? ` (relative to ${versionName})`
                : ""}
            </h3>
            {versionId && reuse.length > 0 && (
              <button
                className="btn"
                disabled={busy}
                onClick={() => void applyVersionToAll()}
                title={`Set all member locks to ${versionName}`}
              >
                Apply {versionName} to all
              </button>
            )}
          </div>
          {versions.length === 0 ? (
            <p className="cov-muted">No versions yet. Create a version first to assign member locks.</p>
          ) : reuse.length === 0 ? (
            <p className="cov-muted">No member requirements linked to this canonical.</p>
          ) : (
            <table aria-label="Locked versions" className="cov-member-locks-table">
              <thead>
                <tr>
                  <th scope="col">Requirement</th>
                  <th scope="col">Project</th>
                  <th scope="col">Locked version</th>
                </tr>
              </thead>
              <tbody>
                {reuse.map((row) => (
                  <tr
                    key={row.requirementKey}
                    className={
                      versionId !== "" && row.acceptedVersionId === versionId
                        ? "cov-member-current"
                        : undefined
                    }
                  >
                    <td className="cov-member-key">{row.requirementKey}</td>
                    <td className="cov-muted">{row.projectKey || "(none)"}</td>
                    <td>
                      <select
                        className="cov-select"
                        value={row.acceptedVersionId}
                        onChange={(e) => {
                          void SetMemberVersion(
                            profileId,
                            selected,
                            row.requirementKey,
                            e.target.value,
                          ).then(() => reload());
                        }}
                      >
                        <option value="">(unassigned)</option>
                        {versions.map((v) => (
                          <option key={v.id} value={v.id}>
                            {v.name}
                          </option>
                        ))}
                      </select>
                      {versionId !== "" && row.acceptedVersionId === versionId && (
                        <span className="cov-member-badge">current</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </>
  );
}
