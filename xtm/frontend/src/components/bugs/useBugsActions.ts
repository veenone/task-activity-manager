import { SyncBugs, CreateContainerAndAllocate, ListContainers, AllocateTests, ExportBugsWithRunHistory, errMsg } from "../../api";
import type { BugsState } from "./useBugsState";

// useBugsActions is what the Bugs panel does: a bugs-only sync, the Excel
// export, and the two ways to put the checked bugs' tests into a Test
// Execution (a new one, or an existing one picked in a dialog).
export function useBugsActions(s: BugsState) {
  const { profileId, prompt, setError, setNotice, selected, checked, setChecked, setCreating, setAddToExecOpen, setAddToExecLoading, setAddToExecExecs, addToExecTarget, setAddToExecTarget, setAddToExecFilter, setAddToExecWorking, setAddToExecError, setSyncing, setExporting, bugsQuery, bugs, unionTestKeys } = s;

  // syncBugs refreshes only the defect issues from Jira (partial sync), so the
  // Bugs panel can update without re-running preconditions / containers /
  // requirements (RND_P_4TFINT_05-214).
  async function syncBugs() {
    setSyncing(true);
    setError("");
    setNotice("");
    try {
      await SyncBugs(profileId);
      void bugsQuery.refetch();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setSyncing(false);
    }
  }

  // Export the checked bugs (or the open bug if none are checked) together with
  // their full run history to an Excel workbook.
  async function exportBugs() {
    const bugKeys =
      checked.size > 0
        ? bugs.filter((b) => checked.has(b.key)).map((b) => b.key)
        : selected && !selected.startsWith("NEW-")
          ? [selected]
          : [];
    if (bugKeys.length === 0) return;
    setExporting(true);
    setError("");
    setNotice("");
    try {
      const path = await ExportBugsWithRunHistory(profileId, bugKeys);
      if (path) {
        setNotice(
          `Exported ${bugKeys.length} bug${bugKeys.length === 1 ? "" : "s"} to ${path}`,
        );
      }
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setExporting(false);
    }
  }

  // Create a Test Execution whose members are the union of the checked bugs'
  // linked tests, to isolate a run that verifies only those bugs
  // (RND_P_4TFINT_05-222).
  async function createExecFromBugs() {
    if (unionTestKeys.length === 0) return;
    const checkedKeys = bugs
      .filter((b) => checked.has(b.key))
      .map((b) => b.key);
    const joined = checkedKeys.join(", ");
    const defaultName =
      joined.length > 60
        ? `Verify bugs: ${joined.slice(0, 57)}...`
        : `Verify bugs: ${joined}`;
    const name = await prompt({
      title: "New Test Execution",
      defaultValue: defaultName,
      placeholder: "Test Execution name",
      submitLabel: "Create",
    });
    if (!name || !name.trim()) return;
    setCreating(true);
    setError("");
    setNotice("");
    try {
      const res = await CreateContainerAndAllocate(
        profileId,
        "testexec",
        name.trim(),
        unionTestKeys,
      );
      setNotice(
        `Created Test Execution ${res.tempKey} with ${res.added} test${
          res.added === 1 ? "" : "s"
        }. It will appear in Containers.`,
      );
      setChecked(new Set());
      void bugsQuery.refetch();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setCreating(false);
    }
  }

  // Open the "Add tests to existing execution" modal: load the list of Test
  // Executions then show the picker.
  async function openAddToExec() {
    if (unionTestKeys.length === 0) return;
    setAddToExecOpen(true);
    setAddToExecError("");
    setAddToExecFilter("");
    setAddToExecTarget("");
    setAddToExecExecs([]);
    setAddToExecLoading(true);
    try {
      const cs = await ListContainers(profileId, "testexec");
      const list = cs ?? [];
      setAddToExecExecs(list);
      if (list.length > 0) setAddToExecTarget(list[0].key);
    } catch (e) {
      setAddToExecError(errMsg(e));
    } finally {
      setAddToExecLoading(false);
    }
  }

  // Confirm: allocate the union test keys to the chosen execution.
  async function confirmAddToExec() {
    if (!addToExecTarget || unionTestKeys.length === 0) return;
    setAddToExecWorking(true);
    setAddToExecError("");
    try {
      const r = await AllocateTests(profileId, addToExecTarget, unionTestKeys);
      const added = r.added.length;
      const already = r.alreadyMembers.length;
      setAddToExecOpen(false);
      setNotice(
        `Added ${added} test${added === 1 ? "" : "s"} to ${addToExecTarget}` +
          (already > 0 ? ` (${already} already present).` : "."),
      );
      setChecked(new Set());
    } catch (e) {
      setAddToExecError(errMsg(e));
    } finally {
      setAddToExecWorking(false);
    }
  }

  return { syncBugs, exportBugs, createExecFromBugs, openAddToExec, confirmAddToExec };
}

export type BugsActions = ReturnType<typeof useBugsActions>;
