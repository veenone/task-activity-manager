import { Fragment } from "react";
import { BrowserOpenURL } from "../../api";
import { formatDateTime } from "../../dates";
import type { BugsState, BugsPanelProps } from "./useBugsState";

// AffectedTests is the selected bug's linked tests. A row expands to its run
// history, grouped by Test Plan and sortable by column.
export function AffectedTests({ s, p }: { s: BugsState; p: BugsPanelProps }) {
  const { setDetailKey, setSidebarDetail, setDetailVersion, expandedTests, runHistoryCache, runHistoryLoading, tests, canLink, getRunSort, toggleRunSort, sortedRuns, toggleTestExpand } = s;
  const { jiraUrl, onOpenTest } = p;
  return (
    <>
      <h4>Affected tests ({tests.length})</h4>
      {tests.length === 0 ? (
        <p className="muted">No affected tests.</p>
      ) : (
        <table aria-label="Tests linked to this bug" className="board-table">
          <thead>
            <tr>
              <th scope="col" style={{ width: "1.5rem" }} />
              <th scope="col">Test</th>
              <th scope="col" style={{ width: "1.5rem" }} />
              <th scope="col">Project</th>
              <th scope="col">Summary</th>
              <th scope="col">Status</th>
              <th scope="col">Latest result</th>
            </tr>
          </thead>
          <tbody>
            {tests.map((t) => {
              const isExpanded = expandedTests.has(t.key);
              const isLoading = runHistoryLoading.has(t.key);
              const history = runHistoryCache.get(t.key);
              return (
                <Fragment key={t.key}>
                  <tr>
                    <td>
                      <button
                        className="btn-icon"
                        title={isExpanded ? "Collapse run history" : "Expand run history"}
                        onClick={() => toggleTestExpand(t.key)}
                        style={{ fontSize: "0.75rem", padding: "0 0.25rem" }}
                      >
                        {isExpanded ? "▾" : "▸"}
                      </button>
                    </td>
                    <td>
                      <button
                        className="mono bug-link-key"
                        onClick={() => onOpenTest(t.key)}
                        title={`Open ${t.key} in Browse`}
                      >
                        {t.key}
                      </button>
                    </td>
                    <td>
                      <button
                        className="btn-icon"
                        title={`Open ${t.key} detail here`}
                        onClick={() => {
                          setDetailKey(t.key);
                          setDetailVersion((v) => v + 1);
                          setSidebarDetail({ kind: "test", key: t.key });
                        }}
                        style={{ fontSize: "0.75rem", padding: "0 0.25rem" }}
                      >
                        ↗
                      </button>
                    </td>
                    <td className="muted">{t.project || "—"}</td>
                    <td>{t.summary}</td>
                    <td>{t.status || "—"}</td>
                    <td>
                      {t.runStatus ? (
                        <span
                          className={`run-badge run-${t.runStatus.toLowerCase()}`}
                        >
                          {t.runStatus}
                        </span>
                      ) : (
                        <span className="muted">not run</span>
                      )}
                    </td>
                  </tr>
                  {isExpanded && (
                    <tr>
                      <td colSpan={10} style={{ padding: "0.25rem 0.5rem 0.5rem 2rem", background: "var(--bg-subtle, #f8f8f8)" }}>
                        {isLoading ? (
                          <span className="muted">Loading run history…</span>
                        ) : !history || history.length === 0 ? (
                          <span className="muted">No run history for this test.</span>
                        ) : (
                          (() => {
                            const sort = getRunSort(t.key);
                            function SortTh({ field, children }: { field: string; children: React.ReactNode }) {
                              const active = sort.field === field;
                              return (
                                <th scope="col"
                                  style={{ cursor: "pointer", userSelect: "none", whiteSpace: "nowrap" }}
                                  onClick={() => toggleRunSort(t.key, field)}
                                  title={`Sort by ${field}`}
                                >
                                  {children}
                                  {active ? (sort.desc ? " ▾" : " ▴") : ""}
                                </th>
                              );
                            }
                            // Group runs by plan key. A run that appears in multiple plans
                            // shows up under each of them. Runs with no planKeys go last.
                            const planKeys: string[] = [];
                            const seenPlans = new Set<string>();
                            for (const r of history) {
                              for (const pk of (r.planKeys ?? [])) {
                                if (!seenPlans.has(pk)) { seenPlans.add(pk); planKeys.push(pk); }
                              }
                            }
                            const hasNoPlan = history.some(r => !r.planKeys?.length);
                            const groups: Array<{ planKey: string | null; runs: typeof history }> = [
                              ...planKeys.map(pk => ({
                                planKey: pk,
                                runs: history.filter(r => r.planKeys?.includes(pk)),
                              })),
                              ...(hasNoPlan ? [{ planKey: null, runs: history.filter(r => !r.planKeys?.length) }] : []),
                            ];
                            const RunRow = ({ r, i }: { r: typeof history[0]; i: number }) => {
                              const isSubTask = !!r.execParentKey;
                              const base = (jiraUrl ?? "").trim().replace(/\/+$/, "");
                              return (
                              <tr key={`${r.execKey}-${i}`}>
                                <td>
                                  <span style={{ display: "flex", alignItems: "baseline", gap: "0.35em", flexWrap: "wrap" }}>
                                    {r.execKey ? (
                                      <button
                                        className="mono bug-link-key"
                                        onClick={() => setSidebarDetail({ kind: "exec", key: r.execKey })}
                                        title={`Open Test Execution ${r.execKey}`}
                                      >
                                        {r.execKey}
                                      </button>
                                    ) : (
                                      <span className="muted">—</span>
                                    )}
                                    {isSubTask && (
                                      <span
                                        className="exec-subtask-badge"
                                        title={r.execIssueType || "Sub Test Execution"}
                                      >
                                        Sub-task
                                      </span>
                                    )}
                                  </span>
                                  {r.execSummary && (
                                    <span className="muted" style={{ display: "block", fontSize: "0.9em" }}>{r.execSummary}</span>
                                  )}
                                  {isSubTask && (
                                    <span className="exec-parent-line">
                                      <span className="exec-parent-label">↳ Parent:</span>{" "}
                                      {canLink && !r.execParentKey.startsWith("NEW-") ? (
                                        <button
                                          className="mono bug-link-key"
                                          onClick={() => BrowserOpenURL(`${base}/browse/${r.execParentKey}`)}
                                          title={`Open ${r.execParentKey} in Jira`}
                                        >
                                          {r.execParentKey}
                                        </button>
                                      ) : (
                                        <span className="mono">{r.execParentKey}</span>
                                      )}
                                      {r.execParentSummary && (
                                        <span className="muted exec-parent-summary"> — {r.execParentSummary}</span>
                                      )}
                                    </span>
                                  )}
                                </td>
                                <td>
                                  {r.runStatus ? (
                                    <span className={`run-badge run-${r.runStatus.toLowerCase()}`}>{r.runStatus}</span>
                                  ) : (
                                    <span className="muted">—</span>
                                  )}
                                </td>
                                <td>{r.fixVersions?.length ? r.fixVersions.join(", ") : <span className="muted">—</span>}</td>
                                <td>{r.environment || <span className="muted">—</span>}</td>
                                <td className="muted">{formatDateTime(r.finishedAt || r.startedAt)}</td>
                                <td className="muted" style={{ whiteSpace: "nowrap" }}>{formatDateTime(r.execCreated) || <span className="muted">—</span>}</td>
                                <td className="muted" style={{ whiteSpace: "nowrap" }}>{formatDateTime(r.execUpdated) || <span className="muted">—</span>}</td>
                                <td className="muted" style={{ whiteSpace: "nowrap" }}>{formatDateTime(r.execResolved) || <span className="muted">—</span>}</td>
                                <td>{r.executedBy || <span className="muted">—</span>}</td>
                                <td>
                                  {r.defects?.length ? (
                                    <span>
                                      {r.defects.map((d, di) => (
                                        <span key={d}>
                                          {di > 0 && ", "}
                                          {canLink && !d.startsWith("NEW-") ? (
                                            <button
                                              className="mono bug-link-key"
                                              onClick={() => {
                                                const base = (jiraUrl ?? "").trim().replace(/\/+$/, "");
                                                BrowserOpenURL(`${base}/browse/${d}`);
                                              }}
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
                            );
                            };
                            return (
                              <table aria-label={`Test runs of ${t.key}`} className="board-table" style={{ fontSize: "0.85em" }}>
                                <thead>
                                  <tr>
                                    <SortTh field="execKey">Execution</SortTh>
                                    <SortTh field="runStatus">Result</SortTh>
                                    <SortTh field="fixVersions">Fix Version(s)</SortTh>
                                    <SortTh field="environment">Environment</SortTh>
                                    <SortTh field="finishedAt">Run Date</SortTh>
                                    <SortTh field="execCreated">Created</SortTh>
                                    <SortTh field="execUpdated">Updated</SortTh>
                                    <SortTh field="execResolved">Resolved</SortTh>
                                    <SortTh field="executedBy">By</SortTh>
                                    <SortTh field="defects">Defects</SortTh>
                                  </tr>
                                </thead>
                                <tbody>
                                  {groups.map((g) => (
                                    <Fragment key={g.planKey ?? "__no_plan__"}>
                                      <tr className="run-plan-group-header">
                                        <td colSpan={10}>
                                          {g.planKey ? (
                                            <button
                                              className="mono bug-link-key"
                                              onClick={() => setSidebarDetail({ kind: "plan", key: g.planKey! })}
                                              title={`Open Test Plan ${g.planKey}`}
                                            >
                                              Plan: {g.planKey}
                                            </button>
                                          ) : (
                                            <span className="muted">No test plan</span>
                                          )}
                                          <span className="muted" style={{ marginLeft: "0.5em" }}>
                                            ({g.runs.length} run{g.runs.length === 1 ? "" : "s"})
                                          </span>
                                        </td>
                                      </tr>
                                      {sortedRuns(g.runs, sort).map((r, i) => (
                                        <RunRow key={`${g.planKey ?? ""}:${r.execKey}-${i}`} r={r} i={i} />
                                      ))}
                                    </Fragment>
                                  ))}
                                </tbody>
                              </table>
                            );
                          })()
                        )}
                      </td>
                    </tr>
                  )}
                </Fragment>
              );
            })}
          </tbody>
        </table>
      )}
    </>
  );
}
