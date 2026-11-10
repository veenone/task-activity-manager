import { SortControl } from "../SortControl";
import { Modal } from "../Modal";
import type { ContainerState } from "./useContainerState";
import type { ContainerActions, ContainerViewProps } from "./useContainerActions";

// ContainerFilterBar narrows the list (execution type, environment) and holds
// sort and the Set env dialog. It sits in the Filters row's panel (#456); a
// plan's or set's consolidated results moved to the card's details.
export function ContainerFilterBar({ s, a }: { s: ContainerState; a: ContainerActions; p: ContainerViewProps }) {
  const {
    kind, cExecType, setCExecType, cEnv, setCEnv, batchEnvOp, setBatchEnvOp, batchEnvName,
    setBatchEnvName, batchEnvBusy, envToolsOpen, setEnvToolsOpen, cSortField, setCSortField,
    cSortDesc, setCSortDesc, caps, containers, viewContainers, envOptions,
  } = s;
  const { applyBatchEnv } = a;
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
