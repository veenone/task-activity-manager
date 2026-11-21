import { useEffect, useMemo, useState } from "react";
import { Pager, errMsg, useProfile, useViewState } from "@agile-suite/core";
import { ExportBacklog, GRID_COLUMNS } from "../api";
import type { Issue, IssueQuery, Profile, Settings, SortColumn } from "../api";
import { useIssues, useSprints } from "../queries/issues";
import { useOpenSprints } from "../queries/boards";
import { IssueTable } from "./IssueTable";
import { IssueDetailPanel } from "./IssueDetailPanel";
import { useModal } from "../modals";
import { NewIssueModal } from "./NewIssueModal";
import { ImportIssuesModal } from "./ImportIssuesModal";
import { useDebounced } from "../lib/useDebounced";
import { useSubtaskType } from "../queries/people";
import { IssueFilters } from "./IssueFilters";
import { NO_FILTER, isFiltered, queryFields } from "../lib/issueFilter";
import type { IssueFilter } from "../lib/issueFilter";
import { PERM_CREATE_ISSUES } from "../api/permissions";
import { useDenial } from "../queries/permissions";
import { useGuardedSelect } from "../lib/unsavedEdit";

// The page sizes the pager offers. 25 stays the default so the first render
// is unchanged; the larger steps are for planning, where paging 25 at a time
// through a project's backlog is the wrong interaction.
const PAGE_SIZES = [25, 50, 100, 250];
const SEARCH_DELAY_MS = 250;

interface IssueListViewProps {
  // Keys this view's filter, sort, page and selection in the view-state
  // store, so each tab gets them back when it is shown again (#524).
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
// and page state live in the view-state store, so they outlast the tab, and
// reset in the render the profile changes in. Selection is kept here too,
// so the detail panel can sit beside the table.
export function IssueListView({ viewId, label, baseQuery, showCreate, showImport, emptyNote, onPage }: IssueListViewProps) {
  const { activeId, activeProfile } = useProfile<Profile, Settings>();
  const createDenied = useDenial(activeId)(PERM_CREATE_ISSUES);
  const [text, setText] = useViewState(activeId, viewId, "text", "");
  // Type, status and assignee are one object because IssueFilters sets them
  // as one, which is also what makes a profile switch one reset rather than
  // three that could drift apart.
  const [filter, setFilter] = useViewState<IssueFilter>(activeId, viewId, "filter", NO_FILTER);
  const [sprintId, setSprintId] = useViewState(activeId, viewId, "sprintId", "");
  const [page, setPage] = useViewState(activeId, viewId, "page", 0);
  // Stored under no profile, so a profile switch, which clears the
  // profile's own view state, leaves it alone (see the reset below).
  const [pageSize, setPageSize] = useViewState("", viewId, "pageSize", PAGE_SIZES[0]);
  const [selectedKey, setSelectedKey] = useViewState(activeId, viewId, "selectedKey", "");
  // "" is the store's rank order, the right default for a backlog. A column
  // is sorted ascending, then descending, then back to rank, so the default
  // is always one more click away rather than unreachable.
  const [sort, setSort] = useViewState<SortColumn | "">(activeId, viewId, "sort", "");
  const [desc, setDesc] = useViewState(activeId, viewId, "desc", false);

  // The filters belong to the profile they were set for, so a switch clears
  // them during the render that first sees the new id. An effect would be one
  // commit too late: a query would go out pairing the new profile with the
  // old filters before the reset landed.
  const [filtersFor, setFiltersFor] = useState(activeId);
  if (filtersFor !== activeId) {
    setFiltersFor(activeId);
    setText("");
    setFilter(NO_FILTER);
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
    () => ({ ...baseQuery, ...queryFields(filter), text: search, sprintId, offset: page * pageSize, limit: pageSize, sort, desc, groupSubtasks: true }),
    [baseQuery, search, filter, sprintId, page, pageSize, sort, desc],
  );
  const issues = useIssues(activeId, query);
  // The export's own state. It is not a query: nothing is cached, the
  // user asked for a file once, and what comes back is where it went.
  const [exporting, setExporting] = useState(false);
  const [exported, setExported] = useState("");
  const [exportError, setExportError] = useState("");

  async function exportBacklog(q: IssueQuery) {
    setExporting(true);
    setExportError("");
    setExported("");
    try {
      // An empty path is a cancelled dialog, which is the user saying no
      // and nothing to announce.
      setExported(await ExportBacklog(activeId, q));
    } catch (e) {
      setExportError(errMsg(e));
    } finally {
      setExporting(false);
    }
  }
  useEffect(() => {
    if (issues.data) onPage?.(issues.data.total, issues.data.issues);
  }, [issues.data, onPage]);
  const sprints = useSprints(activeId);
  // The detail panel's own Sprint field, not the filter bar above: away from
  // a board this is the only sprint list an issue here can be moved through.
  const openSprints = useOpenSprints(activeId);
  const subtaskType = useSubtaskType(activeId);
  const { isOpen, openModal, closeModal } = useModal();
  // Another row asks first when the panel holds an unsaved edit (#521).
  const selectIssue = useGuardedSelect(selectedKey, setSelectedKey);

  const total = issues.data?.total ?? 0;
  const rows = issues.data?.issues ?? [];
  // The panel is keyed on the selection, not on the page: a sort or a page
  // step moves the open issue off the rows the grid holds, and the panel
  // stays on the last row it saw for that key until the issue comes back
  // into view (#525). The row is captured during render, like the profile
  // reset above, so no commit shows the panel closed.
  const inRows = rows.find((r) => r.key === selectedKey);
  const [held, setHeld] = useState<Issue | undefined>(undefined);
  if (inRows && inRows !== held) setHeld(inRows);
  const selected = inRows ?? (held?.key === selectedKey ? held : undefined);
  const filtered = search !== "" || sprintId !== "" || isFiltered(filter);

  // Every filter change goes back to the first page, and it does so in the
  // same event as the filter itself so no render ever pairs a new filter with
  // the old offset.
  function changeFilter(next: IssueFilter) {
    setFilter(next);
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
    <section className="backlog" aria-label={label} data-tour={`${viewId}-body`}>
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
        <IssueFilters value={filter} onChange={changeFilter} />
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
        <button
          type="button"
          className="btn filter-export"
          disabled={!activeId || exporting}
          // The export is of the filter, so the paging goes: a planning
          // session wants the whole list it narrowed to, not the page.
          onClick={() => void exportBacklog({ ...query, offset: 0, limit: 0 })}
        >
          {exporting ? "Exporting" : "Export"}
        </button>
        {showImport && (
          <button type="button" className="btn filter-import" disabled={!activeId || !!createDenied} title={createDenied || undefined} onClick={() => openModal("import")}>
            Import
          </button>
        )}
        {showCreate && (
          <button type="button" className="btn btn-primary filter-new" disabled={!activeId || !!createDenied} title={createDenied || undefined} onClick={() => openModal("newIssue")}>
            + New
          </button>
        )}
      </div>

      {exported && <p className="muted small filter-note">Saved to {exported}</p>}
      {exportError && <p className="error-text filter-note">The backlog could not be exported: {exportError}</p>}

      <div className="backlog-body">
        <div className="backlog-grid">
          {issues.isError ? (
            <p className="error-text" data-testid="issues-error">Could not load issues: {issues.error.message}</p>
          ) : issues.isPending && activeId !== "" ? (
            // Without a profile the query is disabled and pending for good,
            // so the line is only drawn for a read that is under way.
            <p className="muted backlog-empty">Reading the issues</p>
          ) : total === 0 && !issues.isPending ? (
            <p className="muted backlog-empty">
              {filtered ? "No issues match this filter." : emptyNote}
            </p>
          ) : (
            <IssueTable key={activeId} issues={rows} subtaskLabel={subtaskType.data} selectedKey={selectedKey} onSelect={selectIssue} sort={sort} desc={desc} onSort={toggleSort} />
          )}
          <Pager
            page={page}
            pageSize={pageSize}
            total={total}
            onPage={setPage}
            onPageSize={(n) => {
              setPageSize(n);
              setPage(0);
            }}
            pageSizeOptions={PAGE_SIZES}
            sizeName="Issue groups per page"
            sizeLabel={(n) => `${n} groups / page`}
          />
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
