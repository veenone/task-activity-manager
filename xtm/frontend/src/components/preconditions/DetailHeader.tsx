import { Menu } from "../Menu";
import type { PreconditionsState } from "./usePreconditionsState";
import type { PreconditionsActions } from "./usePreconditionsActions";

// DetailHeader is the selected precondition's key (linked to Jira), its
// uncommitted badge, and Edit / Save / Cancel / Actions.
export function DetailHeader({ s, a }: { s: PreconditionsState; a: PreconditionsActions }) {
  const { editing, setEditing, detailsOpen, setDetailsOpen, selectedPre, isLocal, canLinkToJira, openInJira } = s;
  const { handleSave, handleCancel, deletePrecondition } = a;
  if (!selectedPre) return null;
  return (
    <>
      <div className="precond-detail-head">
        <div className="precond-detail-id">
          {!editing && (
            <button
              type="button"
              className="collapse-caret"
              onClick={() => setDetailsOpen((o) => !o)}
              aria-expanded={detailsOpen}
              title={detailsOpen ? "Hide details" : "Show details"}
            >
              {detailsOpen ? "▾" : "▸"}
            </button>
          )}
          {canLinkToJira ? (
            <button
              type="button"
              className="mono precond-detail-key precond-key-link"
              onClick={openInJira}
              title="Open this precondition in Jira (browser)"
            >
              {selectedPre.key}
              <span className="detail-key-ext" aria-hidden="true">
                ↗
              </span>
            </button>
          ) : (
            <span className="mono precond-detail-key">
              {selectedPre.key}
            </span>
          )}
          {isLocal && (
            <span
              className="pending-badge"
              title="Not yet created in Jira"
            >
              new · uncommitted
            </span>
          )}
        </div>
        <div className="precond-detail-actions">
          {editing ? (
            <>
              <button
                className="btn"
                onClick={handleCancel}
                title="Discard edits and return to read-only view"
              >
                Cancel
              </button>
              <button
                className="btn btn-primary"
                onClick={handleSave}
                title="Save changes to the local store"
              >
                Save
              </button>
            </>
          ) : (
            <>
              <button
                className="btn"
                onClick={() => setEditing(true)}
                title="Edit this precondition's fields"
              >
                Edit
              </button>
              <Menu
                label="Actions"
                align="right"
                triggerClassName="btn"
                title="Precondition actions"
                items={[
                  {
                    key: "delete",
                    label: "Delete precondition…",
                    onClick: deletePrecondition,
                    danger: true,
                  },
                ]}
              />
            </>
          )}
        </div>
      </div>
    </>
  );
}
