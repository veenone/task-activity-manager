
import { Fragment } from "react";
import { RUN_STATUSES, PAGE_SIZE_OPTIONS } from "./constants";
import { ExecRunCells, DefectsCell, RemarksCell } from "./RunCells";
import { ParameterizedRunRows } from "./ParameterizedRunRows";
import type { ContainerState } from "./useContainerState";
import type { ContainerActions, ContainerViewProps } from "./useContainerActions";

// MemberTable is the selected container's member tests: bulk result setting,
// the sortable table with run cells, and the pager.
export function MemberTable({ s, a, p }: { s: ContainerState; a: ContainerActions; p: ContainerViewProps }) {
  const {
    kind, setBugFor, setLinkBugFor, selectedRuns, setSelectedRuns, bulkStatus, setBulkStatus,
    setBoardPage, memberFvFilter, setDetailKey, setDetailVersion, pageSize, setPageSize, caps,
    profileId, selected, expandedRuns, setExpandedRuns, parameterizedTests, setParameterizedTests,
    containers, board, memberRuns, kindLabel, allRows, boardTotalPages, safePage, pageRows,
  } = s;
  const {
    removeTest, setRunStatus, unlinkBug, setRunComment, toggleRun, toggleRowSort, rowSortIndicator,
    applyBulkRunStatus, openBug,
  } = a;
  const { isDemo } = p;
  return (
    <>
        {kind === "testexec" && selectedRuns.size > 0 && (
          <div className="board-bulk">
            <span className="bulk-count">{selectedRuns.size} selected</span>
            <span className="muted">Set result:</span>
            <select
              className="run-select"
              value={bulkStatus}
              onChange={(e) => setBulkStatus(e.target.value)}
            >
              {RUN_STATUSES.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
            <button className="btn btn-primary" onClick={applyBulkRunStatus}>
              Apply to selected
            </button>
            <button className="btn" onClick={() => setSelectedRuns(new Set())}>
              Clear
            </button>
          </div>
        )}

        <div className="board-scroll">
        {board && containers.length > 0 && (
          <table aria-label="Run history" className="board-table">
            <thead>
              <tr>
                {kind === "testexec" && (
                  <th scope="col" className="board-check-col">
                    <input
                      type="checkbox"
                      checked={
                        pageRows.length > 0 &&
                        pageRows.every((r) => selectedRuns.has(r.testKey))
                      }
                      onChange={(e) => {
                        const checked = e.target.checked;
                        setSelectedRuns((prev) => {
                          const next = new Set(prev);
                          pageRows.forEach((r) =>
                            checked
                              ? next.add(r.testKey)
                              : next.delete(r.testKey),
                          );
                          return next;
                        });
                      }}
                      title="Select all on this page"
                    />
                  </th>
                )}
                <th scope="col"
                  className="board-sort-th"
                  onClick={() => toggleRowSort("key")}
                  title="Sort by test key"
                >
                  Test{rowSortIndicator("key")}
                </th>
                <th scope="col"
                  className="board-sort-th"
                  onClick={() => toggleRowSort("summary")}
                  title="Sort by summary"
                >
                  Summary{rowSortIndicator("summary")}
                </th>
                <th scope="col"
                  className="board-sort-th"
                  onClick={() => toggleRowSort("status")}
                  title="Sort by status"
                >
                  Status{rowSortIndicator("status")}
                </th>
                <th scope="col"
                  className="board-sort-th"
                  onClick={() => toggleRowSort("result")}
                  title="Sort by run result"
                >
                  Execution{rowSortIndicator("result")}
                </th>
                {kind === "testexec" && <th scope="col" title="Run date (finished or started)">Date</th>}
                {kind === "testexec" && <th scope="col" title="Executed by">By</th>}
                {kind === "testexec" && <th scope="col" title="Environment">Environment</th>}
                {kind === "testexec" && <th scope="col" title="Member test's own Fix Version(s)">Fix Version</th>}
                {kind === "testexec" && <th scope="col" title="Defects linked to this run">Defects</th>}
                {kind === "testexec" && <th scope="col" title="Remark / comment on this run">Remarks</th>}
                <th scope="col" aria-label="Remove" />
              </tr>
            </thead>
            <tbody>
              {allRows.length === 0 ? (
                <tr>
                  <td colSpan={kind === "testexec" ? 12 : 5} className="muted">
                    This {kindLabel.toLowerCase()} doesn't have any tests yet.
                    Use "+ Add tests" to add some.
                  </td>
                </tr>
              ) : (
                pageRows.map((r) => {
                  const resultKey = `${selected}:${r.testKey}`;
                  const expanded = expandedRuns.has(resultKey);
                  const parameterized = parameterizedTests.has(resultKey);
                  return <Fragment key={r.testKey}>
                  <tr
                    className={selectedRuns.has(r.testKey) ? "board-row-sel" : ""}
                  >
                    {kind === "testexec" && (
                      <td className="board-check-col">
                        <input
                          type="checkbox"
                          checked={selectedRuns.has(r.testKey)}
                          onChange={() => toggleRun(r.testKey)}
                        />
                      </td>
                    )}
                    <td className="mono">
                      {kind === "testexec" && caps.supportsParameterizedRunDetail && (
                        <button
                          type="button"
                          className="parameterized-run-toggle"
                          aria-expanded={expanded}
                          aria-label={`${expanded ? "Hide" : "Show"} dataset iterations for ${r.testKey}`}
                          onClick={() => setExpandedRuns((current) => {
                            const next = new Set(current);
                            if (next.has(resultKey)) next.delete(resultKey); else next.add(resultKey);
                            return next;
                          })}
                        >
                          <span aria-hidden="true">{expanded ? "▾" : "▸"}</span>
                        </button>
                      )}
                      {r.testKey}
                      {r.isExternal && (
                        <span
                          className="ext-badge"
                          title="This test lives in a different Jira project than this profile"
                        >
                          ext
                        </span>
                      )}
                      <button
                        className="btn-icon"
                        title={`Open ${r.testKey} detail here`}
                        onClick={() => {
                          setDetailKey(r.testKey);
                          setDetailVersion((v) => v + 1);
                        }}
                        style={{ fontSize: "0.75rem", padding: "0 0.25rem", marginLeft: "0.25rem" }}
                      >
                        ↗
                      </button>
                    </td>
                    <td>{r.summary}</td>
                    <td>{r.status || "(none)"}</td>
                    <td>
                      {kind === "testexec" ? (
                        <select
                          className={`run-select run-${(r.runStatus || "todo").toLowerCase()}`}
                          value={r.runStatus || ""}
                          onChange={(e) => setRunStatus(r.testKey, e.target.value)}
                          disabled={parameterized}
                          aria-label={`Overall result for ${r.testKey}`}
                          title={parameterized ? "Calculated from dataset iterations" : "Set this test's result in this execution"}
                        >
                          {!r.runStatus && <option value="">Set result…</option>}
                          {RUN_STATUSES.map((s) => (
                            <option key={s} value={s}>
                              {s}
                            </option>
                          ))}
                        </select>
                      ) : r.runStatus ? (
                        <span
                          className={`run-badge run-${r.runStatus.toLowerCase()}`}
                        >
                          {r.runStatus}
                        </span>
                      ) : (
                        <span className="muted">not run</span>
                      )}
                      {parameterized && <span className="parameterized-aggregate-note">Calculated from iterations</span>}
                    </td>
                    {kind === "testexec" && (
                      <ExecRunCells
                        run={memberRuns.get(r.testKey)}
                        activeFixVersion={memberFvFilter}
                      />
                    )}
                    {kind === "testexec" && (
                      <DefectsCell
                        testKey={r.testKey}
                        defects={memberRuns.get(r.testKey)?.defects ?? []}
                        isDemo={isDemo}
                        onOpenBug={openBug}
                        onUnlink={unlinkBug}
                        onLinkClick={() => setLinkBugFor(r.testKey)}
                      />
                    )}
                    {kind === "testexec" && (
                      <RemarksCell
                        testKey={r.testKey}
                        comment={memberRuns.get(r.testKey)?.comment ?? ""}
                        onSave={setRunComment}
                      />
                    )}
                    <td className="board-remove-cell">
                      {kind === "testexec" &&
                        (caps.supportsBugCreation || caps.supportsBugRouting) &&
                        /^fail/i.test(r.runStatus || "") && (
                          <button
                            className="btn btn-ghost board-bug"
                            title="Create a bug for this failed test"
                            aria-label="Create bug for this failed test"
                            onClick={() => setBugFor({ testKey: r.testKey, summary: r.summary })}
                          >
                            🐞
                          </button>
                        )}
                      <button
                        className="btn btn-ghost board-remove"
                        onClick={() => removeTest(r.testKey)}
                        title={`Remove from this ${kindLabel}`}
                        aria-label={`Remove from this ${kindLabel}`}
                      >
                        ✕
                      </button>
                    </td>
                  </tr>
                  {kind === "testexec" && caps.supportsParameterizedRunDetail && expanded && (
                    <tr className="parameterized-detail-row">
                      <td colSpan={12}>
                        <ParameterizedRunRows
                          profileId={profileId}
                          execKey={selected}
                          testKey={r.testKey}
                          expanded={expanded}
                          canWrite={caps.supportsParameterizedRunWrites}
                          onLoaded={() => setParameterizedTests((current) => {
                            if (current.has(resultKey)) return current;
                            const next = new Set(current);
                            next.add(resultKey);
                            return next;
                          })}
                        />
                      </td>
                    </tr>
                  )}
                </Fragment>;
                })
              )}
            </tbody>
          </table>
        )}
        </div>

        {board && allRows.length > 0 && (
          <div className="board-pager">
            <label className="board-pagesize">
              <span className="muted">Rows per page</span>
              <select
                className="app-select"
                value={pageSize}
                onChange={(e) => {
                  setPageSize(Number(e.target.value));
                  setBoardPage(0);
                }}
              >
                {PAGE_SIZE_OPTIONS.map((n) => (
                  <option key={n} value={n}>
                    {n}
                  </option>
                ))}
              </select>
            </label>
            <span className="muted board-pager-range">
              {(safePage * pageSize + 1).toLocaleString()}–
              {Math.min(
                (safePage + 1) * pageSize,
                allRows.length,
              ).toLocaleString()}{" "}
              of {allRows.length.toLocaleString()} · page {safePage + 1} of{" "}
              {boardTotalPages}
            </span>
            <span className="board-pager-nav">
              <button
                className="btn"
                disabled={safePage === 0}
                onClick={() => setBoardPage((p) => Math.max(0, p - 1))}
              >
                ‹ Prev
              </button>
              <button
                className="btn"
                disabled={safePage >= boardTotalPages - 1}
                onClick={() =>
                  setBoardPage((p) => Math.min(boardTotalPages - 1, p + 1))
                }
              >
                Next ›
              </button>
            </span>
          </div>
        )}
    </>
  );
}
