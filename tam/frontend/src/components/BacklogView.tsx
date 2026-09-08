import { useMemo, useState } from "react";
import { useProfile } from "@agile-suite/core";
import { GRID_COLUMNS, ISSUE_TYPES } from "../api";
import type { IssueQuery, Profile, Settings, SortColumn } from "../api";
import { useIssues, useSprints } from "../queries/issues";
import { IssueTable } from "./IssueTable";
import { IssueDetailPanel } from "./IssueDetailPanel";
import { useModal } from "../modals";
import { NewIssueModal } from "./NewIssueModal";
import { ImportIssuesModal } from "./ImportIssuesModal";
import { useDebounced } from "../lib/useDebounced";
import { useSubtaskType } from "../queries/people";

// The page sizes the pager offers. 25 stays the default so the first render
// is unchanged; the larger steps are for planning, where paging 25 at a time
// through a project's backlog is the wrong interaction.
const PAGE_SIZES = [25, 50, 100, 250];
const SEARCH_DELAY_MS = 250;

// BacklogView is the issue grid with its filter bar and pager. Filter and
// page state live here and reset in the same render the profile changes in.
// Selection is kept here too, so the detail panel can sit beside the table.
export function BacklogView() {
  const { activeId, activeProfile } = useProfile<Profile, Settings>();
  const [text, setText] = useState("");
  const [types, setTypes] = useState<string[]>([]);
  const [sprintId, setSprintId] = useState("");
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState(PAGE_SIZES[0]);
  // What the user has typed into the page box, or null when they are not
  // editing it. The box cannot simply be bound to page + 1: clearing it would
  // snap the field straight back to "1", so typing "23" over it produced
  // "123". null and "" have to be different states for that reason. null
  // shows the real page, "" shows an empty box the user is mid-way through
  // typing into, and page stays where it was until a whole valid number
  // lands.
  const [pageDraft, setPageDraft] = useState<string | null>(null);
  const [selectedKey, setSelectedKey] = useState("");
  // "" is the store's rank order, the right default for a backlog. A column
  // is sorted ascending, then descending, then back to rank, so the default
  // is always one more click away rather than unreachable.
  const [sort, setSort] = useState<SortColumn | "">("");
  const [desc, setDesc] = useState(false);

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
    setSort("");
    setDesc(false);
  }
  // The page size is a reading preference, not a filter, so it survives a
  // profile switch; only the offset it applies to resets.
  const search = useDebounced(text, SEARCH_DELAY_MS, activeId);

  const query = useMemo<IssueQuery>(
    () => ({ text: search, types, sprintId, offset: page * pageSize, limit: pageSize, sort, desc }),
    [search, types, sprintId, page, pageSize, sort, desc],
  );
  const issues = useIssues(activeId, query);
  const sprints = useSprints(activeId);
  const subtaskType = useSubtaskType(activeId);
  const { isOpen, openModal, closeModal } = useModal();

  const total = issues.data?.total ?? 0;
  const rows = issues.data?.issues ?? [];
  const selected = rows.find((r) => r.key === selectedKey);
  const first = total === 0 ? 0 : page * pageSize + 1;
  const last = Math.min(total, (page + 1) * pageSize);
  const lastPage = Math.max(0, Math.ceil(total / pageSize) - 1);
  const filtered = search !== "" || types.length > 0 || sprintId !== "";

  // goToPage is every way of moving pages that is not the text box, so the
  // box's draft never survives a step button and show a stale number.
  function goToPage(n: number) {
    setPage(Math.min(lastPage, Math.max(0, n)));
    setPageDraft(null);
  }

  // Every filter change goes back to the first page, and it does so in the
  // same event as the filter itself so no render ever pairs a new filter with
  // the old offset.
  function toggleType(id: string) {
    setTypes((cur) => (cur.includes(id) ? cur.filter((t) => t !== id) : [...cur, id]));
    setPage(0);
  }

  // A header click cycles that column: first click in the direction the
  // column reads best, second click the other way, third back to rank order.
  // Like a filter change it returns to page one, in the same event, so no
  // render pairs a new order with the old offset.
  function toggleSort(column: SortColumn) {
    const first = GRID_COLUMNS.find((c) => c.id === column)?.firstClickDesc ?? false;
    if (sort !== column) {
      setSort(column);
      setDesc(first);
    } else if (desc !== first) {
      setSort("");
      setDesc(false);
    } else {
      setDesc(!first);
    }
    setPage(0);
  }

  return (
    <section className="backlog" aria-label="Backlog">
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
            <IssueTable issues={rows} subtaskLabel={subtaskType.data} selectedKey={selectedKey} onSelect={setSelectedKey} sort={sort} desc={desc} onSort={toggleSort} />
          )}
          <div className="pager">
            <label className="sr-only" htmlFor="pager-size">Rows per page</label>
            <select
              id="pager-size"
              className="detail-input pager-size"
              value={pageSize}
              onChange={(e) => {
                setPageSize(Number(e.target.value));
                goToPage(0);
              }}
            >
              {PAGE_SIZES.map((n) => (
                <option key={n} value={n}>{`${n} / page`}</option>
              ))}
            </select>
            <span className="pager-buttons">
              <button type="button" className="btn pager-step" aria-label="First page" title="First page" disabled={page === 0} onClick={() => goToPage(0)}>«</button>
              <button type="button" className="btn pager-step" aria-label="Previous page" title="Previous page" disabled={page === 0} onClick={() => goToPage(page - 1)}>‹</button>
              <span className="pager-page">
                <label className="muted small" htmlFor="pager-page">Page</label>
                {/* Typing a page number is the only way to cross a long
                    backlog in one move; the step buttons cover the rest. */}
                <input
                  id="pager-page"
                  type="number"
                  className="detail-input pager-page-input"
                  min={1}
                  max={lastPage + 1}
                  value={pageDraft ?? page + 1}
                  onChange={(e) => {
                    const raw = e.target.value;
                    setPageDraft(raw);
                    // An empty or out-of-range box is a half-typed number,
                    // not a request to jump to the edge of the backlog.
                    const n = Number(raw);
                    if (raw !== "" && Number.isFinite(n) && n >= 1 && n <= lastPage + 1) {
                      setPage(n - 1);
                    }
                  }}
                  onBlur={() => setPageDraft(null)}
                />
                <span className="muted small">{`of ${(lastPage + 1).toLocaleString()}`}</span>
              </span>
              <button type="button" className="btn pager-step" aria-label="Next page" title="Next page" disabled={page >= lastPage} onClick={() => goToPage(page + 1)}>›</button>
              <button type="button" className="btn pager-step" aria-label="Last page" title="Last page" disabled={page >= lastPage} onClick={() => goToPage(lastPage)}>»</button>
            </span>
            <span className="muted small pager-range">{`${first.toLocaleString()} to ${last.toLocaleString()} of ${total.toLocaleString()}`}</span>
          </div>
        </div>
        {selected && (
          <IssueDetailPanel key={selected.key} profileId={activeId} issue={selected} jiraUrl={activeProfile?.jiraUrl} onClose={() => setSelectedKey("")} />
        )}
      </div>

      {isOpen("newIssue") && (
        <NewIssueModal
          onClose={closeModal}
          // A draft started with an issue open almost always belongs where
          // that issue does.
          initialEpic={selected ? (selected.type === "epic" ? selected.key : selected.parentKey) : ""}
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
