import { Pager } from "../Pager";
import { SortControl } from "../SortControl";
import type { PreconditionsState } from "./usePreconditionsState";
import type { PreconditionsActions } from "./usePreconditionsActions";

// PreconditionList is the left side: filter, Sync, sort, New and Import, the
// usage pills, and the paged list.
export function PreconditionList({ s, a }: { s: PreconditionsState; a: PreconditionsActions }) {
  const { canSync, selected, setSelected, filter, setFilter, usageFilter, setUsageFilter, sortField, setSortField, sortDesc, setSortDesc, notice, setShowCreate, setShowImport, syncing, setListPage, listPageSize, setListPageSize, list, loading, usageCounts, filtered, listSafePage, pageList } = s;
  const { syncPreconditions } = a;
  return (
    <>
      <aside className="precond-list">
        <div className="precond-list-head">
          <input
            className="precond-search"
            data-tour="preconditions-tools"
            placeholder="Filter preconditions…"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          />
          <button
            className="btn"
            onClick={syncPreconditions}
            disabled={!canSync}
            aria-busy={syncing}
            title="Refresh just the preconditions from Jira, without syncing everything else"
          >
            {syncing ? "Syncing…" : "Sync"}
          </button>
          <SortControl
            fields={[
              { value: "key", label: "Key" },
              { value: "type", label: "Type" },
              { value: "status", label: "Status" },
              { value: "usage", label: "Usage" },
            ]}
            field={sortField}
            desc={sortDesc}
            onChange={(f, d) => {
              setSortField(f);
              setSortDesc(d);
            }}
          />
          <button
            className="btn btn-primary precond-new"
            onClick={() => setShowCreate(true)}
          >
            New precondition
          </button>
          <button className="btn precond-new" onClick={() => setShowImport(true)}>
            Import…
          </button>
        </div>
        <div className="filter-pill-row">
          {(
            [
              { value: "all", label: "All" },
              { value: "with", label: "With tests" },
              { value: "without", label: "Without tests" },
            ] as const
          ).map(({ value, label }) => (
            <button
              key={value}
              className={`filter-pill${usageFilter === value ? " filter-pill-active" : ""}`}
              onClick={() => setUsageFilter(value)}
              title={`Filter by test usage: ${label}`}
            >
              {label} {usageCounts[value]}
            </button>
          ))}
        </div>
        {notice && <p className="precond-notice muted">{notice}</p>}

        {loading ? (
          <p className="muted precond-empty">Loading…</p>
        ) : list.length === 0 ? (
          <p className="muted precond-empty">
            No preconditions yet. Create one, or sync to pull them from Jira.
          </p>
        ) : filtered.length === 0 ? (
          <p className="muted precond-empty">No preconditions match.</p>
        ) : (
          <>
            <ul className="precond-items">
              {pageList.map((p) => (
                <li key={p.key}>
                  <button
                    className={`precond-item${p.key === selected ? " precond-item-active" : ""}`}
                    onClick={() => setSelected(p.key)}
                  >
                    <div className="precond-item-top">
                      <span className="mono precond-item-key">{p.key}</span>
                      <span
                        className={`pre-type-badge pre-type-${p.type.toLowerCase()}`}
                      >
                        {p.type || "—"}
                      </span>
                    </div>
                    <div className="precond-item-summary">
                      {p.summary || "(untitled)"}
                    </div>
                    <div className="precond-item-meta muted">
                      {p.status && (
                        <span className="status-pill">{p.status}</span>
                      )}
                      {p.testCount} test{p.testCount === 1 ? "" : "s"}
                    </div>
                  </button>
                </li>
              ))}
            </ul>
            {filtered.length > 0 && (
              <Pager
                compact
                page={listSafePage}
                pageSize={listPageSize}
                total={filtered.length}
                onPage={setListPage}
                onPageSize={(n) => {
                  setListPageSize(n);
                  setListPage(0);
                }}
              />
            )}
          </>
        )}
      </aside>
    </>
  );
}
