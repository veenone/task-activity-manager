import { Markdown } from "../Markdown";
import { MarkdownField } from "../MarkdownField";
import { PRECOND_TYPES } from "./usePreconditionsState";
import type { PreconditionsState } from "./usePreconditionsState";

// DetailFields shows the summary, type, condition and description, or their
// editors while an edit is open.
export function DetailFields({ s }: { s: PreconditionsState }) {
  const { editing, summaryDraft, setSummaryDraft, descDraft, setDescDraft, condDraft, setCondDraft, typeDraft, setTypeDraft, condOpen, setCondOpen, descOpen, setDescOpen, detailsOpen, selectedPre } = s;
  if (!selectedPre) return null;
  return (
    <>
      {editing ? (
        <>
          <div className="precond-field">
            <span>Summary</span>
            <MarkdownField
              className="detail-input"
              multiline={false}
              value={summaryDraft}
              onChange={setSummaryDraft}
              onCommit={() => {}}
              placeholder="Markdown supported."
            />
          </div>

          <label className="precond-field">
            <span>Type</span>
            <select
              className="detail-input"
              value={typeDraft}
              onChange={(e) => setTypeDraft(e.target.value)}
            >
              {PRECOND_TYPES.map((t) => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
              {typeDraft && !PRECOND_TYPES.includes(typeDraft) && (
                <option value={typeDraft}>{typeDraft}</option>
              )}
            </select>
          </label>

          <div className="precond-field">
            <span>Condition</span>
            <MarkdownField
              className="detail-input precond-desc"
              value={condDraft}
              onChange={setCondDraft}
              onCommit={() => {}}
              rows={4}
              placeholder="Markdown supported."
            />
          </div>

          <div className="precond-field">
            <span>Description</span>
            <MarkdownField
              className="detail-input precond-desc"
              value={descDraft}
              onChange={setDescDraft}
              onCommit={() => {}}
              rows={5}
              placeholder="Markdown supported."
            />
          </div>
        </>
      ) : detailsOpen ? (
        <>
          <div className="precond-field">
            <span>Summary</span>
            <div className="precond-ro-val">
              {selectedPre.summary.trim() ? (
                <Markdown>{selectedPre.summary}</Markdown>
              ) : (
                <span className="muted">No summary.</span>
              )}
            </div>
          </div>

          <div className="precond-field">
            <span>Type</span>
            <div className="precond-ro-val">
              {selectedPre.type || "Manual"}
            </div>
          </div>

          {selectedPre.condition?.trim() ? (
            <div className="detail-description">
              <button
                className="bugs-md-desc-toggle"
                onClick={() => setCondOpen((o) => !o)}
                aria-expanded={condOpen}
              >
                {condOpen ? "▾" : "▸"} Condition
              </button>
              {condOpen && (
                <div className="bugs-md-detail-extra-text">
                  <Markdown>{selectedPre.condition}</Markdown>
                </div>
              )}
            </div>
          ) : (
            <div className="precond-field">
              <span>Condition</span>
              <div className="precond-ro-val">
                <span className="muted">No condition defined.</span>
              </div>
            </div>
          )}

          {selectedPre.description?.trim() ? (
            <div className="detail-description">
              <button
                className="bugs-md-desc-toggle"
                onClick={() => setDescOpen((o) => !o)}
                aria-expanded={descOpen}
              >
                {descOpen ? "▾" : "▸"} Description
              </button>
              {descOpen && (
                <div className="bugs-md-detail-extra-text">
                  <Markdown>{selectedPre.description}</Markdown>
                </div>
              )}
            </div>
          ) : (
            <div className="precond-field">
              <span>Description</span>
              <div className="precond-ro-val">
                <span className="muted">No description.</span>
              </div>
            </div>
          )}
        </>
      ) : null}
    </>
  );
}
