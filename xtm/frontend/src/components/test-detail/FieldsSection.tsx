import { ChangeTestType, errMsg } from "../../api";
import { LabelsField } from "../LabelsField";
import { ComponentsField } from "../ComponentsField";
import { formatDateTime } from "../../dates";
import { DirtyDot } from "./rows";
import { EXEC_TYPE_OPTIONS } from "./types";
import type { TestDetailState } from "./useTestDetailState";
import type { TestDetailActions } from "./useTestDetailActions";
import type { TestDetailProps } from "./types";

// FieldsSection is the summary and the field list: status, priority, labels,
// components, execution type, folder, and who created and last updated it.
export function FieldsSection({ s, a, p }: { s: TestDetailState; a: TestDetailActions; p: TestDetailProps }) {
  const { profileId, activeProfile, transitions, allStatuses, saveError, setSaveError, summary, setSummary, priority, setPriority, labels, setLabels, execType, setExecType, setPrefillNotice, setLocalReloadKey, caps, meta, test, isDirty } = s;
  const { saveField, saveComponents, moveToFolder, applyTransition } = a;
  const { testKey, folders, readOnly } = p;
  if (!test) return null;
  return (
    <>
      {saveError && (
        <div className="error-text detail-save-error">{saveError}</div>
      )}

      <div className="field-label">
        Summary {isDirty("summary") && <DirtyDot />}
      </div>
      {readOnly ? (
        <p className="detail-input detail-input-static">{summary}</p>
      ) : (
        <input
          className="detail-input"
          value={summary}
          onChange={(e) => setSummary(e.target.value)}
          onBlur={() => saveField("summary", summary)}
          spellCheck
        />
      )}

      <dl className="detail-fields">
        <dt>
          Status {isDirty("status") && <DirtyDot />}
        </dt>
        <dd>
          <div className="status-row">
            <span className="status-pill">{test.status || "—"}</span>
            {caps.supportsWorkflowTransitions ? (
              // Xray: the existing workflow-transition picker, unchanged.
              !readOnly &&
              transitions.length > 0 && (
                <select
                  className="transition-select"
                  value=""
                  onChange={(e) => {
                    if (e.target.value) applyTransition(e.target.value);
                  }}
                >
                  <option value="">Move to…</option>
                  {transitions.map((t) => (
                    <option key={t.id} value={t.to}>
                      {t.name} → {t.to}
                    </option>
                  ))}
                </select>
              )
            ) : (
              // Kiwi (statusModel "settable"): no workflow, just a
              // dropdown of every valid status. Picking one calls the
              // same TransitionTest — the commit engine routes it to
              // Kiwi's case_status field.
              !readOnly &&
              allStatuses.length > 0 && (
                <select
                  className="transition-select"
                  value={test.status || ""}
                  onChange={(e) => {
                    if (e.target.value && e.target.value !== test.status) {
                      applyTransition(e.target.value);
                    }
                  }}
                >
                  {allStatuses.map((s) => (
                    <option key={s} value={s}>
                      {s}
                    </option>
                  ))}
                </select>
              )
            )}
          </div>
        </dd>

        <dt>
          Priority {isDirty("priority") && <DirtyDot />}
        </dt>
        <dd>
          {readOnly ? (
            <span>{priority || "—"}</span>
          ) : (
            <input
              className="detail-input detail-input-inline"
              value={priority}
              onChange={(e) => setPriority(e.target.value)}
              onBlur={() => saveField("priority", priority)}
            />
          )}
        </dd>

        <dt>
          Labels {isDirty("labels") && <DirtyDot />}
        </dt>
        <dd>
          <LabelsField
            profileId={profileId}
            value={labels}
            onChange={setLabels}
            onSave={(v) => saveField("labels", v)}
            readOnly={!!readOnly}
          />
        </dd>

        <dt>
          Components {isDirty("components") && <DirtyDot />}
        </dt>
        <dd>
          <ComponentsField
            profileId={profileId}
            projectKey={activeProfile?.projectKey ?? ""}
            value={test.components ?? []}
            readOnly={!!readOnly}
            onSave={saveComponents}
          />
        </dd>

        {caps.supportsTestTypes && (
          <>
            <dt>
              Execution type {isDirty("exec_type") && <DirtyDot />}
            </dt>
            <dd>
              {readOnly ? (
                <span>{execType || "—"}</span>
              ) : (
                <select
                  className="detail-input detail-input-inline"
                  value={execType}
                  onChange={async (e) => {
                    const next = e.target.value;
                    setExecType(next);
                    try {
                      const res = await ChangeTestType(profileId, testKey, next);
                      // Re-fetch the test so any pre-filled body appears.
                      setLocalReloadKey((k) => k + 1);
                      if (res.canPrefill && !res.prefilled) {
                        setPrefillNotice(res);
                      }
                    } catch (err) {
                      setSaveError(`Type change failed: ${errMsg(err)}`);
                    }
                  }}
                >
                  <option value="">—</option>
                  {EXEC_TYPE_OPTIONS.map((o) => (
                    <option key={o} value={o}>
                      {o}
                    </option>
                  ))}
                </select>
              )}
            </dd>
          </>
        )}

        {caps.supportsFolders && folders.length > 0 && (
          <>
            <dt>
              Folder {isDirty("folder") && <DirtyDot />}
            </dt>
            <dd>
              {readOnly ? (
                <span>{test.folderId || "(repository root)"}</span>
              ) : (
                <select
                  className="detail-input detail-input-inline"
                  value={test.folderId}
                  onChange={(e) => moveToFolder(e.target.value)}
                >
                  <option value="">(repository root)</option>
                  {folders.map((f) => (
                    <option key={f.id} value={f.id}>
                      {f.id}
                    </option>
                  ))}
                </select>
              )}
            </dd>
          </>
        )}

        <dt>Created</dt>
        <dd>{formatDateTime(meta?.created)}</dd>

        <dt>Creator</dt>
        <dd>{meta?.creator || "—"}</dd>

        <dt>Updated</dt>
        <dd>{formatDateTime(meta?.updated || test.updated)}</dd>

        <dt>Updated by</dt>
        <dd>{meta?.updatedBy || "—"}</dd>
      </dl>
    </>
  );
}
