import { PAGE_SIZE_OPTIONS } from "./columns";
import type { TestTableState } from "./useTestTableState";

// TablePager is the footer: page size, page navigation, jump-to-page, and
// the shown range.
export function TablePager({ s }: { s: TestTableState }) {
  const { pageSize, setPageSize, pageInput, setPageInput, page, loading, from, to, totalPages, currentPage, goToPage, commitPageInput } = s;
  return (
    <>
      <div className="table-footer">
        <div className="pager">
          <select
            className="page-size"
            value={pageSize}
            onChange={(e) => setPageSize(Number(e.target.value))}
            title="Rows per page"
          >
            {PAGE_SIZE_OPTIONS.map((n) => (
              <option key={n} value={n}>
                {n} / page
              </option>
            ))}
          </select>
          <button
            className="btn page-btn"
            disabled={currentPage <= 1 || loading}
            onClick={() => goToPage(1)}
            title="First page"
          >
            «
          </button>
          <button
            className="btn page-btn"
            disabled={currentPage <= 1 || loading}
            onClick={() => goToPage(currentPage - 1)}
            title="Previous page"
          >
            ‹
          </button>
          <span className="page-indicator">
            Page{" "}
            <input
              className="page-jump"
              type="text"
              inputMode="numeric"
              value={pageInput}
              placeholder={String(currentPage)}
              disabled={loading}
              onChange={(e) =>
                setPageInput(e.target.value.replace(/[^0-9]/g, ""))
              }
              onBlur={commitPageInput}
              onKeyDown={(e) => {
                if (e.key === "Enter") (e.target as HTMLInputElement).blur();
              }}
              title="Jump to page"
            />{" "}
            of {totalPages.toLocaleString()}
          </span>
          <button
            className="btn page-btn"
            disabled={currentPage >= totalPages || loading}
            onClick={() => goToPage(currentPage + 1)}
            title="Next page"
          >
            ›
          </button>
          <button
            className="btn page-btn"
            disabled={currentPage >= totalPages || loading}
            onClick={() => goToPage(totalPages)}
            title="Last page"
          >
            »
          </button>
        </div>
        <span className="muted count">
          {loading
            ? "Loading…"
            : `${from.toLocaleString()}–${to.toLocaleString()} of ${page.total.toLocaleString()}`}
        </span>
      </div>
    </>
  );
}
