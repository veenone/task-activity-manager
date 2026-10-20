import { REVIEW_ENABLED } from "../features";
import type { AppState } from "./useAppState";

// BulkToolbar appears in Browse while rows are selected and opens the bulk
// dialogs the active backend supports.
export function BulkToolbar({ s }: { s: AppState }) {
  const { openModal, view, selectedSet, setSelectedSet, folders, caps } = s;
  return (
    <>
      {view === "browse" && selectedSet.size > 0 && (
        <div className="bulk-toolbar">
          <span className="bulk-count">{selectedSet.size} selected</span>
          <button
            className="btn btn-primary"
            onClick={() => openModal("bulkEdit")}
          >
            Bulk edit…
          </button>
          <button
            className="btn btn-primary"
            onClick={() => openModal("bulkLabels")}
          >
            Labels…
          </button>
          <button
            className="btn btn-primary"
            onClick={() => openModal("bulkComponents")}
          >
            Components…
          </button>
          <button
            className="btn btn-primary"
            onClick={() => openModal("bulkRename")}
          >
            Rename summaries…
          </button>
          <button
            className="btn btn-primary"
            onClick={() => openModal("bulkStepReplace")}
          >
            Replace step text…
          </button>
          {caps.supportsWorkflowTransitions && (
            <button
              className="btn btn-primary"
              onClick={() => openModal("bulkTransition")}
            >
              Bulk transition…
            </button>
          )}
          <button
            className="btn btn-primary"
            onClick={() => openModal("bulkAllocate")}
          >
            Allocate…
          </button>
          {caps.supportsFolderWrites && folders.length > 0 && (
            <button
              className="btn btn-primary"
              onClick={() => openModal("bulkMove")}
            >
              Move to folder…
            </button>
          )}
          {caps.supportsPreconditionObjects && (
            <button
              className="btn btn-primary"
              onClick={() => openModal("bulkPreconditions")}
            >
              Preconditions…
            </button>
          )}
          {caps.supportsRequirementObjects && (
            <button
              className="btn btn-primary"
              onClick={() => openModal("bulkRequirements")}
            >
              Requirements…
            </button>
          )}
          {REVIEW_ENABLED && (
            <button
              className="btn btn-primary"
              onClick={() => openModal("bulkReview")}
            >
              Review…
            </button>
          )}
          <button className="btn" onClick={() => setSelectedSet(new Set())}>
            Clear
          </button>
        </div>
      )}
    </>
  );
}
