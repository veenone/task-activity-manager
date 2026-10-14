
import { RunBar, RunBadge } from "./RunCells";
import type { ContainerState } from "./useContainerState";
import type { ContainerActions, ContainerViewProps } from "./useContainerActions";

// ContainerCard is the selected container's detail: name, parent,
// environments, fix versions, run bar and related defects.
export function ContainerCard({ s, a, p }: { s: ContainerState; a: ContainerActions; p: ContainerViewProps }) {
  const {
    kind, envDraft, setEnvDraft, bugsExpanded, setBugsExpanded, editingName, setEditingName,
    nameDraft, setNameDraft, setShowAdd, setShowJUnitImport, memberFvFilter, setMemberFvFilter,
    memberRunFilter, cardCollapsed, setCardCollapsed, caps, board, relatedBugs, focusRef,
    kindLabel, selectedContainer, pickRunFilter,
  } = s;
  const { commitInlineRename, addEnvironment, removeEnvironment, openParent, openBug } = a;
  const { isDemo, jiraUrl } = p;
  return (
    <>
        {selectedContainer && (
          <div className="container-card">
            <div className="container-card-top">
              <button
                type="button"
                className="collapse-caret"
                onClick={() => setCardCollapsed(!cardCollapsed)}
                aria-expanded={!cardCollapsed}
                title={cardCollapsed ? "Show details" : "Hide details"}
              >
                {cardCollapsed ? "▸" : "▾"}
              </button>
              <span className={`kind-badge kind-${kind}`}>{kindLabel}</span>
              <span className="mono container-card-key">
                {selectedContainer.key}
              </span>
              {selectedContainer.status && (
                <span className="status-pill">{selectedContainer.status}</span>
              )}
              <span className="container-card-count">
                {(board?.rows.length ?? 0).toLocaleString()} test
                {(board?.rows.length ?? 0) === 1 ? "" : "s"}
              </span>
              <button
                className="btn container-card-add"
                onClick={() => setShowAdd(true)}
                title={`Assign tests to this ${kindLabel}`}
              >
                + Add tests
              </button>
              {kind === "testexec" && (
                <button
                  className="btn"
                  onClick={() => setShowJUnitImport(true)}
                  title="Import test run results from a JUnit XML report"
                >
                  Import results (JUnit XML)
                </button>
              )}
            </div>

            {!cardCollapsed && (
              <>
            {selectedContainer.parentKey && (
              <div className="container-parent">
                <button
                  className="mono container-parent-link"
                  onClick={() => openParent(selectedContainer.parentKey)}
                  title={`Open parent ${selectedContainer.parentKey} in Jira`}
                >
                  ↳ {selectedContainer.parentKey}
                </button>
                {selectedContainer.issueType && (
                  <span className="container-parent-type">
                    {selectedContainer.issueType}
                  </span>
                )}
              </div>
            )}

            {caps.supportsEnvironments && kind === "testexec" && (
              <div className="container-environments">
                <span className="container-env-label">Environments</span>
                <div className="container-env-chips">
                  {(selectedContainer.environments ?? []).length === 0 && (
                    <span className="muted">None</span>
                  )}
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

            {kind === "testexec" &&
              (selectedContainer.fixVersions ?? []).length > 0 && (
                <div className="container-environments">
                  <span className="container-env-label">Fix version(s)</span>
                  <div className="container-env-chips">
                    {memberFvFilter && (
                      <button
                        key="__all"
                        className="env-chip fix-version-chip fix-version-chip--all"
                        title="Clear fix-version filter (show all members)"
                        onClick={() => setMemberFvFilter("")}
                      >
                        All
                      </button>
                    )}
                    {(selectedContainer.fixVersions ?? []).map((fv) => (
                      <button
                        key={fv}
                        className={`env-chip fix-version-chip${memberFvFilter === fv ? " fix-version-chip--active" : ""}`}
                        title={
                          memberFvFilter === fv
                            ? `Showing only members with fix version ${fv} (click to clear)`
                            : `Filter member table to fix version ${fv}`
                        }
                        onClick={() =>
                          setMemberFvFilter((cur) => (cur === fv ? "" : fv))
                        }
                      >
                        {fv}
                      </button>
                    ))}
                  </div>
                </div>
              )}

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
              <h2 className="container-card-name">
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

            {board && board.runCounts.length > 0 && (
              <div className="container-card-runs">
                <RunBar
                  counts={board.runCounts}
                  active={memberRunFilter}
                  onPick={pickRunFilter}
                />
                <div className="board-counts">
                  {board.runCounts.map((b) => (
                    <RunBadge
                      key={b.label}
                      status={b.label}
                      count={b.count}
                      active={memberRunFilter === b.label}
                      onPick={() => pickRunFilter(b.label)}
                    />
                  ))}
                </div>
              </div>
            )}
              </>
            )}

            {/* Related defects reached through this container's member Tests -
                shown for executions, or for any container that has linked bugs.
                Surfaces a bug that reaches this execution only via a cross-project
                member Test, which the per-test Bugs panel cannot show (#219).
                The section is collapsible so a large bug list can never push the
                member table below off-screen (-274). */}
            {(kind === "testexec" || relatedBugs.length > 0) && (
              <div className="tp-bugs-collapsible">
                <button
                  className="tp-bugs-header"
                  onClick={() => setBugsExpanded((e) => !e)}
                  title={bugsExpanded ? "Collapse related bugs" : "Expand related bugs"}
                >
                  <span
                    className="tp-bugs-chevron"
                    style={{ transform: bugsExpanded ? "rotate(90deg)" : "none" }}
                  >
                    ▶
                  </span>
                  <span>Related bugs</span>
                  {relatedBugs.length > 0 && (
                    <span className="container-bugs-count">
                      ({relatedBugs.length})
                    </span>
                  )}
                </button>
                {bugsExpanded && (
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
                              title={
                                isDemo || !jiraUrl
                                  ? b.key
                                  : `Open ${b.key} in Jira`
                              }
                            >
                              {b.key}
                            </button>
                            <span className="container-bug-summary">{b.summary}</span>
                            {b.status && (
                              <span className="status-pill container-bug-status">
                                {b.status}
                              </span>
                            )}
                          </li>
                        ))}
                      </ul>
                    )}
                  </div>
                )}
              </div>
            )}
          </div>
        )}
    </>
  );
}
