import type { TestTableProps } from "./test-table/useTestTableState";
import { useTestTableState } from "./test-table/useTestTableState";
import { useTestTableActions } from "./test-table/useTestTableActions";
import { FilterBar } from "./test-table/FilterBar";
import { Grid } from "./test-table/Grid";
import { TablePager } from "./test-table/TablePager";

// TestTable is the Browse grid of tests. The hooks in test-table/ hold what it
// knows and does; the filter bar, grid and pager render it.
export function TestTable(p: TestTableProps) {
  const s = useTestTableState(p);
  const a = useTestTableActions(s, p);

  return (
    <div className="testtable">
      <FilterBar s={s} a={a} p={p} />

      <Grid s={s} a={a} p={p} />

      <TablePager s={s} />
    </div>
  );
}
