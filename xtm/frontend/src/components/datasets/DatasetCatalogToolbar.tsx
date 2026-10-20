import type { DatasetCatalogState } from "./useDatasetCatalogState";

// DatasetCatalogToolbar is the view's filter row, built from the same shared
// pieces as the Containers view's: board-head for the row, board-picker for
// each labelled control, app-select and detail-input for the controls, and
// board-head-actions holding the scan at the far end.
export function DatasetCatalogToolbar({ state }: { state: DatasetCatalogState }) {
  const syncedAt = state.catalog.data?.state?.lastSyncedAt;
  return (
    <div className="board-head dataset-toolbar">
      <input
        type="search"
        className="detail-input dataset-search"
        value={state.search}
        aria-label="Search datasets"
        placeholder="Search Tests or containers"
        onChange={(event) => state.setSearch(event.target.value)}
      />
      <label className="board-picker">
        <span>Scope</span>
        <select
          className="app-select board-select-narrow"
          aria-label="Dataset scope"
          value={state.scope}
          onChange={(event) => state.setScope(event.target.value)}
        >
          <option value="">All scopes</option>
          <option value="test">Test defaults</option>
          <option value="plan">Plan overrides</option>
          <option value="execution">Execution overrides</option>
        </select>
      </label>
      <label className="board-picker">
        <span>Ownership</span>
        <select
          className="app-select board-select-narrow"
          aria-label="Test ownership"
          value={state.ownership}
          onChange={(event) => state.setOwnership(event.target.value)}
        >
          <option value="">All Tests</option>
          <option value="owned">This project</option>
          <option value="external">Other projects</option>
        </select>
      </label>
      <div className="board-head-actions dataset-toolbar-sync">
        {syncedAt && (
          <span className="muted">
            Last scanned {new Date(syncedAt).toLocaleString()}
          </span>
        )}
        <button
          className="btn btn-primary"
          disabled={!state.canSync || state.syncing}
          aria-label={state.syncing ? "Scanning datasets" : "Scan datasets"}
          onClick={() => void state.sync()}
        >
          {state.syncing ? "Scanning…" : "Scan datasets"}
        </button>
      </div>
    </div>
  );
}
