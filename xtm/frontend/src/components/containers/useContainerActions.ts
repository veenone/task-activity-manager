import { SeedSampleContainers, CleanSampleData, CreateContainerAndAllocate, EditContainer, DeleteContainer, SetContainerEnvironments, DeallocateTests, SetTestRunStatus, BulkSetTestRunStatus, UnlinkBugFromRun, SetTestRunComment, BulkEditContainers, ExportPytest, BrowserOpenURL, GetRunRollupBreakdown, errMsg } from "../../api";
import { usePrompt } from "../usePrompt";
import { useConfirm } from "../useConfirm";
import { useNotice } from "../useNotice";
import type { ContainerState } from "./useContainerState";

export interface ContainerViewProps {
  onChanged: () => void;
  // Sample-data generation is a demo aid — only offered for demo profiles so a
  // user can't seed fake containers into a real project (FR-5).
  isDemo: boolean;
  jiraUrl?: string;
  onOpenTest?: (testKey: string) => void;
}

// useContainerActions is everything the Containers view does: run results,
// defect links and remarks on members, rename, environments (one execution or
// every shown one), container create / rename / delete, sample data, the
// pytest scaffold and the roll-up breakdown. Each write is queued for commit.
export function useContainerActions(s: ContainerState, p: ContainerViewProps) {
  const {
    kind, envDraft, setEnvDraft, batchEnvOp, batchEnvName, setBatchEnvBusy, rowSortField,
    setRowSortField, rowSortDesc, setRowSortDesc, selected, setSelected, setError, setSeeding,
    selectedRuns, setSelectedRuns, bulkStatus, setEditingName, nameDraft, setBreakdownStatus,
    breakdown, setBreakdown, breakdownFor, setBreakdownFor, setBreakdownLoading, profileId,
    containers, kindLabel, selectedContainer, viewContainers,
  } = s;
  const { onChanged, isDemo, jiraUrl } = p;
  const { prompt } = usePrompt();
  const { confirm } = useConfirm();
  const { notice } = useNotice();

  // removeTest unassigns a single Test from the selected container (FR-3.4–3.6),
  // queued for commit.
  async function removeTest(testKey: string) {
    if (!selected) return;
    setError("");
    try {
      await DeallocateTests(profileId, selected, [testKey]);
      onChanged();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  // setRunStatus updates a Test's run result in the selected Test Execution,
  // queued for commit to Xray.
  async function setRunStatus(testKey: string, status: string) {
    if (!selected || !status) return;
    setError("");
    try {
      await SetTestRunStatus(profileId, selected, testKey, status);
      onChanged();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  // unlinkBug removes one defect link from a Test's run result in the
  // selected Test Execution, queued for commit to Xray.
  async function unlinkBug(testKey: string, bugKey: string) {
    if (!selected) return;
    setError("");
    try {
      await UnlinkBugFromRun(profileId, selected, testKey, bugKey);
      onChanged();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  // setRunComment updates a Test's run remark in the selected Test Execution,
  // queued for commit to Xray.
  async function setRunComment(testKey: string, comment: string) {
    if (!selected) return;
    setError("");
    try {
      await SetTestRunComment(profileId, selected, testKey, comment);
      onChanged();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  // commitInlineRename saves the detail card's editable name (an inline path to
  // the same rename CRUD as the Actions menu).
  async function commitInlineRename() {
    setEditingName(false);
    const name = nameDraft.trim();
    if (!selectedContainer || !name || name === selectedContainer.summary) return;
    setError("");
    try {
      await EditContainer(profileId, selected, name);
      onChanged();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  // addEnvironment / removeEnvironment edit the selected execution's Test
  // Environments, computing the new set and queuing it via
  // SetContainerEnvironments (committed to Jira as a custom-field update).
  async function addEnvironment() {
    const name = envDraft.trim();
    if (!selectedContainer || !name) return;
    const cur = selectedContainer.environments ?? [];
    if (cur.includes(name)) {
      setEnvDraft("");
      return;
    }
    setEnvDraft("");
    setError("");
    try {
      await SetContainerEnvironments(profileId, selected, [...cur, name]);
      onChanged();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  async function removeEnvironment(name: string) {
    if (!selectedContainer) return;
    const cur = selectedContainer.environments ?? [];
    setError("");
    try {
      await SetContainerEnvironments(
        profileId,
        selected,
        cur.filter((e) => e !== name),
      );
      onChanged();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  // applyBatchEnv runs one environment operation across every execution in the
  // current filtered view (viewContainers), via the bulk BulkEditContainers
  // path. set_env replaces each execution's environments, so it is confirmed.
  async function applyBatchEnv() {
    const name = batchEnvName.trim();
    const keys = viewContainers.map((c) => c.key);
    if (keys.length === 0) return;
    if (batchEnvOp !== "set_env" && !name) return;
    if (
      batchEnvOp === "set_env" &&
      !(await confirm({
        title: "Replace environments",
        message:
          `Set environments to "${name || "(none)"}" on ${keys.length} execution${keys.length === 1 ? "" : "s"}? ` +
          "This will replace their current environments.",
        confirmLabel: "Replace",
        danger: true,
      }))
    )
      return;
    setBatchEnvBusy(true);
    setError("");
    try {
      const res = await BulkEditContainers(profileId, keys, {
        operation: batchEnvOp,
        field: "",
        value: name,
      });
      if (res.failed && res.failed.length > 0) {
        setError(
          `Updated ${res.succeeded.length}, failed ${res.failed.length}: ${res.failed[0].error}`,
        );
      }
      onChanged();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBatchEnvBusy(false);
    }
  }

  async function seed() {
    setSeeding(true);
    setError("");
    try {
      await SeedSampleContainers(profileId);
      onChanged();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setSeeding(false);
    }
  }

  // cleanSample removes the sample containers a previous seed created, so a real
  // project can start fresh. Real synced data is untouched (FR-5).
  async function cleanSample() {
    if (
      !(await confirm({
        title: "Clean sample data",
        message:
          "Remove all sample Test Sets, Plans, and Executions created by 'Regenerate sample data'? " +
          "Your real synced containers won't be affected.",
        confirmLabel: "Delete",
        danger: true,
      }))
    )
      return;
    setError("");
    try {
      const removed = await CleanSampleData(profileId);
      await notice({
        title: "Sample data cleaned",
        message:
          removed > 0
            ? `Removed ${removed} sample container${removed === 1 ? "" : "s"}.`
            : "No sample data found for this project.",
      });
      setSelected("");
      onChanged();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  // toggleRun / toggleRunAll manage the bulk execution-result selection.
  function toggleRun(testKey: string) {
    setSelectedRuns((prev) => {
      const next = new Set(prev);
      if (next.has(testKey)) next.delete(testKey);
      else next.add(testKey);
      return next;
    });
  }

  function toggleRowSort(field: string) {
    if (rowSortField === field) {
      setRowSortDesc((d) => !d);
    } else {
      setRowSortField(field);
      setRowSortDesc(false);
    }
  }
  function rowSortIndicator(field: string): string {
    if (rowSortField !== field) return "";
    return rowSortDesc ? " ↓" : " ↑";
  }

  // applyBulkRunStatus sets one result on every selected Test in the execution.
  async function applyBulkRunStatus() {
    if (!selected || selectedRuns.size === 0) return;
    setError("");
    try {
      const res = await BulkSetTestRunStatus(
        profileId,
        selected,
        [...selectedRuns],
        bulkStatus,
      );
      if (res.failed && res.failed.length > 0) {
        setError(
          `Set ${res.succeeded.length}, failed ${res.failed.length}: ${res.failed[0].error}`,
        );
      }
      setSelectedRuns(new Set());
      onChanged();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  function openParent(parentKey: string) {
    const base = (jiraUrl ?? "").trim().replace(/\/+$/, "");
    if (base && !isDemo && !parentKey.startsWith("NEW-")) {
      BrowserOpenURL(`${base}/browse/${parentKey}`);
    }
  }
  // openBug mirrors BugsPanel.openBug: open the defect in Jira when a real URL
  // exists (skip in demo mode and for not-yet-committed NEW- keys).
  function openBug(bugKey: string) {
    const base = (jiraUrl ?? "").trim().replace(/\/+$/, "");
    if (base && !isDemo && !bugKey.startsWith("NEW-")) {
      BrowserOpenURL(`${base}/browse/${bugKey}`);
    }
  }

  async function newContainer() {
    const name = await prompt({
      title: `New ${kindLabel}`,
      placeholder: `${kindLabel} name`,
      submitLabel: "Create",
    });
    if (!name || !name.trim()) return;
    setError("");
    try {
      await CreateContainerAndAllocate(profileId, kind, name.trim(), []);
      onChanged();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  async function renameContainer() {
    if (!selected) return;
    const cur = containers.find((c) => c.key === selected);
    const name = await prompt({
      title: `Rename ${kindLabel}`,
      defaultValue: cur?.summary ?? "",
      submitLabel: "Rename",
    });
    if (name === null || !name.trim()) return;
    setError("");
    try {
      await EditContainer(profileId, selected, name.trim());
      onChanged();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  async function generatePytest(style: string) {
    if (!selected) return;
    setError("");
    try {
      const path = await ExportPytest(profileId, selected, style);
      if (path) await notice({ title: "Scaffold saved", message: path });
    } catch (e) {
      setError(errMsg(e));
    }
  }

  async function deleteContainer() {
    if (!selected) return;
    if (
      !(await confirm({
        title: `Delete ${kindLabel}`,
        message: `Delete this ${kindLabel}? Its test memberships are removed too (committed on sync).`,
        confirmLabel: "Delete",
        danger: true,
      }))
    )
      return;
    setError("");
    try {
      await DeleteContainer(profileId, selected);
      setSelected("");
      onChanged();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  // openBreakdown opens the informational modal for one roll-up bucket, fetching
  // the member breakdown for the selected container on first use.
  async function openBreakdown(status: string) {
    if (!selected) return;
    setBreakdownStatus(status);
    if (breakdownFor === selected && breakdown) return;
    setBreakdownLoading(true);
    try {
      const rows = await GetRunRollupBreakdown(profileId, selected);
      setBreakdown(rows ?? []);
      setBreakdownFor(selected);
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBreakdownLoading(false);
    }
  }

  return { removeTest, setRunStatus, unlinkBug, setRunComment, commitInlineRename, addEnvironment, removeEnvironment, applyBatchEnv, seed, cleanSample, toggleRun, toggleRowSort, rowSortIndicator, applyBulkRunStatus, openParent, openBug, newContainer, renameContainer, generatePytest, deleteContainer, openBreakdown };
}

export type ContainerActions = ReturnType<typeof useContainerActions>;
