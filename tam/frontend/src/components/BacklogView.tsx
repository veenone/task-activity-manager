import { useMemo, useState } from "react";
import { useProfile } from "@agile-suite/core";
import { ISSUE_TYPES } from "../api";
import type { IssueQuery, Profile, Settings } from "../api";
import { useIssues, useSprints } from "../queries/issues";
import { IssueTable } from "./IssueTable";
import { IssueDetailPanel } from "./IssueDetailPanel";
import { useModal } from "../modals";
import { NewIssueModal } from "./NewIssueModal";
import { ImportIssuesModal } from "./ImportIssuesModal";
import { useDebounced } from "../lib/useDebounced";

const PAGE_SIZE = 25;
const SEARCH_DELAY_MS = 250;

// BacklogView is the issue grid with its filter bar and pager. Filter and
// page state live here and reset in the same render the profile changes in.
// Selection is kept here too, so the detail panel can sit beside the table.
export function BacklogView() {
  const { activeId } = useProfile<Profile, Settings>();
  const [text, setText] = useState("");
  const [types, setTypes] = useState<string[]>([]);
  const [sprintId, setSprintId] = useState("");
  const [page, setPage] = useState(0);
  const [selectedKey, setSelectedKey] = useState("");

  // The filters belong to the profile they were set for, so a switch clears
  // them during the render that first sees the new id. An effect would be one
  // commit too late: a query would go out pairing the new profile with the
  // old filters before the reset landed.
  const [filtersFor, setFiltersFor] = useState(activeId);
  if (filtersFor !== activeId) {
    setFiltersFor(activeId);
    setText("");
    setTypes([]);
    setSprintId("");
    setPage(0);
    setSelectedKey("");
  }
  const search = useDebounced(text, SEARCH_DELAY_MS, activeId);

  const query = useMemo<IssueQuery>(
    () => ({ text: search, types, sprintId, offset: page * PAGE_SIZE, limit: PAGE_SIZE }),
    [search, types, sprintId, page],
  );
  const issues = useIssues(activeId, query);
  const sprints = useSprints(activeId);
  const { isOpen, openModal, closeModal } = useModal();

  const total = issues.data?.total ?? 0;
  const rows = issues.data?.issues ?? [];
  const selected = rows.find((r) => r.key === selectedKey);
  const first = total === 0 ? 0 : page * PAGE_SIZE + 1;
  const last = Math.min(total, (page + 1) * PAGE_SIZE);
  const lastPage = Math.max(0, Math.ceil(total / PAGE_SIZE) - 1);
  const filtered = search !== "" || types.length > 0 || sprintId !== "";

  // Every filter change goes back to the first page, and it does so in the
  // same event as the filter itself so no render ever pairs a new filter with
  // the old offset.
  function toggleType(id: string) {
    setTypes((cur) => (cur.includes(id) ? cur.filter((t) => t !== id) : [...cur, id]));
    setPage(0);
  }

  return (
    <section className="backlog" aria-labelledby="view-title">
      <div className="filter-bar">
        <input
          type="search"
          className="detail-input filter-search"
          aria-label="Search issues"
          placeholder="Search summary, key, label"
          value={text}
          onChange={(e) => {
            setText(e.target.value);
            setPage(0);
          }}
        />
        <div className="type-filter" role="group" aria-label="Issue types">
          {ISSUE_TYPES.map((t) => (
            <button
              key={t.id}
              type="button"
              className={`chip chip-type chip-type-${t.id} chip-toggle${types.includes(t.id) ? " chip-on" : ""}`}
              aria-pressed={types.includes(t.id)}
              onClick={() => toggleType(t.id)}
            >
              {t.short}
            </button>
          ))}
        </div>
        <select
          className="detail-input filter-sprint"
          aria-label="Sprint"
          value={sprintId}
          onChange={(e) => {
            setSprintId(e.target.value);
            setPage(0);
          }}
        >
          <option value="">All sprints</option>
          {(sprints.data ?? []).map((s) => (
            <option key={s.id} value={s.id}>{s.name}</option>
          ))}
        </select>
        <button type="button" className="btn filter-import" disabled={!activeId} onClick={() => openModal("import")}>
          Import
        </button>
        <button type="button" className="btn btn-primary filter-new" disabled={!activeId} onClick={() => openModal("newIssue")}>
          + New
        </button>
      </div>

      <div className="backlog-body">
        <div className="backlog-grid">
          {issues.isError ? (
            <p className="error-text" data-testid="issues-error">Could not load issues: {issues.error.message}</p>
          ) : total === 0 && !issues.isPending ? (
            <p className="muted backlog-empty">
              {filtered
                ? "No issues match this filter."
                : "No issues cached yet. Use Sync in the topbar to pull this project's issues."}
            </p>
          ) : (
            <IssueTable issues={rows} selectedKey={selectedKey} onSelect={setSelectedKey} />
          )}
          <div className="pager">
            <span className="muted small">{`Showing ${first.toLocaleString()} to ${last.toLocaleString()} of ${total.toLocaleString()}`}</span>
            <span className="pager-buttons">
              <button type="button" className="btn" aria-label="Previous page" disabled={page === 0} onClick={() => setPage((p) => Math.max(0, p - 1))}>Prev</button>
              <span className="muted small">{`${page + 1} / ${lastPage + 1}`}</span>
              <button type="button" className="btn" aria-label="Next page" disabled={page >= lastPage} onClick={() => setPage((p) => Math.min(lastPage, p + 1))}>Next</button>
            </span>
          </div>
        </div>
        {selected && (
          <IssueDetailPanel key={selected.key} profileId={activeId} issue={selected} onClose={() => setSelectedKey("")} />
        )}
      </div>

      {isOpen("newIssue") && (
        <NewIssueModal
          onClose={closeModal}
          onCreated={(key) => {
            setPage(0);
            setSelectedKey(key);
          }}
        />
      )}

      {isOpen("import") && (
        <ImportIssuesModal
          onClose={closeModal}
          onImported={(keys) => {
            setPage(0);
            setSelectedKey(keys[0] ?? "");
          }}
        />
      )}
    </section>
  );
}
