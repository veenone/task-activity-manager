import { colStyle, SELECT_STYLE, COL_LABEL, COL_SORT } from "./columns";
import { renderCell, SortHeader } from "./cells";
import type { TestTableProps, TestTableState } from "./useTestTableState";
import type { TestTableActions } from "./useTestTableActions";

// Grid is the error line, the select-all-matching banner, and the virtualized
// table with its sortable header and keyboard row focus.
export function Grid({ s, a, p }: { s: TestTableState; a: TestTableActions; p: TestTableProps }) {
  const { selectedKey, selectedSet, onSelect, onToggleSelect, onToggleSelectPage, debouncedSearch, status, review, sortBy, desc, setFocusedIndex, error, selectingAll, selectAllError, page, loading, listError, callerKeys, scrollRef, visibleColumns, rowVirtualizer, tabbableIndex, gridMinWidth, pageKeys, allOnPageSelected, someOnPageSelected, canSelectAllMatching, moveFocus, toggleSort } = s;
  const { selectAllMatching } = a;
  const { folderId, containerKey, component, pendingByTestKey } = p;
  return (
    <>
      {/* A live load failure (listError) always wins over a stale imperative
          save/delete-view error, so a real fetch error is never masked. */}
      {(listError || error) && (
        <div className="error-text table-error">{listError || error}</div>
      )}

      {canSelectAllMatching && (
        <div className="select-all-banner">
          You've selected all {pageKeys.length} tests on this page.{" "}
          <button
            className="link-btn"
            onClick={selectAllMatching}
            disabled={selectingAll}
          >
            {selectingAll
              ? "Selecting…"
              : `Select all ${page.total.toLocaleString()} matching this filter`}
          </button>
          {selectAllError && (
            <span className="error-text">  {selectAllError}</span>
          )}
        </div>
      )}

      <div data-tour="grid" className="table-scroll" ref={scrollRef}>
        <table aria-label="Tests" className="grid-table" style={{ minWidth: gridMinWidth }}>
          <thead>
            <tr>
              <th scope="col" className="select-col" style={SELECT_STYLE}>
                <input
                  type="checkbox"
                  checked={allOnPageSelected}
                  ref={(el) => {
                    if (el) el.indeterminate = someOnPageSelected;
                  }}
                  disabled={pageKeys.length === 0}
                  onChange={() => onToggleSelectPage(pageKeys)}
                  aria-label={
                    allOnPageSelected
                      ? "Clear page selection"
                      : "Select all on this page"
                  }
                  title={
                    allOnPageSelected
                      ? "Clear page selection"
                      : "Select all on this page"
                  }
                />
              </th>
              {visibleColumns.map((c) => {
                const sortCol = COL_SORT[c.key];
                return sortCol ? (
                  <SortHeader
                    key={c.key}
                    col={sortCol}
                    label={COL_LABEL[c.key]}
                    sortBy={sortBy}
                    desc={desc}
                    onSort={toggleSort}
                    style={colStyle(c.key)}
                  />
                ) : (
                  <th scope="col" key={c.key} style={colStyle(c.key)}>
                    {COL_LABEL[c.key]}
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody
            style={{
              height: rowVirtualizer.getTotalSize(),
              position: "relative",
            }}
          >
            {rowVirtualizer.getVirtualItems().map((vrow) => {
              const t = page.tests[vrow.index];
              const hasPending = pendingByTestKey.has(t.key);
              const isSelected = selectedSet.has(t.key);
              const callsOthers = callerKeys.has(t.key);
              return (
                <tr
                  key={t.key}
                  data-rowindex={vrow.index}
                  className={
                    (t.key === selectedKey ? "row-selected " : "") +
                    (isSelected ? "row-checked" : "")
                  }
                  style={{ transform: `translateY(${vrow.start}px)` }}
                  tabIndex={vrow.index === tabbableIndex ? 0 : -1}
                  aria-selected={isSelected}
                  onClick={() => onSelect(t.key)}
                  onFocus={() => setFocusedIndex(vrow.index)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      onSelect(t.key);
                    } else if (e.key === "ArrowDown") {
                      e.preventDefault();
                      moveFocus(vrow.index + 1);
                    } else if (e.key === "ArrowUp") {
                      e.preventDefault();
                      moveFocus(vrow.index - 1);
                    } else if (e.key === "Home") {
                      e.preventDefault();
                      moveFocus(0);
                    } else if (e.key === "End") {
                      e.preventDefault();
                      moveFocus(page.tests.length - 1);
                    }
                  }}
                >
                  <td
                    className="select-col"
                    style={SELECT_STYLE}
                    onClick={(e) => e.stopPropagation()}
                  >
                    <input
                      type="checkbox"
                      checked={isSelected}
                      aria-label={`Select ${t.key}`}
                      onChange={() => onToggleSelect(t.key)}
                    />
                  </td>
                  {visibleColumns.map((c) =>
                    renderCell(c.key, t, hasPending, callsOthers, colStyle(c.key)),
                  )}
                </tr>
              );
            })}
          </tbody>
        </table>
        {!loading && page.tests.length === 0 && (
          <div className="grid-empty muted">
            {page.total === 0 &&
            debouncedSearch === "" &&
            status.trim() === "" &&
            folderId === "" &&
            containerKey === "" &&
            component === "" &&
            review === ""
              ? "No tests yet. Run a sync to pull them from Jira."
              : "No tests match the current filter."}
          </div>
        )}
      </div>
    </>
  );
}
