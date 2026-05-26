import { useEffect, useState } from "react";
import { ListTests, errMsg } from "../api";
import type { TestPage, TestQuery, PendingChange } from "../api";

interface Props {
  profileId: string;
  folderId: string;
  refreshKey: number;
  selectedKey: string | null;
  pendingByTestKey: Map<string, PendingChange[]>;
  selectedSet: Set<string>;
  onSelect: (key: string) => void;
  onToggleSelect: (key: string) => void;
  onToggleSelectPage: (keys: string[]) => void;
}

const PAGE_SIZE = 100;

type SortCol = "key" | "summary" | "status" | "updated";

export function TestTable({
  profileId,
  folderId,
  refreshKey,
  selectedKey,
  pendingByTestKey,
  selectedSet,
  onSelect,
  onToggleSelect,
  onToggleSelectPage,
}: Props) {
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [status, setStatus] = useState("");
  const [sortBy, setSortBy] = useState<SortCol>("key");
  const [desc, setDesc] = useState(false);
  const [offset, setOffset] = useState(0);

  const [page, setPage] = useState<TestPage>({ tests: [], total: 0 });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(search), 250);
    return () => clearTimeout(t);
  }, [search]);

  useEffect(() => {
    setOffset(0);
  }, [debouncedSearch, status, folderId, sortBy, desc, profileId]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError("");
    const q: TestQuery = {
      search: debouncedSearch,
      status: status.trim(),
      folderId,
      sortBy,
      desc,
      limit: PAGE_SIZE,
      offset,
    };
    ListTests(profileId, q)
      .then((p) => {
        if (!cancelled) setPage(p);
      })
      .catch((e) => {
        if (!cancelled) setError(errMsg(e));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [
    profileId,
    debouncedSearch,
    status,
    folderId,
    sortBy,
    desc,
    offset,
    refreshKey,
  ]);

  function toggleSort(col: SortCol) {
    if (sortBy === col) {
      setDesc((d) => !d);
    } else {
      setSortBy(col);
      setDesc(false);
    }
  }

  const from = page.total === 0 ? 0 : offset + 1;
  const to = Math.min(offset + PAGE_SIZE, page.total);

  const pageKeys = page.tests.map((t) => t.key);
  const allOnPageSelected =
    pageKeys.length > 0 && pageKeys.every((k) => selectedSet.has(k));
  const someOnPageSelected =
    !allOnPageSelected && pageKeys.some((k) => selectedSet.has(k));

  return (
    <div className="testtable">
      <div className="filters">
        <input
          className="search"
          placeholder="Search key, summary, description…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <input
          className="status-filter"
          placeholder="Status (exact match)"
          value={status}
          onChange={(e) => setStatus(e.target.value)}
        />
        <span className="muted count">
          {loading
            ? "Loading…"
            : `${from}–${to} of ${page.total.toLocaleString()}`}
        </span>
        <div className="pager">
          <button
            className="btn"
            disabled={offset === 0 || loading}
            onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
          >
            ‹ Prev
          </button>
          <button
            className="btn"
            disabled={offset + PAGE_SIZE >= page.total || loading}
            onClick={() => setOffset(offset + PAGE_SIZE)}
          >
            Next ›
          </button>
        </div>
      </div>

      {error && <div className="error-text table-error">{error}</div>}

      <div className="table-scroll">
        <table>
          <thead>
            <tr>
              <th className="select-col">
                <input
                  type="checkbox"
                  checked={allOnPageSelected}
                  ref={(el) => {
                    if (el) el.indeterminate = someOnPageSelected;
                  }}
                  disabled={pageKeys.length === 0}
                  onChange={() => onToggleSelectPage(pageKeys)}
                  title={
                    allOnPageSelected
                      ? "Clear page selection"
                      : "Select all on this page"
                  }
                />
              </th>
              <SortHeader col="key" label="Key" sortBy={sortBy} desc={desc} onSort={toggleSort} />
              <SortHeader col="summary" label="Summary" sortBy={sortBy} desc={desc} onSort={toggleSort} />
              <SortHeader col="status" label="Status" sortBy={sortBy} desc={desc} onSort={toggleSort} />
              <th>Priority</th>
              <th>Labels</th>
              <SortHeader col="updated" label="Updated" sortBy={sortBy} desc={desc} onSort={toggleSort} />
            </tr>
          </thead>
          <tbody>
            {page.tests.map((t) => {
              const hasPending = pendingByTestKey.has(t.key);
              const isSelected = selectedSet.has(t.key);
              return (
                <tr
                  key={t.key}
                  className={
                    (t.key === selectedKey ? "row-selected " : "") +
                    (isSelected ? "row-checked" : "")
                  }
                  onClick={() => onSelect(t.key)}
                >
                  <td
                    className="select-col"
                    onClick={(e) => e.stopPropagation()}
                  >
                    <input
                      type="checkbox"
                      checked={isSelected}
                      onChange={() => onToggleSelect(t.key)}
                    />
                  </td>
                  <td className="mono">
                    {hasPending && (
                      <span className="row-dirty-dot" title="Pending edits">
                        ●
                      </span>
                    )}
                    {t.key}
                  </td>
                  <td className="summary-cell">{t.summary}</td>
                  <td>
                    {t.status ? (
                      <span className="status-pill">{t.status}</span>
                    ) : (
                      <span className="muted">—</span>
                    )}
                  </td>
                  <td>{t.priority || "—"}</td>
                  <td className="labels-cell">
                    {t.labels && t.labels.length > 0 ? (
                      t.labels.map((l) => (
                        <span key={l} className="label-chip">
                          {l}
                        </span>
                      ))
                    ) : (
                      <span className="muted">—</span>
                    )}
                  </td>
                  <td className="muted">{formatDate(t.updated)}</td>
                </tr>
              );
            })}
            {!loading && page.tests.length === 0 && (
              <tr>
                <td colSpan={7} className="empty-row muted">
                  {page.total === 0 &&
                  debouncedSearch === "" &&
                  status.trim() === "" &&
                  folderId === ""
                    ? "No tests yet — run a sync to pull them from Jira."
                    : "No tests match the current filter."}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function SortHeader({
  col,
  label,
  sortBy,
  desc,
  onSort,
}: {
  col: SortCol;
  label: string;
  sortBy: SortCol;
  desc: boolean;
  onSort: (c: SortCol) => void;
}) {
  const active = sortBy === col;
  return (
    <th className="sortable" onClick={() => onSort(col)}>
      {label}
      <span className="sort-caret">{active ? (desc ? " ▼" : " ▲") : ""}</span>
    </th>
  );
}

function formatDate(s: string): string {
  if (!s) return "—";
  const d = new Date(s);
  return isNaN(d.getTime()) ? s : d.toLocaleDateString();
}
