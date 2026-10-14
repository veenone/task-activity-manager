
import type { TestDetailState } from "./useTestDetailState";
import type { TestDetailActions } from "./useTestDetailActions";
import type { TestDetailProps } from "./types";

// DetailHeader is the resize edge, the key (linked to Jira) with its status,
// Clone and Close, the not-yet-created banner, and the load state.
export function DetailHeader({ s, a, p }: { s: TestDetailState; a: TestDetailActions; p: TestDetailProps }) {
  const { cloning, canLinkToJira, test, panelLoading, displayError, openInJira, startResize } = s;
  const { cloneThisTest } = a;
  const { testKey, onClose, onCloned, readOnly } = p;
  return (
    <>
      <div
        className="detail-resizer"
        role="presentation"
        onMouseDown={startResize}
        title="Drag to resize"
      />
      <div className="detail-head">
        <div className="detail-head-id">
          {canLinkToJira ? (
            <button
              className="mono detail-key detail-key-link"
              onClick={openInJira}
              title="Open this test in Jira (browser)"
            >
              {testKey}
              <span className="detail-key-ext" aria-hidden="true">
                ↗
              </span>
            </button>
          ) : (
            <span className="mono detail-key">{testKey}</span>
          )}
          {test && (
            <span className="status-pill detail-head-status">
              {test.status || "—"}
            </span>
          )}
        </div>
        <div className="detail-head-actions">
          {!readOnly && onCloned && !testKey.startsWith("NEW-") && (
            <button
              className="btn btn-ghost detail-clone"
              onClick={cloneThisTest}
              disabled={cloning}
              title="Create a new test that copies this test's fields and steps"
            >
              {cloning ? "Cloning…" : "⧉ Clone"}
            </button>
          )}
          <button
            className="btn btn-ghost detail-close"
            onClick={onClose}
            title="Close"
           aria-label="Close">
            ✕
          </button>
        </div>
      </div>

      {testKey.startsWith("NEW-") && (
        <div className="detail-uncommitted-banner">
          This test hasn't been created in Jira yet. It'll be created when you
          commit.
        </div>
      )}

      {panelLoading && <div className="muted detail-body">Loading…</div>}
      {displayError && (
        <div className="error-text detail-body">{displayError}</div>
      )}
    </>
  );
}
