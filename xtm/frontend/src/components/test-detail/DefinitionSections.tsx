import { MarkdownField } from "../MarkdownField";
import { DirtyDot } from "./rows";
import type { TestDetailState } from "./useTestDetailState";
import type { TestDetailActions } from "./useTestDetailActions";
import type { TestDetailProps } from "./types";

// DescriptionSection is the markdown description and the type-change notice.
export function DescriptionSection({ s, a, p }: { s: TestDetailState; a: TestDetailActions; p: TestDetailProps }) {
  const { description, setDescription, prefillNotice, setPrefillNotice, isDirty } = s;
  const { saveField } = a;
  const { readOnly } = p;
  return (
    <>
      <h4>
        Description {isDirty("description") && <DirtyDot />}
      </h4>
      {readOnly ? (
        <p className="detail-input detail-input-static detail-desc-static">
          {description || <span className="muted">No description yet.</span>}
        </p>
      ) : (
        <MarkdownField
          className="detail-desc-edit"
          value={description}
          onChange={setDescription}
          onCommit={() => saveField("description", description)}
          rows={8}
          placeholder="No description. Click to add (markdown supported)."
        />
      )}

      {prefillNotice && (
        <div className="prefill-notice">
          The previous type already had content, so it was left unchanged.{" "}
          <button
            className="link-btn"
            onClick={() => setPrefillNotice(null)}
          >
            Dismiss
          </button>
        </div>
      )}
    </>
  );
}

// CucumberEditor edits a Cucumber test's scenario and its type.
export function CucumberEditor({ s, a, p }: { s: TestDetailState; a: TestDetailActions; p: TestDetailProps }) {
  const { cucumberScenario, setCucumberScenario, cucumberType, setCucumberType } = s;
  const { saveField } = a;
  const { readOnly } = p;
  return (
    <>
      <section className="cuke-editor">
        <h4 className="steps-head">Cucumber scenario</h4>
        <label className="cuke-type-label">
          Scenario type{" "}
          <select
            className="detail-input detail-input-inline"
            value={cucumberType}
            disabled={readOnly}
            onChange={(e) => {
              setCucumberType(e.target.value);
              if (!readOnly) saveField("cucumber_type", e.target.value);
            }}
          >
            <option value="Scenario">Scenario</option>
            <option value="Scenario Outline">Scenario Outline</option>
          </select>
        </label>
        <textarea
          className="cuke-scenario mono"
          value={cucumberScenario}
          readOnly={readOnly}
          onChange={(e) => setCucumberScenario(e.target.value)}
          onBlur={() => {
            if (!readOnly) saveField("cucumber_scenario", cucumberScenario);
          }}
          rows={14}
          placeholder="Given ...\nWhen ...\nThen ..."
        />
      </section>
    </>
  );
}

// GenericEditor edits a Generic test's definition.
export function GenericEditor({ s, a, p }: { s: TestDetailState; a: TestDetailActions; p: TestDetailProps }) {
  const { genericDefinition, setGenericDefinition } = s;
  const { saveField } = a;
  const { readOnly } = p;
  return (
    <>
      <section className="generic-editor">
        <h4 className="steps-head">Generic definition</h4>
        <textarea
          className="generic-def mono"
          value={genericDefinition}
          readOnly={readOnly}
          onChange={(e) => setGenericDefinition(e.target.value)}
          onBlur={() => {
            if (!readOnly) saveField("generic_definition", genericDefinition);
          }}
          rows={14}
          placeholder="Enter test definition…"
        />
      </section>
    </>
  );
}
