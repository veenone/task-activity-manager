import { useEffect, useMemo, useState } from "react";
import { useProfile } from "@agile-suite/core";
import { GRID_COLUMNS, ISSUE_TYPES } from "../api";
import type { Issue, IssueQuery, Profile, Settings, SortColumn } from "../api";
import { useIssues, useSprints } from "../queries/issues";
import { useOpenSprints } from "../queries/boards";
import { IssueTable } from "./IssueTable";
import { IssueDetailPanel } from "./IssueDetailPanel";
import { useModal } from "../modals";
import { NewIssueModal } from "./NewIssueModal";
import { ImportIssuesModal } from "./ImportIssuesModal";
import { useDebounced } from "../lib/useDebounced";
import { useProjectTypes, useSubtaskType } from "../queries/people";

// The page sizes the pager offers. 25 stays the default so the first render
// is unchanged; the larger steps are for planning, where paging 25 at a time
// through a project's backlog is the wrong interaction.
const PAGE_SIZES = [25, 50, 100, 250];
const SEARCH_DELAY_MS = 250;

// typeChips is what the filter bar offers: one chip per type the project
// has, in the order Jira lists them, keyed by the value the sync stored in
// the type column. A type TAM models is that logical type, under TAM's own
// short label; a type it does not is the project's own name, on the neutral
// chip, the rule #67 set for the New issue dialog.
//
// Two Jira types can mean the same logical type ("Task" and "Todo" both do),
// and two chips filtering for the same rows is a filter bar lying about
// having two filters, so the first one wins. A project whose types have
// never been synced falls back to the six TAM models, which is what the bar
// offered before it could know better.
function typeChips(projectTypes: { name: string; logical: string }[] | undefined) {
  if (!projectTypes || projectTypes.length === 0) {
    return ISSUE_TYPES.map((t) => ({ id: t.id, label: t.short, title: t.label, modelled: true }));
  }
  const chips = new Map<string, { id: string; label: string; title: string; modelled: boolean }>();
  for (const t of projectTypes) {
    const id = t.logical || t.name;
    const short = ISSUE_TYPES.find((x) => x.id === t.logical)?.short;
    if (!chips.has(id)) chips.set(id, { id, label: short ?? t.name, title: t.name, modelled: !!short });
  }
  return [...chips.values()];
}

interface IssueListViewProps {
  // Namespaces this instance's DOM ids (the pager's label/input pairs), so
  // two mounted tabs never collide on the same id. Filter, sort and page
  // state need no namespacing of their own: each tab is its own component
  // instance, so its useState is already private to it.
  viewId: string;
  label: string;
  // Merged under the component's own filter state, so baseQuery only fills
  // in fields the tab drives on its own (see the query useMemo below).
  baseQuery?: Partial<IssueQuery>;
  showCreate?: boolean;
  showImport?: boolean;
  emptyNote: string;
  // Fired with this page's total and rows after every successful load, so a
  // caller that wants its own summary above the grid (Assigned to me's
  // header and stale-rows note) can build it from the exact query this view
  // already ran, instead of paying for a second one.
  onPage?: (total: number, rows: Issue[]) => void;
}

// IssueListView is the issue grid with its filter bar and pager, generic over
// whatever a tab (Backlog, Assigned to me, ...) narrows the query with. Filter
// and page state live here and reset in the same render the profile changes
// in. Selection is kept here too, so the detail panel can sit beside the
// table.
export function IssueListView({ viewId, label, baseQuery, showCreate, showImport, emptyNote, onPage }: IssueListViewProps) {
  const { activeId, activeProfile } = useProfile<Profile, Settings>();
  const projectTypes = useProjectTypes(activeId);
  const chips = useMemo(() => typeChips(projectTypes.data), [projectTypes.data]);
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
    () => ({ ...baseQuery, text: search, types, sprintId, offset: page * pageSize, limit: pageSize, sort, desc, groupSubtasks: true }),
    [baseQuery, search, types, sprintId, page, pageSize, sort, desc],
  );
  const issues = useIssues(activeId, query);
  useEffect(() => {
    if (issues.data) onPage?.(issues.data.total, issues.data.issues);
  }, [issues.data, onPage]);
  const sprints = useSprints(activeId);
  // The detail panel's own Sprint field, not the filter bar above: away from
  // a board this is the only sprint list an issue here can be moved through.
  const openSprints = useOpenSprints(activeId);
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
    <section className="backlog" aria-label={label}>
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
          {chips.map((t) => (
            <button
              key={t.id}
              type="button"
              className={`chip chip-type chip-type-${t.modelled ? t.id : "none"} chip-toggle${types.includes(t.id) ? " chip-on" : ""}`}
              aria-pressed={types.includes(t.id)}
              title={t.title}
              onClick={() => toggleType(t.id)}
            >
              {t.label}
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
        {showImport && (
          <button type="button" className="btn filter-import" disabled={!activeId} onClick={() => openModal("import")}>
            Import
          </button>
        )}
        {showCreate && (
          <button type="button" className="btn btn-primary filter-new" disabled={!activeId} onClick={() => openModal("newIssue")}>
            + New
          </button>
        )}
      </div>

      <div className="backlog-body">
        <div className="backlog-grid">
          {issues.isError ? (
            <p className="error-text" data-testid="issues-error">Could not load issues: {issues.error.message}</p>
          ) : total === 0 && !issues.isPending ? (
            <p className="muted backlog-empty">
              {filtered ? "No issues match this filter." : emptyNote}
            </p>
          ) : (
            <IssueTable key={activeId} issues={rows} subtaskLabel={subtaskType.data} selectedKey={selectedKey} onSelect={setSelectedKey} sort={sort} desc={desc} onSort={toggleSort} />
          )}
          <div className="pager">
            <label className="sr-only" htmlFor={`pager-size-${viewId}`}>Issue groups per page</label>
            <select
              id={`pager-size-${viewId}`}
              className="detail-input pager-size"
              value={pageSize}
              onChange={(e) => {
                setPageSize(Number(e.target.value));
                goToPage(0);
              }}
            >
              {PAGE_SIZES.map((n) => (
                <option key={n} value={n}>{`${n} groups / page`}</option>
              ))}
            </select>
            <span className="pager-buttons">
              <button type="button" className="btn pager-step" aria-label="First page" title="First page" disabled={page === 0} onClick={() => goToPage(0)}>«</button>
              <button type="button" className="btn pager-step" aria-label="Previous page" title="Previous page" disabled={page === 0} onClick={() => goToPage(page - 1)}>‹</button>
              <span className="pager-page">
                <label className="muted small" htmlFor={`pager-page-${viewId}`}>Page</label>
                {/* Typing a page number is the only way to cross a long
                    backlog in one move; the step buttons cover the rest. */}
                <input
                  id={`pager-page-${viewId}`}
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
          <IssueDetailPanel
            key={selected.key}
            profileId={activeId}
            issue={selected}
            jiraUrl={activeProfile?.jiraUrl}
            // undefined here covers a read still in flight and one that
            // failed alike (retry is off, so a failure is permanent for the
            // session): both fall back to the panel's read-only branch with
            // nothing said, a known limit rather than a bug.
            sprints={openSprints.data}
            // The profile-wide list, so an empty one really does mean this
            // profile has never synced a board, unlike a board's own list.
            emptyNote="No sprints yet, sync a board first"
            onClose={() => setSelectedKey("")}
          />
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
