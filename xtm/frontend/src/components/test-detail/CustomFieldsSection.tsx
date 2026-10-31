import { CustomFieldRow } from "./rows";
import { fieldGroups } from "./customFieldScopes";
import type { TestDetailState } from "./useTestDetailState";
import type { TestDetailProps } from "./types";

// CustomFieldsSection lists the custom fields, collapsed until opened.
//
// A backend can report more than one namespace: Jira's own fields on the test
// issue, which are the editable ones, and on a Zephyr profile also fields that
// belong to an execution or to a step. Those are labelled and read-only,
// because a custom field edit commits as a Jira issue update and there is
// nothing on the issue for them to update (#371 Phase 3). A backend with one
// namespace gets no labels at all, so an Xray profile looks unchanged.
export function CustomFieldsSection({ s, p }: { s: TestDetailState; p: TestDetailProps }) {
  const { profileId, customFields, setCustomFields, customFieldsOpen, setCustomFieldsOpen } = s;
  const { testKey, pendingForTest, onEdited, readOnly } = p;
  const groups = fieldGroups(customFields);
  return (
    <>
      {customFields.length > 0 && (
        <>
          <h4 className="detail-collapse-h4">
            <button
              type="button"
              className="detail-collapse-toggle"
              onClick={() => setCustomFieldsOpen((o) => !o)}
              aria-expanded={customFieldsOpen}
              title={
                customFieldsOpen
                  ? "Hide custom fields"
                  : "Show custom fields"
              }
            >
              <span
                className="detail-collapse-chevron"
                style={{
                  transform: customFieldsOpen ? "rotate(90deg)" : "none",
                }}
                aria-hidden="true"
              >
                ▶
              </span>
              Custom Fields
              <span className="detail-collapse-count">
                {customFields.length}
              </span>
            </button>
          </h4>
          {customFieldsOpen &&
            groups.map((g) => (
              <div key={g.label}>
                {groups.length > 1 && (
                  <p className="muted detail-note">
                    {g.label}
                    {!g.editable && ". Read-only here: these fields belong to a run, not to the test."}
                  </p>
                )}
                <dl className="detail-fields">
                  {g.fields.map((f) => (
                    <CustomFieldRow
                      profileId={profileId}
                      key={f.fieldId}
                      testKey={testKey}
                      field={f}
                      pendingForTest={pendingForTest}
                      readOnly={readOnly || !g.editable}
                      onLocalChange={(value) =>
                        setCustomFields((prev) =>
                          prev.map((p) =>
                            p.fieldId === f.fieldId ? { ...p, value } : p,
                          ),
                        )
                      }
                      onEdited={onEdited}
                    />
                  ))}
                </dl>
              </div>
            ))}
        </>
      )}
    </>
  );
}
