import { MultiAddSelect } from "../MultiAddSelect";
import { PreconditionRow, ContainerSection, DirtyDot } from "./rows";
import type { TestDetailState } from "./useTestDetailState";
import type { TestDetailActions } from "./useTestDetailActions";
import type { TestDetailProps } from "./types";

// PreconditionsSection lists linked preconditions and links more.
export function PreconditionsSection({ s, a, p }: { s: TestDetailState; a: TestDetailActions; p: TestDetailProps }) {
  const { profileId, precondLoading, precondError, setShowCrossPrecond, setSwapKind, caps, crossProjectEnabled, preconditions, allPreconditions, isDirty } = s;
  const { refreshPreconditions, removePrecondition, addPreconditions, createAndAssociatePrecondition } = a;
  const { onEdited, readOnly } = p;
  return (
    <>
      {caps.supportsPreconditionObjects && (
        <>
          <h4>
            Preconditions {isDirty("preconditions") && <DirtyDot />}
            <button
              className="link-btn steps-refresh"
              onClick={refreshPreconditions}
              disabled={precondLoading}
              title="Re-fetch preconditions from Jira"
            >
              {precondLoading ? "Loading…" : "Refresh"}
            </button>
          </h4>
          {precondError && (
            <p className="error-text">{precondError}</p>
          )}
          {preconditions.length === 0 ? (
            <p className="muted">None linked</p>
          ) : (
            <ul className="pre-list">
              {preconditions.map((p) => (
                <PreconditionRow
                  profileId={profileId}
                  key={p.key}
                  precondition={p}
                  readOnly={readOnly}
                  onRemove={removePrecondition}
                  onEdited={onEdited}
                />
              ))}
            </ul>
          )}
          {!readOnly && (
            <div className="pre-add pre-add-row">
              <MultiAddSelect
                placeholder="+ Add precondition…"
                onAdd={addPreconditions}
                options={allPreconditions
                  .filter((p) => !preconditions.some((lp) => lp.key === p.key))
                  .map((p) => ({
                    value: p.key,
                    label: `${p.key} — ${p.summary}`,
                  }))}
              />
              <button
                type="button"
                className="btn btn-ghost pre-add-new"
                onClick={createAndAssociatePrecondition}
                title="Create a brand-new precondition and link it"
              >
                ＋ New
              </button>
              {crossProjectEnabled && (
                <button
                  type="button"
                  className="btn btn-ghost pre-add-new"
                  onClick={() => setShowCrossPrecond(true)}
                  title="Link a precondition from another project"
                >
                  ↗ Other project
                </button>
              )}
              <button
                type="button"
                className="btn btn-ghost pre-add-new"
                onClick={() => setSwapKind("precondition")}
                disabled={preconditions.length === 0 && allPreconditions.length === 0}
                title="Swap preconditions: remove some and add others in one apply"
              >
                ⇄ Swap
              </button>
            </div>
          )}
        </>
      )}
    </>
  );
}

// MembershipsSection lists the sets, plans and executions holding the test.
export function MembershipsSection({ s, a, p }: { s: TestDetailState; a: TestDetailActions; p: TestDetailProps }) {
  const { containers } = s;
  const { deallocateContainer } = a;
  const { readOnly } = p;
  return (
    <>
      {(() => {
        const sets = containers.filter((c) => c.kind === "testset");
        const plans = containers.filter((c) => c.kind === "testplan");
        const execs = containers.filter((c) => c.kind === "testexec");
        if (containers.length === 0) {
          return (
            <>
              <h4>Memberships</h4>
              <p className="muted">
                This test isn't in any set, plan, or execution yet.
              </p>
            </>
          );
        }
        return (
          <>
            {sets.length > 0 && (
              <ContainerSection
                title="Test Sets"
                items={sets}
                readOnly={readOnly}
                onRemove={deallocateContainer}
              />
            )}
            {plans.length > 0 && (
              <ContainerSection
                title="Test Plans"
                items={plans}
                readOnly={readOnly}
                onRemove={deallocateContainer}
              />
            )}
            {execs.length > 0 && (
              <ContainerSection
                title="Test Executions"
                items={execs}
                showRunStatus
                readOnly={readOnly}
                onRemove={deallocateContainer}
              />
            )}
          </>
        );
      })()}
    </>
  );
}

// RequirementsSection lists covered requirements and links more.
export function RequirementsSection({ s, a, p }: { s: TestDetailState; a: TestDetailActions; p: TestDetailProps }) {
  const { setSwapKind, caps, requirements, allRequirements, isDirty } = s;
  const { addRequirements, removeRequirement } = a;
  const { readOnly } = p;
  return (
    <>
      {caps.supportsRequirementObjects && (
        <>
          <h4>
            Requirements {isDirty("requirements") && <DirtyDot />}
          </h4>
          {requirements.length === 0 ? (
            <p className="muted">
              This test isn't linked to any requirement yet.
            </p>
          ) : (
            <ul className="pre-list req-link-list">
              {requirements.map((rq) => (
                <li key={rq.key}>
                  <span className="mono">{rq.key}</span>
                  <span className="muted req-link-project">{rq.projectKey}</span>
                  <span className="req-link-summary">{rq.summary}</span>
                  {rq.status && (
                    <span className="status-pill req-link-status">
                      {rq.status}
                    </span>
                  )}
                  {!readOnly && (
                    <button
                      className="btn btn-ghost pre-remove"
                      onClick={() => removeRequirement(rq.key)}
                      title="Unlink this requirement"
                      aria-label="Unlink this requirement"
                    >
                      ✕
                    </button>
                  )}
                </li>
              ))}
            </ul>
          )}
          {!readOnly && (
            <div className="pre-add pre-add-row">
              <MultiAddSelect
                placeholder="+ Link requirement…"
                onAdd={addRequirements}
                options={allRequirements
                  .filter((r) => !requirements.some((lr) => lr.key === r.key))
                  .map((r) => ({
                    value: r.key,
                    label: `${r.key} — ${r.summary}`,
                  }))}
              />
              <button
                type="button"
                className="btn btn-ghost pre-add-new"
                onClick={() => setSwapKind("requirement")}
                disabled={requirements.length === 0 && allRequirements.length === 0}
                title="Swap requirements: unlink some and link others in one apply"
              >
                ⇄ Swap
              </button>
            </div>
          )}
        </>
      )}
    </>
  );
}

// BugsSection lists the bugs linked to the test.
export function BugsSection({ s }: { s: TestDetailState }) {
  const { canLinkToJira, bugs, openBugInJira } = s;
  return (
    <>
      <h4>Bugs</h4>
      {bugs.length === 0 ? (
        <p className="muted">No bugs linked yet.</p>
      ) : (
        <ul className="pre-list bug-link-list">
          {bugs.map((b) => (
            <li key={b.key}>
              {canLinkToJira && !b.key.startsWith("NEW-") ? (
                <button
                  className="mono bug-link-key"
                  onClick={() => openBugInJira(b.key)}
                  title={`Open ${b.key} in Jira (browser)`}
                >
                  {b.key}
                </button>
              ) : (
                <span className="mono">{b.key}</span>
              )}
              <span className="muted req-link-project">{b.projectKey}</span>
              <span className="req-link-summary">{b.summary}</span>
              {b.status && (
                <span className="status-pill req-link-status">{b.status}</span>
              )}
            </li>
          ))}
        </ul>
      )}
    </>
  );
}
