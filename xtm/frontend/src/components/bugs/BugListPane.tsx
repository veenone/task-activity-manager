import { errMsg } from "../../api";
import { Pager } from "../Pager";
import { SortControl } from "../SortControl";
import type { BugsState } from "./useBugsState";
import type { BugsActions } from "./useBugsActions";

// BugListPane is the left side: the bulk actions, filter, test-linkage pills,
// sort, select-all controls, the paged bug list and its pager.
export function BugListPane({ s, a }: { s: BugsState; a: BugsActions }) {
  const { filter, setFilter, error, notice, selected, setSelected, checked, creating, setPage, pageSize, setPageSize, sortField, setSortField, sortDesc, setSortDesc, testFilter, setTestFilter, syncing, exporting, bugsQuery, bugs, bugTestsQuery, unionTestKeys, testLinkCounts, shown, safePage, paged, allPageChecked, somePageChecked, allShownChecked, someShownChecked, toggleChecked, toggleSelectAll, toggleSelectAllShown } = s;
  const { syncBugs, exportBugs, createExecFromBugs, openAddToExec } = a;
  return (
    <>
      <div className="bugs-md-list">
        <div className="bugs-md-head">
          <span className="bugs-md-title">Bugs</span>
          <div className="bugs-md-actions">
            <button
              className="btn"
              onClick={createExecFromBugs}
              disabled={creating || unionTestKeys.length === 0}
              title={
                checked.size === 0
                  ? "Tick one or more bugs to create a Test Execution from their linked tests"
                  : unionTestKeys.length === 0
                    ? "The checked bugs have no linked tests"
                    : `Create a Test Execution containing the ${unionTestKeys.length} test${
                        unionTestKeys.length === 1 ? "" : "s"
                      } linked to the ${checked.size} checked bug${
                        checked.size === 1 ? "" : "s"
                      }`
              }
            >
              {creating
                ? "Creating…"
                : `Create Test Execution${
                    unionTestKeys.length > 0 ? ` (${unionTestKeys.length})` : ""
                  }`}
            </button>
            <button
              className="btn"
              onClick={openAddToExec}
              disabled={unionTestKeys.length === 0}
              title={
                checked.size === 0
                  ? "Tick one or more bugs to add their linked tests to an existing Test Execution"
                  : unionTestKeys.length === 0
                    ? "The checked bugs have no linked tests"
                    : `Add the ${unionTestKeys.length} test${
                        unionTestKeys.length === 1 ? "" : "s"
                      } linked to the ${checked.size} checked bug${
                        checked.size === 1 ? "" : "s"
                      } to an existing Test Execution`
              }
            >
              {`Add to execution${unionTestKeys.length > 0 ? ` (${unionTestKeys.length})` : ""}`}
            </button>
            <button
              className="btn"
              onClick={syncBugs}
              disabled={syncing}
              title="Refresh just the linked bugs from Jira, without syncing everything else"
            >
              {syncing ? "Syncing…" : "Sync"}
            </button>
            <button
              className="btn"
              onClick={exportBugs}
              disabled={
                exporting ||
                (checked.size === 0 && (!selected || selected.startsWith("NEW-")))
              }
              title={
                checked.size > 0
                  ? `Export ${checked.size} checked bug${checked.size === 1 ? "" : "s"} with run history to Excel`
                  : selected && !selected.startsWith("NEW-")
                    ? `Export ${selected} with run history to Excel`
                    : "Select or open a bug to export"
              }
            >
              {exporting ? "Exporting…" : "Export…"}
            </button>
          </div>
        </div>
        {(error || bugsQuery.error || bugTestsQuery.error) && (
          <div className="error-text">
            {error || errMsg(bugsQuery.error || bugTestsQuery.error)}
          </div>
        )}
        {notice && <p className="reqs-notice muted">{notice}</p>}
        <input
          className="search bugs-md-filter"
          placeholder="Filter bugs by key, summary, project, status…"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        />
        <div className="bugs-md-filter-row">
          <div className="filter-pill-row">
            {(
              [
                { value: "all", label: "All bugs" },
                { value: "with", label: "With tests" },
                { value: "without", label: "Without tests" },
              ] as const
            ).map(({ value, label }) => (
              <button
                key={value}
                className={`filter-pill${testFilter === value ? " filter-pill-active" : ""}`}
                onClick={() => setTestFilter(value)}
                title={`Filter by test linkage: ${label}`}
              >
                {label} {testLinkCounts[value]}
              </button>
            ))}
          </div>
          <SortControl
            fields={[
              { value: "key", label: "Key" },
              { value: "status", label: "Status" },
              { value: "project", label: "Project" },
              { value: "priority", label: "Priority" },
            ]}
            field={sortField}
            desc={sortDesc}
            onChange={(f, d) => {
              setSortField(f);
              setSortDesc(d);
            }}
          />
        </div>
        {shown.length === 0 ? (
          <p className="muted bugs-md-empty">
            {bugs.length === 0
              ? "No bugs linked to this profile's tests yet. File one from a failed test in a Test Execution, or try a demo profile."
              : "No bugs match the filter."}
          </p>
        ) : (
          <>
            <div className="bugs-md-select-all-row">
              <label className="bugs-md-select-all">
                <input
                  type="checkbox"
                  checked={allPageChecked}
                  ref={(el) => {
                    if (el) el.indeterminate = somePageChecked;
                  }}
                  onChange={toggleSelectAll}
                  title={allPageChecked ? "Deselect all on this page" : "Select all on this page"}
                />
                {allPageChecked ? "Deselect page" : "Select page"}
              </label>
              <label className="bugs-md-select-all">
                <input
                  type="checkbox"
                  checked={allShownChecked}
                  ref={(el) => {
                    if (el) el.indeterminate = someShownChecked;
                  }}
                  onChange={toggleSelectAllShown}
                  title={
                    allShownChecked
                      ? `Deselect all ${shown.length} bug${shown.length === 1 ? "" : "s"}`
                      : `Select all ${shown.length} bug${shown.length === 1 ? "" : "s"} matching the filter (every page)`
                  }
                />
                {allShownChecked ? `Deselect all (${shown.length})` : `Select all (${shown.length})`}
              </label>
              {checked.size > 0 && (
                <span className="muted bugs-md-select-count">
                  ({checked.size} total selected)
                </span>
              )}
            </div>
            <ul className="bugs-md-items">
              {paged.map((b) => (
                <li
                  key={b.key}
                  className={`bugs-md-item row-stretch${b.key === selected ? " bugs-md-item-selected" : ""}`}
                >
                  <div className="bugs-md-item-top">
                    <input
                      type="checkbox"
                      className="bugs-md-check"
                      checked={checked.has(b.key)}
                      title="Include this bug's linked tests when creating a Test Execution"
                      onChange={() => toggleChecked(b.key)}
                    />
                    <button
                      className="row-select mono bugs-md-key"
                      aria-current={b.key === selected ? "true" : undefined}
                      aria-label={`${b.key} ${b.summary}`}
                      onClick={() => setSelected(b.key)}
                    >
                      {b.key}
                    </button>
                    <span className="muted">{b.projectKey}</span>
                    {b.status && <span className="status-pill">{b.status}</span>}
                  </div>
                  <div className="bugs-md-item-summary">
                    {b.summary || "(no summary)"}
                  </div>
                  <div className="bugs-md-item-meta muted">
                    {b.priority} ·{" "}
                    {b.testKeys.length === 0 ? (
                      <span className="bugs-md-no-tests">no tests</span>
                    ) : (
                      <span className="bugs-md-test-count">{b.testKeys.length} test{b.testKeys.length === 1 ? "" : "s"}</span>
                    )}
                  </div>
                </li>
              ))}
            </ul>
            <div className="bugs-md-pager">
              <Pager
                compact
                page={safePage}
                pageSize={pageSize}
                total={shown.length}
                onPage={setPage}
                pageSizeOptions={[5, 10, 15, 25, 50]}
                onPageSize={(n) => {
                  setPageSize(n);
                  setPage(0);
                }}
              />
            </div>
          </>
        )}
      </div>
    </>
  );
}
