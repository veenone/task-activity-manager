import { statusClass, statusText } from "./status";
import type { CoverageState } from "./useCoverageState";
import type { CoverageActions } from "./useCoverageActions";

// CoverageMatrix lists each group's parameters and values with their coverage,
// and maps tests to a value.
export function CoverageMatrix({ s, a }: { s: CoverageState; a: CoverageActions }) {
  const { versionId, newGroupName, setNewGroupName, setMapping, model, report, staleByValue } = s;
  const { addGroup, deleteNode } = a;
  return (
    <>
      <div className="cov-matrix">
        {!versionId && (
          <p className="cov-empty">Select or create a version above to view the coverage matrix.</p>
        )}
        {versionId && report &&
          report.groups.map((g) => {
            const grp = model?.groups.find((x) => x.id === g.groupId);
            return (
              <div key={g.groupId} className="cov-group">
                <div className="cov-group-head">
                  <span className="cov-group-name">{g.name}</span>
                  <span className="cov-group-pct">
                    {g.tested}/{g.total} ({g.percent}%)
                  </span>
                  <button className="cov-x" title="Delete group" onClick={() => void deleteNode("group", g.groupId)} aria-label="Delete group">
                    ×
                  </button>
                </div>
                <table aria-label={`Values in ${g.name}`} className="cov-table">
                  <tbody>
                    {grp?.parameters.flatMap((p) => {
                      const paramTested = p.values.filter((v) => report.values[v.id]?.tested === true).length;
                      return [
                        <tr key={`ph-${p.id}`} className="cov-param-head">
                          <td colSpan={3} className="cov-param-name">{p.name}</td>
                          <td className="cov-param-pct">{paramTested}/{p.values.length} tested</td>
                        </tr>,
                        ...p.values.map((v) => {
                          const vc = report.values[v.id];
                          return (
                            <tr key={v.id} className="cov-vrow">
                              <td className="cov-vlabel">
                                <span className="cov-vrow-bullet" aria-hidden="true">•</span>{" "}
                                {v.valueLabel}
                                {v.valueKind !== "value" && <span className="cov-kind">{v.valueKind}</span>}
                                {!v.isRequired && <span className="cov-kind cov-optional">optional</span>}
                              </td>
                              <td>
                                <span className={statusClass(vc)}>{statusText(vc)}</span>
                                {staleByValue[v.id] ? <span className="cov-stale-dot" title="stale mapping">⚠</span> : null}
                              </td>
                              <td className="cov-tests">{vc?.testKeys.join(", ")}</td>
                              <td>
                                <button className="btn" onClick={() => setMapping({ valueId: v.id, label: v.valueLabel })}>
                                  Map…
                                </button>
                              </td>
                            </tr>
                          );
                        }),
                      ];
                    })}
                  </tbody>
                </table>
              </div>
            );
          })}
        {versionId && (
          <div className="cov-addgroup">
            <input
              className="cov-input"
              placeholder="Add group…"
              value={newGroupName}
              onChange={(e) => setNewGroupName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") void addGroup();
              }}
            />
            <button className="btn" disabled={!newGroupName.trim()} onClick={() => void addGroup()}>
              Add group
            </button>
          </div>
        )}
      </div>
    </>
  );
}
