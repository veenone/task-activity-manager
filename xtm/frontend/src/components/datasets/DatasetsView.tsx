import { errMsg } from "../../api";
import { DatasetCatalogTable } from "./DatasetCatalogTable";
import type { ContainerNavigationTarget } from "../../contexts/NavContext";
import { DatasetCatalogToolbar } from "./DatasetCatalogToolbar";
import { useDatasetCatalogState } from "./useDatasetCatalogState";
import "./datasets.css";

interface Props {
  jiraUrl: string;
  onOpenTest: (key: string) => void;
  onOpenContainer: (target: ContainerNavigationTarget) => void;
}

export function DatasetsView(props: Props) {
  const state = useDatasetCatalogState();
  const page = state.catalog.data;
  const loadError = state.catalog.error ? errMsg(state.catalog.error) : "";

  return (
    <section className="dataset-page" aria-labelledby="datasets-heading">
      {/* Like every other view, this one opens on its filters; the name is
          in the view tabs above, and here for a screen reader. */}
      <h2 id="datasets-heading" className="sr-only">Datasets</h2>
      <DatasetCatalogToolbar state={state} />
      <p className="muted dataset-intro">Test defaults and the Plan or Execution overrides that change them.</p>
      {(state.syncError || loadError) && (
        <div className="error-text" role="alert">
          {state.syncError || loadError}
        </div>
      )}
      {!page && state.catalog.isLoading ? (
        <p className="dataset-empty muted">Loading datasets…</p>
      ) : !page?.state ? (
        <div className="dataset-empty">
          <h3>This project has not been scanned for datasets yet.</h3>
          <p className="muted">
            Scan to list this project's datasets. Jira is not changed.
          </p>
        </div>
      ) : page.total === 0 ? (
        <div className="dataset-empty">
          <h3>No Tests with datasets match these filters.</h3>
          <p className="muted">
            Change the filters, or scan again if datasets changed in Jira.
          </p>
        </div>
      ) : (
        <DatasetCatalogTable state={state} {...props} />
      )}
    </section>
  );
}
