import { EditPreconditionField, DeletePrecondition, BulkAssociatePreconditions, SyncPreconditions, errMsg } from "../../api";
import type { PreconditionsViewProps, PreconditionsState } from "./usePreconditionsState";

// usePreconditionsActions is what the view does: a preconditions-only sync,
// saving the edited fields, unlinking a test, and deleting the precondition.
export function usePreconditionsActions(s: PreconditionsState, p: PreconditionsViewProps) {
  const { profileId, confirm, beginSync, endSync, canSync, selected, setSelected, setError, setNotice, setEditing, setSyncing, summaryDraft, setSummaryDraft, descDraft, setDescDraft, condDraft, setCondDraft, typeDraft, setTypeDraft, selectedPre } = s;
  const { onChanged } = p;

  async function syncPreconditions() {
    if (!canSync) return;
    setError("");
    setNotice("");
    setSyncing(true);
    beginSync();
    try {
      await SyncPreconditions(profileId);
      onChanged();
      setNotice("Preconditions refreshed from Jira.");
    } catch (e) {
      setError(errMsg(e));
    } finally {
      endSync();
      setSyncing(false);
    }
  }

  // saveField persists an inline field edit (summary / type / description / condition).
  async function saveField(
    field: "summary" | "type" | "description" | "condition",
    value: string,
  ) {
    if (!selected || !selectedPre) return;
    const current = selectedPre[field];
    if (value === current) return;
    setError("");
    try {
      await EditPreconditionField(profileId, selected, field, value);
      onChanged();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  // handleSave commits all draft values that differ from the stored values and
  // then exits edit mode. Fields that are unchanged are skipped (saveField
  // short-circuits on equality). Errors surface in the existing error banner.
  async function handleSave() {
    await saveField("summary", summaryDraft);
    await saveField("type", typeDraft);
    await saveField("condition", condDraft);
    await saveField("description", descDraft);
    setEditing(false);
  }

  // handleCancel discards all draft changes and returns to read-only mode.
  function handleCancel() {
    setSummaryDraft(selectedPre?.summary ?? "");
    setDescDraft(selectedPre?.description ?? "");
    setCondDraft(selectedPre?.condition ?? "");
    setTypeDraft(selectedPre?.type || "Manual");
    setEditing(false);
  }

  async function removeTest(testKey: string) {
    if (!selected) return;
    setError("");
    try {
      await BulkAssociatePreconditions(profileId, [testKey], [selected], false);
      onChanged();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  async function deletePrecondition() {
    if (!selectedPre) return;
    if (
      !(await confirm({
        title: "Delete precondition",
        message:
          `Delete precondition ${selectedPre.key}? It will be unlinked from ` +
          `${selectedPre.testCount} test${selectedPre.testCount === 1 ? "" : "s"} ` +
          `and removed from Jira on commit.`,
        confirmLabel: "Delete",
        danger: true,
      }))
    )
      return;
    setError("");
    try {
      await DeletePrecondition(profileId, selected);
      setSelected("");
      onChanged();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  return { syncPreconditions, saveField, handleSave, handleCancel, removeTest, deletePrecondition };
}

export type PreconditionsActions = ReturnType<typeof usePreconditionsActions>;
