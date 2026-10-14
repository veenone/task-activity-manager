import { GetTestSteps, CheckJiraTestSteps, AddTestStep, AddCalledTestStep, ReorderTestSteps, errMsg } from "../../api";
import type { Step } from "../../api";

import type { TestDetailProps } from "./types";
import type { TestDetailState } from "./useTestDetailState";

// useStepActions refreshes, adds, duplicates and reorders steps.
export function useStepActions(s: TestDetailState, p: TestDetailProps) {
  const { profileId, steps, setSteps, setStepsLoading, setStepsError, setJiraStepInfo, setSaveError } = s;
  const { testKey, onEdited, readOnly } = p;

  // refreshSteps re-fetches Steps from Xray, bypassing the cache. Useful
  // after someone else edits steps directly in Jira.
  async function refreshSteps() {
    setStepsLoading(true);
    setStepsError("");
    try {
      const s = await GetTestSteps(profileId, testKey, true);
      setSteps(s ?? []);
      // Refresh resolved the load — clear the warning. If it's still empty,
      // re-check Jira so the banner reflects reality.
      if ((s ?? []).length > 0) {
        setJiraStepInfo(null);
      } else {
        try {
          setJiraStepInfo(await CheckJiraTestSteps(profileId, testKey));
        } catch (e) {
          console.error("check jira steps:", errMsg(e));
        }
      }
    } catch (e) {
      setStepsError(errMsg(e));
    } finally {
      setStepsLoading(false);
    }
  }

  // addStep appends a new, empty step locally (FR-2.5). The backend returns
  // it with a temporary id; the user fills the fields in place (each blur
  // folds into the queued create) and it lands in Jira on commit.
  async function addStep() {
    if (readOnly) return;
    setSaveError("");
    try {
      const s = await AddTestStep(profileId, testKey, "", "", "");
      setSteps((prev) => [...prev, s]);
      onEdited();
    } catch (e) {
      setSaveError(`Add step failed: ${errMsg(e)}`);
    }
  }

  // duplicateStep appends a copy of an existing step on this same test
  // (RND_P_4TFINT_05-204) — a call step clones its called test, a manual step
  // clones its action/data/expected. The copy is a new queued step the user can
  // reorder; it lands in Jira on commit like any added step.
  async function duplicateStep(step: Step) {
    if (readOnly) return;
    setSaveError("");
    try {
      const s = step.calledTestKey
        ? await AddCalledTestStep(profileId, testKey, step.calledTestKey)
        : await AddTestStep(profileId, testKey, step.action, step.data, step.expected);
      setSteps((prev) => [...prev, s]);
      onEdited();
    } catch (e) {
      setSaveError(`Duplicate step failed: ${errMsg(e)}`);
    }
  }

  // moveStep swaps a step with its neighbour and persists the whole new order
  // (FR-2.5). The reorder is a single test-level pending change; on failure we
  // roll the local list back so the UI matches what was actually saved.
  async function moveStep(index: number, dir: "up" | "down") {
    if (readOnly) return;
    const target = dir === "up" ? index - 1 : index + 1;
    if (target < 0 || target >= steps.length) return;
    const previous = steps;
    const next = [...steps];
    [next[index], next[target]] = [next[target], next[index]];
    setSteps(next);
    setSaveError("");
    try {
      await ReorderTestSteps(
        profileId,
        testKey,
        next.map((s) => s.xrayId),
      );
      onEdited();
    } catch (e) {
      setSteps(previous);
      setSaveError(`Reorder failed: ${errMsg(e)}`);
    }
  }

  return { refreshSteps, addStep, duplicateStep, moveStep };
}
