import { CustomFieldRow } from "./rows";
import type { TestDetailState } from "./useTestDetailState";
import type { TestDetailProps } from "./types";

// CustomFieldsSection lists the Jira custom fields, collapsed until opened.
export function CustomFieldsSection({ s, p }: { s: TestDetailState; p: TestDetailProps }) {
  const { profileId, customFields, setCustomFields, customFieldsOpen, setCustomFieldsOpen } = s;
  const { testKey, pendingForTest, onEdited, readOnly } = p;
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
          {customFieldsOpen && (
            <dl className="detail-fields">
              {customFields.map((f) => (
                <CustomFieldRow
                  profileId={profileId}
                  key={f.fieldId}
                  testKey={testKey}
                  field={f}
                  pendingForTest={pendingForTest}
                  readOnly={readOnly}
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
          )}
        </>
      )}
    </>
  );
}
