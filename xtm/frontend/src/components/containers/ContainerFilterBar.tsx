import { SortControl } from "../SortControl";
import { Modal } from "../Modal";
import { RunBadge } from "./RunCells";
import type { ContainerState } from "./useContainerState";
import type { ContainerActions, ContainerViewProps } from "./useContainerActions";

// ContainerFilterBar narrows the list (execution type, environment), shows a
// plan or set's consolidated results, and holds sort and the Set env dialog.
export function ContainerFilterBar({ s, a }: { s: ContainerState; a: ContainerActions; p: ContainerViewProps }) {
  const {
    kind, cExecType, setCExecType, cEnv, setCEnv, batchEnvOp, setBatchEnvOp, batchEnvName,
    setBatchEnvName, batchEnvBusy, envToolsOpen, setEnvToolsOpen, cSortField, setCSortField,
    cSortDesc, setCSortDesc, caps, containers, rollup, kindLabel, viewContainers, envOptions,
  } = s;
  const { applyBatchEnv, openBreakdown } = a;
  return (
      <div className="container-filter-bar">
        {kind === "testexec" && (
          <select
            className="container-status-filter app-select"
            value={cExecType}
            onChange={(e) => setCExecType(e.target.value)}
            title="Filter by execution type"
          >
            <option value="">All executions</option>
            <option value="standalone">Standalone</option>
            <option value="subtask">Sub-task</option>
          </select>
        )}
        {caps.supportsEnvironments && kind === "testexec" && envOptions.length > 0 && (
          <select
            className="container-status-filter app-select"
            value={cEnv}
            onChange={(e) => setCEnv(e.target.value)}
            title="Filter by test environment"
          >
            <option value="">All environments</option>
            {envOptions.map((e) => (
              <option key={e} value={e}>
                {e}
              </option>
            ))}
          </select>
        )}
        {/* Consolidated-results summary for the selected Test Plan / Set, shown
            inline on the filter row (moved here from the detail card; the label
            filter it replaced now sits beside the Status filter above). */}
        {kind !== "testexec" && rollup && rollup.total > 0 && (
          <div className="container-rollup">
            <span
              className="container-rollup-label"
              title={
                `Each test in this ${kindLabel} gets one combined result across the ` +
                `${rollup.execCount} Test Execution${rollup.execCount === 1 ? "" : "s"} that ran it. ` +
                `The worst result wins, so any FAIL makes the test FAIL overall. These badges count ` +
                `tests by that combined result; "(not run)" means no execution has recorded a result yet. ` +
                `Click a badge to see which tests are behind it.`
              }
            >
              Consolidated results across {rollup.execCount} execution{rollup.execCount === 1 ? "" : "s"}
              <span className="rollup-info" aria-hidden="true">ⓘ</span>
            </span>
            <div className="board-counts">
              {rollup.passed > 0 && (
                <RunBadge
                  status="PASS"
                  count={rollup.passed}
                  onPick={() => openBreakdown("PASS")}
                  pickHint="See the tests behind this result"
                />
              )}
              {rollup.failed > 0 && (
                <RunBadge
                  status="FAIL"
                  count={rollup.failed}
                  onPick={() => openBreakdown("FAIL")}
                  pickHint="See the tests behind this result"
                />
              )}
              {rollup.executing > 0 && (
                <RunBadge
                  status="EXECUTING"
                  count={rollup.executing}
                  onPick={() => openBreakdown("EXECUTING")}
                  pickHint="See the tests behind this result"
                />
              )}
              {rollup.aborted > 0 && (
                <RunBadge
                  status="ABORTED"
                  count={rollup.aborted}
                  onPick={() => openBreakdown("ABORTED")}
                  pickHint="See the tests behind this result"
                />
              )}
              {rollup.blocked > 0 && (
                <RunBadge
                  status="BLOCKED"
                  count={rollup.blocked}
                  onPick={() => openBreakdown("BLOCKED")}
                  pickHint="See the tests behind this result"
                />
              )}
              {rollup.notRun > 0 && (
                <RunBadge
                  status="(not run)"
                  count={rollup.notRun}
                  onPick={() => openBreakdown("(not run)")}
                  pickHint="See the tests behind this result"
                />
              )}
            </div>
          </div>
        )}
        <span className="muted container-filter-count">
          {viewContainers.length} of {containers.length}
        </span>

        {/* View controls (sort) and the batch-environment action sit in their
            own right-aligned group so they read as tools, not status filters
            (#310), while staying on the filter row to save vertical space. */}
        <div className="filter-tools">
          <SortControl
            fields={[
              { value: "key", label: "Key" },
              { value: "summary", label: "Name" },
              { value: "status", label: "Status" },
            ]}
            field={cSortField}
            desc={cSortDesc}
            onChange={(f, d) => {
              setCSortField(f);
              setCSortDesc(d);
            }}
          />
          {caps.supportsEnvironments && kind === "testexec" && (
            <div className="menu">
              <button
                className="btn"
                onClick={() => setEnvToolsOpen((o) => !o)}
                title="Apply an environment change to every execution currently shown"
                aria-haspopup="dialog"
                aria-expanded={envToolsOpen}
              >
                Set env on shown
                <span className="menu-caret" aria-hidden="true">
                  ▾
                </span>
              </button>
              {envToolsOpen && (
                <Modal
                  onClose={() => setEnvToolsOpen(false)}
                  className="modal bulk-modal"
                  labelledBy="env-tools-title"
                >
                  <div className="pending-head">
                    <h2 id="env-tools-title">
                      Set environment on the {viewContainers.length} shown
                    </h2>
                    <button
                      className="btn btn-ghost"
                      onClick={() => setEnvToolsOpen(false)}
                      aria-label="Close"
                      title="Close"
                    >
                      ✕
                    </button>
                  </div>
                  <div className="bulk-body">
                    <div className="env-tools-row">
                      <select
                        className="container-status-filter app-select"
                        value={batchEnvOp}
                        onChange={(e) =>
                          setBatchEnvOp(
                            e.target.value as "add_env" | "remove_env" | "set_env",
                          )
                        }
                        title="Batch environment operation"
                      >
                        <option value="add_env">Add env</option>
                        <option value="remove_env">Remove env</option>
                        <option value="set_env">Set env</option>
                      </select>
                      <input
                        className="container-env-add app-select"
                        list="container-env-names"
                        value={batchEnvName}
                        placeholder="Environment…"
                        onChange={(e) => setBatchEnvName(e.target.value)}
                      />
                      <datalist id="container-env-names">
                        {envOptions.map((e) => (
                          <option key={e} value={e} />
                        ))}
                      </datalist>
                    </div>
                  </div>
                  <div className="pending-actions">
                    <button className="btn" onClick={() => setEnvToolsOpen(false)}>
                      Cancel
                    </button>
                    <button
                      className="btn btn-primary"
                      onClick={async () => {
                        await applyBatchEnv();
                        setEnvToolsOpen(false);
                      }}
                      disabled={
                        batchEnvBusy ||
                        viewContainers.length === 0 ||
                        (batchEnvOp !== "set_env" && !batchEnvName.trim())
                      }
                      title="Apply to all executions currently shown by the filter"
                    >
                      {batchEnvBusy
                        ? "Applying…"
                        : `Apply to ${viewContainers.length}`}
                    </button>
                  </div>
                </Modal>
              )}
            </div>
          )}
        </div>
      </div>
  );
}
