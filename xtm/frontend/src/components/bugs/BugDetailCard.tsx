import { formatDateTime } from "../../dates";
import { Markdown } from "../Markdown";
import type { BugsState } from "./useBugsState";

// BugDetailCard is the selected bug's header, fields, and the description and
// defect fields loaded with its detail.
export function BugDetailCard({ s }: { s: BugsState }) {
  const { bugDetail, bugDetailLoading, descOpen, setDescOpen, analysisOpen, setAnalysisOpen, detailsOpen, setDetailsOpen, canLink, sel, openBug } = s;
  if (!sel) return null;
  return (
    <>
      <div className="bugs-md-detail-card">
        <div className="bugs-md-detail-card-header">
          {canLink && !sel.key.startsWith("NEW-") ? (
            <button
              className="mono bug-link-key bugs-md-detail-key"
              onClick={() => openBug(sel.key)}
              title={`Open ${sel.key} in Jira`}
            >
              {sel.key}
            </button>
          ) : (
            <span className="mono bugs-md-detail-key">{sel.key}</span>
          )}
          {sel.status && <span className="status-pill">{sel.status}</span>}
        </div>
        <div className="bugs-md-summary-row">
          <button
            type="button"
            className="collapse-caret"
            onClick={() => setDetailsOpen((o) => !o)}
            aria-expanded={detailsOpen}
            title={detailsOpen ? "Hide details" : "Show details"}
          >
            {detailsOpen ? "▾" : "▸"}
          </button>
          <h2 className="bugs-md-detail-summary">
            {sel.summary || "(no summary)"}
          </h2>
        </div>
        {detailsOpen && (
          <>
        <dl className="detail-fields bugs-md-detail-fields bugs-md-detail-fields-2col">
          {sel.issueType && (
            <>
              <dt>Type</dt>
              <dd>{sel.issueType}</dd>
            </>
          )}
          <dt>Project</dt>
          <dd className="mono">{sel.projectKey}</dd>
          {sel.priority && (
            <>
              <dt>Priority</dt>
              <dd>{sel.priority}</dd>
            </>
          )}
          {sel.updated && (
            <>
              <dt>Updated</dt>
              <dd className="muted">{formatDateTime(sel.updated)}</dd>
            </>
          )}
          {bugDetail?.reporter && (
            <>
              <dt>Reporter</dt>
              <dd>{bugDetail.reporter}</dd>
            </>
          )}
          {bugDetail?.severity && (
            <>
              <dt>Severity</dt>
              <dd>{bugDetail.severity}</dd>
            </>
          )}
          <dt>Affects</dt>
          <dd>{sel.testKeys.length} test{sel.testKeys.length === 1 ? "" : "s"}</dd>
        </dl>
        {bugDetailLoading && (
          <p className="muted bugs-md-detail-extra-loading">Loading bug detail…</p>
        )}
        {!bugDetailLoading && bugDetail && (
          <div className="bugs-md-detail-extra">
            {bugDetail.description && (
              <div className="bugs-md-detail-extra-field">
                <button
                  className="bugs-md-desc-toggle"
                  onClick={() => setDescOpen((o) => !o)}
                  aria-expanded={descOpen}
                >
                  {descOpen ? "▾" : "▸"} Description
                </button>
                {descOpen && (
                  <div className="bugs-md-detail-extra-text">
                    <Markdown>{bugDetail.description}</Markdown>
                  </div>
                )}
              </div>
            )}
            {bugDetail.defectOrigin && (
              <div className="bugs-md-detail-extra-field">
                <strong>Defect Origin</strong>
                <div className="bugs-md-detail-extra-text">{bugDetail.defectOrigin}</div>
              </div>
            )}
            {bugDetail.defectAnalysis && (
              <div className="bugs-md-detail-extra-field">
                <button
                  className="bugs-md-desc-toggle"
                  onClick={() => setAnalysisOpen((o) => !o)}
                  aria-expanded={analysisOpen}
                >
                  {analysisOpen ? "▾" : "▸"} Defect Analysis
                </button>
                {analysisOpen && (
                  <div className="bugs-md-detail-extra-text">
                    <Markdown>{bugDetail.defectAnalysis}</Markdown>
                  </div>
                )}
              </div>
            )}
            {bugDetail.correctionDetails && (
              <div className="bugs-md-detail-extra-field">
                <strong>Correction Details</strong>
                <div className="bugs-md-detail-extra-text">
                  <Markdown>{bugDetail.correctionDetails}</Markdown>
                </div>
              </div>
            )}
          </div>
        )}
        </>
        )}
      </div>
    </>
  );
}
