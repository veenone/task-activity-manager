import { formatDateTime } from "../../dates";
import { RunStatusBadge } from "./rows";
import type { TestDetailState } from "./useTestDetailState";

// RunHistorySection is the sortable table of this test's runs.
export function RunHistorySection({ s }: { s: TestDetailState }) {
  const { runHistorySort, canLinkToJira, runHistory, runHistoryLoading, runHistoryError, openBugInJira, toggleRunHistorySort, sortedRunHistory } = s;
  return (
    <>
      <h4>Run history</h4>
      {runHistoryError && (
        <div className="error-text">{runHistoryError}</div>
      )}
      {!runHistoryError && runHistoryLoading && (
        <p className="muted">Loading…</p>
      )}
      {!runHistoryError && !runHistoryLoading && runHistory !== null && (
        runHistory.length === 0 ? (
          <p className="muted">No run history yet.</p>
        ) : (
          <table aria-label="Run history" className="run-history-table">
            <thead>
              <tr>
                <th scope="col">Execution</th>
                <th scope="col"
                  style={{ cursor: "pointer", userSelect: "none", whiteSpace: "nowrap" }}
                  onClick={() => toggleRunHistorySort("runStatus")}
                  title="Sort by result"
                >
                  Result{runHistorySort.field === "runStatus" ? (runHistorySort.desc ? " ▾" : " ▴") : ""}
                </th>
                <th scope="col">By</th>
                <th scope="col">Environment</th>
                <th scope="col">Plan(s)</th>
                <th scope="col">Fix Version(s)</th>
                <th scope="col"
                  style={{ cursor: "pointer", userSelect: "none", whiteSpace: "nowrap" }}
                  onClick={() => toggleRunHistorySort("createdAt")}
                  title="Sort by created date"
                >
                  Created{runHistorySort.field === "createdAt" ? (runHistorySort.desc ? " ▾" : " ▴") : ""}
                </th>
                <th scope="col"
                  style={{ cursor: "pointer", userSelect: "none", whiteSpace: "nowrap" }}
                  onClick={() => toggleRunHistorySort("updatedAt")}
                  title="Sort by updated date"
                >
                  Updated{runHistorySort.field === "updatedAt" ? (runHistorySort.desc ? " ▾" : " ▴") : ""}
                </th>
                <th scope="col">Defects</th>
              </tr>
            </thead>
            <tbody>
              {sortedRunHistory(runHistory).map((r, i) => (
                <tr key={`${r.execKey}-${i}`}>
                  <td>
                    {canLinkToJira && r.execKey && !r.execKey.startsWith("NEW-") ? (
                      <button
                        className="mono bug-link-key"
                        onClick={() => openBugInJira(r.execKey)}
                        title={r.execSummary || `Open ${r.execKey} in Jira`}
                      >
                        {r.execKey}
                      </button>
                    ) : (
                      <span className="mono" title={r.execSummary}>{r.execKey}</span>
                    )}
                    {r.execSummary && (
                      <span className="muted run-history-exec-summary">{r.execSummary}</span>
                    )}
                  </td>
                  <td>
                    {r.runStatus ? <RunStatusBadge status={r.runStatus} /> : <span className="muted">—</span>}
                  </td>
                  <td>{r.executedBy || <span className="muted">—</span>}</td>
                  <td>{r.environment || <span className="muted">—</span>}</td>
                  <td>{r.planKeys?.length ? r.planKeys.join(", ") : <span className="muted">—</span>}</td>
                  <td>{r.fixVersions?.length ? r.fixVersions.join(", ") : <span className="muted">—</span>}</td>
                  <td className="muted run-history-date" style={{ whiteSpace: "nowrap" }}>
                    {formatDateTime(r.createdAt) || "—"}
                  </td>
                  <td className="muted run-history-date" style={{ whiteSpace: "nowrap" }}>
                    {formatDateTime(r.updatedAt) || "—"}
                  </td>
                  <td>
                    {r.defects?.length ? (
                      <span className="run-history-defects">
                        {r.defects.map((d, di) => (
                          <span key={d}>
                            {di > 0 && ", "}
                            {canLinkToJira && !d.startsWith("NEW-") ? (
                              <button
                                className="mono bug-link-key"
                                onClick={() => openBugInJira(d)}
                                title={`Open ${d} in Jira`}
                              >
                                {d}
                              </button>
                            ) : (
                              <span className="mono">{d}</span>
                            )}
                          </span>
                        ))}
                      </span>
                    ) : (
                      <span className="muted">—</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )
      )}
    </>
  );
}
