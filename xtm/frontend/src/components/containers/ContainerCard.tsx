
import { RunBar, RunBadge } from "./RunCells";
import type { ContainerState } from "./useContainerState";
import type { ContainerActions, ContainerViewProps } from "./useContainerActions";

// ContainerCard is the selected container's detail. Its first line carries
// what an analyst needs while working the tests: kind, key, name, status,
// count, a slim run bar, Related bugs and the add buttons. Parent,
// environments, fix versions and the result badges fold behind the caret,
// so the member table below keeps the height (#456). The bar, badges and
// fix versions only show; the Filters row is the one place that filters,
// and they mark what it has chosen.
export function ContainerCard({ s, a, p }: { s: ContainerState; a: ContainerActions; p: ContainerViewProps }) {
  const {
    kind, envDraft, setEnvDraft, bugsExpanded, setBugsExpanded, editingName, setEditingName,
    nameDraft, setNameDraft, setShowAdd, setShowJUnitImport, memberFvFilter,
    memberRunFilter, cardCollapsed, setCardCollapsed, caps, board, relatedBugs, focusRef,
    kindLabel, selectedContainer, rollup,
  } = s;
  const { commitInlineRename, addEnvironment, removeEnvironment, openParent, openBug, openBreakdown } = a;
  const { isDemo, jiraUrl } = p;
  if (!selectedContainer) return null;
  const showBugs = kind === "testexec" || relatedBugs.length > 0;
  return (
    <div className="container-card">
      <div className="container-card-top">
        <button
          type="button"
          className="collapse-caret"
          onClick={() => setCardCollapsed(!cardCollapsed)}
          aria-expanded={!cardCollapsed}
          aria-label={cardCollapsed ? "Show details" : "Hide details"}
          title={cardCollapsed ? "Show details" : "Hide details"}
        >
          {cardCollapsed ? "▸" : "▾"}
        </button>
        <span className={`kind-badge kind-${kind}`}>{kindLabel}</span>
        <span className="mono container-card-key">{selectedContainer.key}</span>
        {editingName ? (
          <input
            className="container-card-name-edit"
            ref={focusRef}
            value={nameDraft}
            onChange={(e) => setNameDraft(e.target.value)}
            onBlur={commitInlineRename}
            onKeyDown={(e) => {
              if (e.key === "Enter") commitInlineRename();
              if (e.key === "Escape") setEditingName(false);
            }}
          />
        ) : (
          <h2 className="container-card-name" title={selectedContainer.summary}>
            <button
              type="button"
              title="Click to rename"
              onClick={() => {
                setNameDraft(selectedContainer.summary);
                setEditingName(true);
              }}
            >
              {selectedContainer.summary || "(untitled)"}
            </button>
          </h2>
        )}
        {selectedContainer.status && <span className="status-pill">{selectedContainer.status}</span>}
        <span className="container-card-count">
          {(board?.rows.length ?? 0).toLocaleString()} test
          {(board?.rows.length ?? 0) === 1 ? "" : "s"}
        </span>
        {board && board.runCounts.length > 0 && (
          <span className="container-card-runbar">
            <RunBar counts={board.runCounts} active={memberRunFilter} />
          </span>
        )}
        <span className="container-card-actions">
          {showBugs && (
            <button
              type="button"
              className="btn btn-ghost"
              aria-expanded={bugsExpanded}
              onClick={() => setBugsExpanded((e) => !e)}
              title={bugsExpanded ? "Hide related bugs" : "Show the bugs linked through this container's tests"}
            >
              Related bugs{relatedBugs.length > 0 ? ` (${relatedBugs.length})` : ""}
            </button>
          )}
          <button className="btn container-card-add" onClick={() => setShowAdd(true)} title={`Assign tests to this ${kindLabel}`}>
            + Add tests
          </button>
          {kind === "testexec" && (
            <button className="btn" onClick={() => setShowJUnitImport(true)} title="Import test run results from a JUnit XML report">
              Import results
            </button>
          )}
        </span>
      </div>

      {/* Related defects reached through this container's member Tests (#219).
          The list opens under the line and scrolls within its own box, so a
          long one never pushes the member table off-screen (-274). */}
      {showBugs && bugsExpanded && (
        <div className="tp-bugs-body">
          {relatedBugs.length === 0 ? (
            <span className="muted">None</span>
          ) : (
            <ul className="container-bugs-list">
              {relatedBugs.map((b) => (
                <li key={b.key} className="container-bug">
                  <button
                    className="mono container-bug-key"
                    onClick={() => openBug(b.key)}
                    title={isDemo || !jiraUrl ? b.key : `Open ${b.key} in Jira`}
                  >
                    {b.key}
                  </button>
                  <span className="container-bug-summary">{b.summary}</span>
                  {b.status && <span className="status-pill container-bug-status">{b.status}</span>}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      {!cardCollapsed && (
        <div className="container-card-details">
          {selectedContainer.parentKey && (
            <div className="container-parent">
              <button
                className="mono container-parent-link"
                onClick={() => openParent(selectedContainer.parentKey)}
                title={`Open parent ${selectedContainer.parentKey} in Jira`}
              >
                ↳ {selectedContainer.parentKey}
              </button>
              {selectedContainer.issueType && <span className="container-parent-type">{selectedContainer.issueType}</span>}
            </div>
          )}

          {caps.supportsEnvironments && kind === "testexec" && (
            <div className="container-environments">
              <span className="container-env-label">Environments</span>
              <div className="container-env-chips">
                {(selectedContainer.environments ?? []).length === 0 && <span className="muted">None</span>}
                {(selectedContainer.environments ?? []).map((env) => (
                  <span key={env} className="env-chip">
                    {env}
                    <button
                      className="env-chip-remove"
                      title={`Remove ${env}`}
                      aria-label={`Remove ${env}`}
                      onClick={() => removeEnvironment(env)}
                    >
                      ✕
                    </button>
                  </span>
                ))}
                <input
                  className="container-env-add app-select"
                  value={envDraft}
                  placeholder="Add environment…"
                  onChange={(e) => setEnvDraft(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") addEnvironment();
                  }}
                  title="Type an environment name and press Enter"
                />
              </div>
            </div>
          )}

          {kind === "testexec" && (selectedContainer.fixVersions ?? []).length > 0 && (
            <div className="container-environments">
              <span className="container-env-label">Fix version(s)</span>
              <div className="container-env-chips">
                {(selectedContainer.fixVersions ?? []).map((fv) => (
                  <span key={fv} className={`env-chip fix-version-chip${memberFvFilter === fv ? " fix-version-chip--active" : ""}`}>
                    {fv}
                  </span>
                ))}
              </div>
            </div>
          )}

          {board && board.runCounts.length > 0 && (
            <div className="board-counts">
              {board.runCounts.map((b) => (
                <RunBadge
                  key={b.label}
                  status={b.label}
                  count={b.count}
                  active={memberRunFilter === b.label}
                />
              ))}
            </div>
          )}

          {/* A plan's or set's consolidated results across its executions. */}
          {kind !== "testexec" && rollup && rollup.total > 0 && (
            <div className="container-rollup">
              <span
                className="container-rollup-label"
                title={
                  `Each test in this ${kindLabel} gets one combined result across the ` +
                  `${rollup.execCount} Test Execution${rollup.execCount === 1 ? "" : "s"} that ran it. ` +
                  `The worst result wins, so any FAIL makes the test FAIL overall. These badges count ` +
                  `tests by that combined result; "(not run)" means no execution has recorded a result yet. ` +
                  `Click a badge to see which tests are behind it.`
                }
              >
                Consolidated results across {rollup.execCount} execution{rollup.execCount === 1 ? "" : "s"}
                <span className="rollup-info" aria-hidden="true">ⓘ</span>
              </span>
              <div className="board-counts">
                {(
                  [
                    ["PASS", rollup.passed],
                    ["FAIL", rollup.failed],
                    ["EXECUTING", rollup.executing],
                    ["ABORTED", rollup.aborted],
                    ["BLOCKED", rollup.blocked],
                    ["(not run)", rollup.notRun],
                  ] as const
                )
                  .filter(([, n]) => n > 0)
                  .map(([status, n]) => (
                    <RunBadge
                      key={status}
                      status={status}
                      count={n}
                      onPick={() => openBreakdown(status)}
                      pickHint="See the tests behind this result"
                    />
                  ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
