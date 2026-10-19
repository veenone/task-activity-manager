import { StepRow, InlineStepsEditor } from "./StepRow";
import type { TestDetailState } from "./useTestDetailState";
import type { TestDetailActions } from "./useTestDetailActions";
import type { TestDetailProps } from "./types";

// StepsSection lists and edits the steps: a step list for Xray, one text
// field for Kiwi.
export function StepsSection({ s, a, p }: { s: TestDetailState; a: TestDetailActions; p: TestDetailProps }) {
  const { profileId, confirm, steps, setSteps, stepsLoading, stepsError, setShowCloneSteps, setShowCallPicker, setShowCrossCall, setShowCrossClone, jiraStepInfo, caps, crossProjectEnabled } = s;
  const parameters = s.dataset.parameters;
  const { refreshSteps, addStep, duplicateStep, moveStep } = a;
  const { testKey, pendingForTest, onEdited, readOnly } = p;
  return (
    <>
      <h4 className="steps-head">
        Steps
        {caps.stepModel === "objects" &&
          pendingForTest.some((p) => p.entityType === "test_step_order") && (
            <span className="steps-reordered" title="Step order changed">
              reordered
            </span>
          )}
        <button
          className="link-btn steps-refresh"
          onClick={refreshSteps}
          disabled={stepsLoading}
          title="Re-fetch steps from Jira"
        >
          {stepsLoading ? "Loading…" : "Refresh"}
        </button>
        {!readOnly && caps.stepModel === "objects" && (
          <button
            className="link-btn steps-clone"
            onClick={() => setShowCloneSteps(true)}
            disabled={stepsLoading}
            title="Append a copy of another test's steps to this one"
          >
            Clone from…
          </button>
        )}
        {!readOnly && caps.stepModel === "objects" && crossProjectEnabled && (
          <button
            className="link-btn steps-clone"
            onClick={() => setShowCrossClone(true)}
            disabled={stepsLoading}
            title="Clone steps from a test in another project"
          >
            ↗ Other project
          </button>
        )}
      </h4>
      {stepsError && <div className="error-text">{stepsError}</div>}
      {caps.stepModel === "objects" ? (
        // Xray: the existing multi-step CRUD list, unchanged.
        <>
          {!stepsError &&
            !stepsLoading &&
            steps.length === 0 &&
            (jiraStepInfo && jiraStepInfo.count > 0 ? (
              <div className="steps-warning">
                ⚠ Jira reports {jiraStepInfo.count} step
                {jiraStepInfo.count === 1 ? "" : "s"} for this test that didn't
                load here. Don't add new steps yet, since that would create
                duplicates.{" "}
                <button className="link-btn" onClick={refreshSteps}>
                  Load from Jira
                </button>
              </div>
            ) : (
              <p className="muted">No steps defined for this test.</p>
            ))}
          {!stepsError &&
            !stepsLoading &&
            steps.length > 0 &&
            steps.every((s) => !s.action && !s.data && !s.expected) && (
              <div className="steps-warning">
                ⚠ These steps loaded without content. This Xray instance may use
                a step format the tool doesn't recognise yet, so avoid editing
                them to prevent overwriting the real steps in Jira.
              </div>
            )}
          {steps.length > 0 && (
            <ol className="steps-list">
              {steps.map((s, i) => (
                <StepRow
                  profileId={profileId}
                  key={s.xrayId}
                  testKey={testKey}
                  step={s}
                  parameters={parameters}
                  pendingForTest={pendingForTest}
                  isFirst={i === 0}
                  isLast={i === steps.length - 1}
                  readOnly={readOnly}
                  confirm={confirm}
                  onMove={(dir) => moveStep(i, dir)}
                  onDuplicate={() => duplicateStep(s)}
                  onLocalChange={(field, value) => {
                    setSteps((prev) =>
                      prev.map((p) =>
                        p.xrayId === s.xrayId ? { ...p, [field]: value } : p,
                      ),
                    );
                  }}
                  onLocalDelete={(xrayId) => {
                    setSteps((prev) => prev.filter((p) => p.xrayId !== xrayId));
                  }}
                  onEdited={onEdited}
                />
              ))}
            </ol>
          )}
          {!readOnly && !stepsError && !stepsLoading && (
            <div className="steps-add-row">
              <button className="link-btn steps-add" onClick={addStep}>
                + Add step
              </button>
              <button
                className="link-btn steps-add"
                onClick={() => setShowCallPicker(true)}
                title="Add a step that calls another test"
              >
                + Call test
              </button>
              {crossProjectEnabled && (
                <button
                  className="link-btn steps-add"
                  onClick={() => setShowCrossCall(true)}
                  title="Call a test from another project"
                >
                  ↗ Other project
                </button>
              )}
            </div>
          )}
        </>
      ) : (
        // Kiwi (stepModel "inline-text"): one flattened step, edited as a
        // single multi-line text field — no add/delete/reorder/call-step.
        !stepsError &&
        !stepsLoading && (
          <InlineStepsEditor
            profileId={profileId}
            testKey={testKey}
            step={steps[0]}
            readOnly={readOnly}
            onLocalChange={(xrayId, value) =>
              setSteps((prev) =>
                prev.map((p) =>
                  p.xrayId === xrayId ? { ...p, action: value } : p,
                ),
              )
            }
            onLocalCreate={(s) => setSteps((prev) => [...prev, s])}
            onEdited={onEdited}
          />
        )
      )}

      {!readOnly && (
        <p className="muted detail-note">
          Your edits are saved locally and queued in <b>Pending</b> until
          you commit them to Jira. Reordering steps will land in a later
          update.
        </p>
      )}
    </>
  );
}
