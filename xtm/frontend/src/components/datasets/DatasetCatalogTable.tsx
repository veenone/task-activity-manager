import { Fragment } from "react";
import type { DatasetCatalogTestSummary } from "../../api";
import { Pager } from "../Pager";
import { DatasetCatalogDetail } from "./DatasetCatalogDetail";
import type { ContainerTarget } from "./DatasetCatalogDetail";
import type { DatasetCatalogState } from "./useDatasetCatalogState";

interface Props {
  state: DatasetCatalogState;
  jiraUrl: string;
  onOpenTest: (key: string) => void;
  onOpenContainer: (target: ContainerTarget) => void;
}

function Count({ value, singular }: { value: number; singular: string }) {
  return (
    <span>
      <strong>{value}</strong> {singular}{value === 1 ? "" : "s"}
    </span>
  );
}

function TestLabels({ test }: { test: DatasetCatalogTestSummary }) {
  return (
    <span className="dataset-labels">
      {test.external && <span className="dataset-label">External</span>}
      {test.defaultParameterCount === 0 && test.overrideCount > 0 && (
        <span className="dataset-label">Override only</span>
      )}
    </span>
  );
}

export function DatasetCatalogTable({
  state,
  jiraUrl,
  onOpenTest,
  onOpenContainer,
}: Props) {
  const tests = state.catalog.data?.tests ?? [];
  const total = state.catalog.data?.total ?? 0;
  return (
    <>
      <div className="dataset-table-scroll">
        <table className="board-table dataset-table" aria-label="Dataset catalog">
          <thead>
            <tr>
              <th scope="col">Test</th>
              <th scope="col">Default</th>
              <th scope="col">Linked containers</th>
              <th scope="col">Overrides</th>
            </tr>
          </thead>
          <tbody>
            {tests.map((test) => {
              const open = state.expandedKey === test.testKey;
              const detailId = `dataset-detail-${test.testKey.replace(/[^a-zA-Z0-9_-]/g, "-")}`;
              return (
                <Fragment key={test.testKey}>
                  <tr>
                    <td>
                      <button
                        className="dataset-expand"
                        aria-label={`${open ? "Collapse" : "Expand"} ${test.testKey}`}
                        aria-expanded={open}
                        aria-controls={detailId}
                        onClick={() => state.toggleExpanded(test.testKey)}
                      >
                        <span aria-hidden="true">{open ? "−" : "+"}</span>
                        <span className="dataset-test-copy">
                          <strong>{test.testKey}</strong>
                          <span>{test.summary}</span>
                        </span>
                      </button>
                      <TestLabels test={test} />
                    </td>
                    <td>
                      <Count value={test.defaultParameterCount} singular="parameter" />
                      <span className="muted"> · </span>
                      <Count value={test.defaultRowCount} singular="row" />
                    </td>
                    <td>
                      <Count value={test.planCount} singular="Plan" />
                      <span className="muted"> · </span>
                      <Count value={test.executionCount} singular="Execution" />
                    </td>
                    <td>{test.overrideCount} override{test.overrideCount === 1 ? "" : "s"}</td>
                  </tr>
                  {open && (
                    <tr key={`${test.testKey}:detail`} className="dataset-detail-row">
                      <td colSpan={4} id={detailId}>
                        <DatasetCatalogDetail
                          profileId={state.profileId}
                          testKey={test.testKey}
                          open
                          jiraUrl={jiraUrl}
                          onOpenTest={onOpenTest}
                          onOpenContainer={onOpenContainer}
                        />
                      </td>
                    </tr>
                  )}
                </Fragment>
              );
            })}
          </tbody>
        </table>
      </div>
      <Pager
        page={state.page}
        pageSize={state.pageSize}
        total={total}
        onPage={state.setPage}
        onPageSize={state.setPageSize}
        pageSizeOptions={[10, 25, 50, 100]}
      />
    </>
  );
}
