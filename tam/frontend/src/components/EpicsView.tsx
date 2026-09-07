import { useEffect, useMemo, useRef, useState } from "react";
import { useProfile } from "@agile-suite/core";
import type { EpicTreeData, Issue, Profile, Settings, TreeQuery } from "../api";
import { useEpicTree } from "../queries/tree";
import { useSprints } from "../queries/issues";
import { EpicTree, MOVED_FLASH_MS } from "./EpicTree";
import { IssueDetailPanel } from "./IssueDetailPanel";
import { useModal } from "../modals";
import { NewIssueModal } from "./NewIssueModal";
import { useDebounced } from "../lib/useDebounced";

const SEARCH_DELAY_MS = 250;

// findIssue looks an issue up in the tree by key, whichever branch holds it:
// an epic itself, one of its children, or an orphan.
function findIssue(tree: EpicTreeData, key: string): Issue | undefined {
  if (!key) return undefined;
  for (const node of tree.epics) {
    if (node.issue.key === key) return node.issue;
    const child = node.children.find((c) => c.key === key);
    if (child) return child;
  }
  return tree.orphans.find((o) => o.key === key);
}

// EpicsView is the epic to story to task tree: a toolbar over EpicTree, with
// the same detail panel the Backlog uses for whichever issue is selected.
export function EpicsView() {
  const { activeId } = useProfile<Profile, Settings>();
  const [text, setText] = useState("");
  const [sprintId, setSprintId] = useState("");
  const [showDone, setShowDone] = useState(false);
  const [selectedKey, setSelectedKey] = useState("");
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [movedKey, setMovedKey] = useState("");

  // A parentKey edit made in the detail panel journals the change and the
  // tree query is invalidated with it, so the freshly loaded tree carries
  // the issue's new parent. Watching that value for the selected issue is
  // how EpicsView notices the move without the panel needing to say so.
  const lastParentRef = useRef<Map<string, string>>(new Map());

  // Filters, selection, and the expanded set all belong to the profile they
  // were set for, so a switch clears them in the render that first sees the
  // new id, the same way BacklogView resets its own filters.
  const [filtersFor, setFiltersFor] = useState(activeId);
  if (filtersFor !== activeId) {
    setFiltersFor(activeId);
    setText("");
    setSprintId("");
    setShowDone(false);
    setSelectedKey("");
    setExpanded(new Set());
    setMovedKey("");
    lastParentRef.current.clear();
  }

  const search = useDebounced(text, SEARCH_DELAY_MS, activeId);
  const query = useMemo<TreeQuery>(() => ({ text: search, sprintId, showDone }), [search, sprintId, showDone]);
  const tree = useEpicTree(activeId, query);
  const sprints = useSprints(activeId);
  const { isOpen, openModal, closeModal } = useModal();

  useEffect(() => {
    if (!tree.data) return;
    const issue = findIssue(tree.data, selectedKey);
    if (!issue) return;
    const prev = lastParentRef.current.get(issue.key);
    if (prev !== undefined && prev !== issue.parentKey) setMovedKey(issue.key);
    lastParentRef.current.set(issue.key, issue.parentKey);
  }, [tree.data, selectedKey]);

  useEffect(() => {
    if (!movedKey) return;
    const t = setTimeout(() => setMovedKey(""), MOVED_FLASH_MS);
    return () => clearTimeout(t);
  }, [movedKey]);

  const selected = tree.data ? findIssue(tree.data, selectedKey) : undefined;
  const epicCount = tree.data?.epics.length ?? 0;
  const orphanCount = tree.data?.orphans.length ?? 0;
  const issueCount = (tree.data?.epics.reduce((sum, e) => sum + e.children.length, 0) ?? 0) + orphanCount;
  const filtered = search !== "" || sprintId !== "" || showDone;
  const empty = epicCount === 0 && orphanCount === 0;

  return (
    <section className="backlog" aria-labelledby="view-title">
      <div className="filter-bar">
        <input
          type="search"
          className="detail-input filter-search"
          aria-label="Search epics and issues"
          placeholder="Search summary, key, label"
          value={text}
          onChange={(e) => setText(e.target.value)}
        />
        <select
          className="detail-input filter-sprint"
          aria-label="Sprint"
          value={sprintId}
          onChange={(e) => setSprintId(e.target.value)}
        >
          <option value="">All sprints</option>
          {(sprints.data ?? []).map((s) => (
            <option key={s.id} value={s.id}>{s.name}</option>
          ))}
        </select>
        <label className="check-row" htmlFor="epics-show-done">
          <input
            id="epics-show-done"
            type="checkbox"
            aria-label="Show done"
            checked={showDone}
            onChange={(e) => setShowDone(e.target.checked)}
          />
          Show done
        </label>
        <button type="button" className="btn" onClick={() => setExpanded(new Set((tree.data?.epics ?? []).map((e) => e.issue.key)))}>
          Expand all
        </button>
        <button type="button" className="btn" onClick={() => setExpanded(new Set())}>
          Collapse all
        </button>
        <button type="button" className="btn btn-primary filter-new" disabled={!activeId} onClick={() => openModal("newIssue")}>
          + New epic
        </button>
      </div>

      <p className="muted epics-summary">{`${epicCount} epics, ${issueCount} issues, ${orphanCount} without an epic`}</p>
      {tree.data?.truncated && <p className="muted small">Showing the first 5,000 issues. Narrow the filter.</p>}

      <div className="epics-body">
        <div className="epics-tree-pane">
          {tree.isError ? (
            <p className="error-text">Could not load the epics: {tree.error.message}</p>
          ) : tree.isLoading ? (
            <p className="muted">Loading epics</p>
          ) : empty ? (
            <p className="muted">
              {filtered ? "No epics or issues match this filter." : "No issues cached yet. Use Sync in the topbar."}
            </p>
          ) : (
            <>
              {tree.isFetching && !tree.isLoading && <p className="muted small">Refreshing</p>}
              {tree.data && (
                <EpicTree
                  tree={tree.data}
                  selectedKey={selectedKey}
                  onSelect={setSelectedKey}
                  expanded={expanded}
                  onExpandedChange={setExpanded}
                  movedKey={movedKey}
                />
              )}
            </>
          )}
        </div>
        {selected && (
          <IssueDetailPanel key={selected.key} profileId={activeId} issue={selected} onClose={() => setSelectedKey("")} />
        )}
      </div>

      {isOpen("newIssue") && (
        <NewIssueModal
          onClose={closeModal}
          initialType="epic"
          onCreated={(key) => {
            setSelectedKey(key);
            setExpanded((prev) => new Set(prev).add(key));
          }}
        />
      )}
    </section>
  );
}
