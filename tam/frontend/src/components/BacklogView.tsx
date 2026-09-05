import { useEffect, useMemo, useState } from "react";
import { useProfile } from "@agile-suite/core";
import { ISSUE_TYPES } from "../api";
import type { IssueQuery, Profile, Settings } from "../api";
import { useIssues, useSprints } from "../queries/issues";
import { IssueTable } from "./IssueTable";

const PAGE_SIZE = 25;
const SEARCH_DELAY_MS = 250;
// The UI is English only and the mockup groups thousands with a comma, so the
// pager formats its counts against a fixed locale rather than the machine's
// regional settings, which would otherwise render 1,248 as 1.248.
const COUNT_LOCALE = "en-US";

// useDebounced returns value after it has stopped changing for delay ms, so
// typing in the search box does not query the backend on every keystroke.
function useDebounced<T>(value: T, delay: number): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const t = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(t);
  }, [value, delay]);
  return debounced;
}

// BacklogView is the issue grid with its filter bar and pager. Filter and
// page state live here and reset when the profile changes. Selection is
// kept here too, so the detail panel can sit beside the table.
export function BacklogView() {
  const { activeId } = useProfile<Profile, Settings>();
  const [text, setText] = useState("");
  const [types, setTypes] = useState<string[]>([]);
  const [sprintId, setSprintId] = useState("");
  const [page, setPage] = useState(0);
  const [selectedKey, setSelectedKey] = useState("");
  const search = useDebounced(text, SEARCH_DELAY_MS);

  useEffect(() => {
    setText("");
    setTypes([]);
    setSprintId("");
    setPage(0);
    setSelectedKey("");
  }, [activeId]);

  // A filter change goes back to the first page.
  useEffect(() => {
    setPage(0);
  }, [search, types, sprintId]);

  const query = useMemo<IssueQuery>(
    () => ({ text: search, types, sprintId, offset: page * PAGE_SIZE, limit: PAGE_SIZE }),
    [search, types, sprintId, page],
  );
  const issues = useIssues(activeId, query);
  const sprints = useSprints(activeId);

  const total = issues.data?.total ?? 0;
  const rows = issues.data?.issues ?? [];
  const first = total === 0 ? 0 : page * PAGE_SIZE + 1;
  const last = Math.min(total, (page + 1) * PAGE_SIZE);
  const lastPage = Math.max(0, Math.ceil(total / PAGE_SIZE) - 1);
  const filtered = search !== "" || types.length > 0 || sprintId !== "";

  function toggleType(id: string) {
    setTypes((cur) => (cur.includes(id) ? cur.filter((t) => t !== id) : [...cur, id]));
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
          onChange={(e) => setText(e.target.value)}
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
          onChange={(e) => setSprintId(e.target.value)}
        >
          <option value="">All sprints</option>
          {(sprints.data ?? []).map((s) => (
            <option key={s.id} value={s.id}>{s.name}</option>
          ))}
        </select>
        <span className="muted small filter-note">Read-only until plan 1b</span>
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
            <span className="muted small">{`Showing ${first.toLocaleString(COUNT_LOCALE)} to ${last.toLocaleString(COUNT_LOCALE)} of ${total.toLocaleString(COUNT_LOCALE)}`}</span>
            <span className="pager-buttons">
              <button type="button" className="btn" aria-label="Previous page" disabled={page === 0} onClick={() => setPage((p) => Math.max(0, p - 1))}>Prev</button>
              <span className="muted small">{`${page + 1} / ${lastPage + 1}`}</span>
              <button type="button" className="btn" aria-label="Next page" disabled={page >= lastPage} onClick={() => setPage((p) => Math.min(lastPage, p + 1))}>Next</button>
            </span>
          </div>
        </div>
        {/* The detail panel arrives in the next task and mounts here. */}
      </div>
    </section>
  );
}
