import { Modal } from "../Modal";
import { REVIEW_ENABLED } from "../../features";
import { EXEC_TYPE_OPTIONS, COL_LABEL } from "./columns";
import type { TestTableProps, TestTableState } from "./useTestTableState";
import type { TestTableActions } from "./useTestTableActions";

// FilterBar is the row above the grid: search, status, exec type, saved
// views, Sync, Export and the Columns dialog.
export function FilterBar({ s, a, p }: { s: TestTableState; a: TestTableActions; p: TestTableProps }) {
  const { selectedKey, search, setSearch, status, setStatus, execType, setExecType, review, setReview, savedViews, activeView, statusOptions, columns, showColumns, setShowColumns, caps, page, toggleColumn, moveColumn } = s;
  const { saveView, applyView, deleteActiveView, exportTests } = a;
  const { onSync, syncing } = p;
  return (
    <>
      <div className="filters">
        <input
          data-tour="search"
          className="search"
          placeholder="Search key, summary, description…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <input
          className="status-filter"
          placeholder="Status…"
          list="status-options"
          value={status}
          onChange={(e) => setStatus(e.target.value)}
          title="Filter by workflow status (pick from the list or type)"
        />
        <datalist id="status-options">
          {statusOptions.map((s) => (
            <option key={s} value={s} />
          ))}
        </datalist>
        {caps.supportsTestTypes && (
          <select
            className="exectype-filter"
            value={execType}
            onChange={(e) => setExecType(e.target.value)}
            title="Filter by execution type (Xray Test Type)"
          >
            <option value="">Any exec type</option>
            {EXEC_TYPE_OPTIONS.map((o) => (
              <option key={o} value={o}>
                {o}
              </option>
            ))}
          </select>
        )}
        {REVIEW_ENABLED && (
          <select
            className="review-filter"
            value={review}
            onChange={(e) => setReview(e.target.value)}
            title="Filter by review verdict"
          >
            <option value="">Any review</option>
            <option value="approved">Approved</option>
            <option value="rejected">Rejected</option>
            <option value="pending">Pending review</option>
            <option value="unreviewed">Unreviewed</option>
          </select>
        )}
        <div className="saved-views">
          <select
            className="view-select"
            value={activeView}
            onChange={(e) => applyView(e.target.value)}
            title="Apply a saved filter"
          >
            <option value="">Saved views…</option>
            {savedViews.map((v) => (
              <option key={v.id} value={v.id}>
                {v.name}
              </option>
            ))}
          </select>
          {activeView && (
            <button
              className="btn btn-ghost view-del"
              onClick={deleteActiveView}
              title="Delete this saved view"
             aria-label="Delete this saved view">
              ✕
            </button>
          )}
          <button className="btn view-save" onClick={saveView}>
            Save view
          </button>
        </div>
        {onSync && (
          <button
            className="btn"
            onClick={onSync}
            disabled={syncing}
            title="Pull the latest test data from Jira (tests only)"
          >
            {syncing ? "Syncing…" : "Sync"}
          </button>
        )}
        <button
          className="btn"
          onClick={exportTests}
          title="Export the filtered tests to CSV or XLSX"
          disabled={page.total === 0}
        >
          Export
        </button>
        {!selectedKey && (
        <div className="columns-menu">
          <button
            className="btn"
            onClick={() => setShowColumns((s) => !s)}
            title="Show / hide / reorder columns"
            aria-haspopup="dialog"
            aria-expanded={showColumns}
          >
            Columns
          </button>
          {showColumns && (
            <Modal
              onClose={() => setShowColumns(false)}
              className="modal bulk-modal"
              labelledBy="columns-title"
            >
              <div className="pending-head">
                <h2 id="columns-title">Columns</h2>
                <button
                  className="btn btn-ghost"
                  onClick={() => setShowColumns(false)}
                  aria-label="Close"
                  title="Close"
                >
                  ✕
                </button>
              </div>
              <div className="bulk-body">
                {columns.map((c, i) => (
                  <div key={c.key} className="columns-row">
                    <label className="columns-label">
                      <input
                        type="checkbox"
                        checked={c.visible}
                        onChange={() => toggleColumn(c.key)}
                      />
                      {COL_LABEL[c.key]}
                    </label>
                    <span className="columns-reorder">
                      <button
                        className="btn btn-ghost"
                        disabled={i === 0}
                        onClick={() => moveColumn(c.key, -1)}
                        title="Move up"
                      >
                        ▲
                      </button>
                      <button
                        className="btn btn-ghost"
                        disabled={i === columns.length - 1}
                        onClick={() => moveColumn(c.key, 1)}
                        title="Move down"
                      >
                        ▼
                      </button>
                    </span>
                  </div>
                ))}
              </div>
            </Modal>
          )}
        </div>
        )}
      </div>
    </>
  );
}
