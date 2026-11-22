import { Pager } from "@agile-suite/core";
import { PAGE_SIZE_OPTIONS } from "./columns";
import type { TestTableState } from "./useTestTableState";

// TablePager is the footer under the test list: core's pager in the band
// that keeps it pinned below the grid.
export function TablePager({ s }: { s: TestTableState }) {
  const { pageSize, setPageSize, page, loading, currentPage, goToPage } = s;
  return (
    <div className="table-footer">
      <Pager
        page={currentPage - 1}
        pageSize={pageSize}
        total={page.total}
        onPage={(p) => goToPage(p + 1)}
        onPageSize={setPageSize}
        pageSizeOptions={PAGE_SIZE_OPTIONS}
        loading={loading}
      />
    </div>
  );
}
