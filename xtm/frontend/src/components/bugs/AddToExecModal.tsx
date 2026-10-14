import { Modal } from "../Modal";
import type { BugsState } from "./useBugsState";
import type { BugsActions } from "./useBugsActions";

// AddToExecModal adds the checked bugs' linked tests to an existing Test
// Execution picked from a filterable list.
export function AddToExecModal({ s, a }: { s: BugsState; a: BugsActions }) {
  const { addToExecOpen, setAddToExecOpen, addToExecLoading, addToExecExecs, addToExecTarget, setAddToExecTarget, addToExecFilter, setAddToExecFilter, addToExecWorking, addToExecError, unionTestKeys } = s;
  const { confirmAddToExec } = a;
  return (
    <>
      {addToExecOpen && (
        <Modal onClose={() => !addToExecWorking && setAddToExecOpen(false)} className="modal bulk-modal" labelledBy="add-to-exec-title">
            <div className="pending-head">
              <h2 id="add-to-exec-title">
                Add to Test Execution ({unionTestKeys.length}{" "}
                {unionTestKeys.length === 1 ? "test" : "tests"})
              </h2>
              <button
                className="btn btn-ghost"
                onClick={() => setAddToExecOpen(false)}
                disabled={addToExecWorking}
                title="Close"
                aria-label="Close"
              >
                ✕
              </button>
            </div>
            <div className="bulk-body">
              {addToExecLoading ? (
                <p className="muted">Loading Test Executions…</p>
              ) : addToExecExecs.length === 0 && !addToExecError ? (
                <p className="muted">
                  No Test Executions found. Sync containers first, or create a
                  new execution with "Create Test Execution".
                </p>
              ) : (
                <>
                  <label className="bulk-row">
                    <span>Filter</span>
                    <input
                      className="search"
                      placeholder="Filter by key or summary…"
                      value={addToExecFilter}
                      onChange={(e) => {
                        setAddToExecFilter(e.target.value);
                        // Reset target to first match when filter changes.
                        const f = e.target.value.trim().toLowerCase();
                        const filtered = !f
                          ? addToExecExecs
                          : addToExecExecs.filter(
                              (c) =>
                                c.key.toLowerCase().includes(f) ||
                                c.summary.toLowerCase().includes(f),
                            );
                        setAddToExecTarget(filtered.length > 0 ? filtered[0].key : "");
                      }}
                    />
                  </label>
                  {(() => {
                    const f = addToExecFilter.trim().toLowerCase();
                    const filtered = !f
                      ? addToExecExecs
                      : addToExecExecs.filter(
                          (c) =>
                            c.key.toLowerCase().includes(f) ||
                            c.summary.toLowerCase().includes(f),
                        );
                    return (
                      <label className="bulk-row">
                        <span>Execution</span>
                        {filtered.length === 0 ? (
                          <span className="muted">No executions match the filter.</span>
                        ) : (
                          <select
                            className="app-select"
                            value={addToExecTarget}
                            onChange={(e) => setAddToExecTarget(e.target.value)}
                          >
                            {filtered.map((c) => (
                              <option key={c.key} value={c.key}>
                                {c.key} — {c.summary}
                              </option>
                            ))}
                          </select>
                        )}
                      </label>
                    );
                  })()}
                  <p className="muted bulk-preview">
                    The {unionTestKeys.length} affected test
                    {unionTestKeys.length === 1 ? "" : "s"} from the checked
                    bugs will be added to the chosen execution. Tests already in
                    it are skipped. Changes are queued locally; commit them from
                    the Pending list.
                  </p>
                </>
              )}
              {addToExecError && (
                <div className="error-text">{addToExecError}</div>
              )}
            </div>
            <div className="pending-actions">
              <button
                className="btn"
                onClick={() => setAddToExecOpen(false)}
                disabled={addToExecWorking}
              >
                Cancel
              </button>
              <button
                className="btn btn-primary"
                onClick={confirmAddToExec}
                disabled={
                  addToExecWorking ||
                  addToExecLoading ||
                  !addToExecTarget ||
                  unionTestKeys.length === 0
                }
              >
                {addToExecWorking ? "Working…" : "Add to Execution"}
              </button>
            </div>
        </Modal>
      )}
    </>
  );
}
