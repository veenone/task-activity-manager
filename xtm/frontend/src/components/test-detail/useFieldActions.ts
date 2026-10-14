import { GetTestContainers, DeallocateTests, GetTestTransitions, GetTestReview, SetTestReview, TransitionTest, AddTestComment, EditTestField, SetTestComponents, CloneTest, MoveTestToFolder, errMsg } from "../../api";
import type { TestCase } from "../../api";
import { keys } from "../../queries/keys";
import type { EditableField } from "./types";
import { REVIEWER_KEY } from "./types";
import type { TestDetailProps } from "./types";
import type { TestDetailState } from "./useTestDetailState";

// useFieldActions saves the test-level fields: the text fields, components,
// folder, status, review verdict and container memberships, and clones the test.
export function useFieldActions(s: TestDetailState, p: TestDetailProps) {
  const { profileId, prompt, confirm, reviewer, reviewNote, setReviewNote, setTransitions, setCloning, setSaveError, caps, queryClient, test } = s;
  const { testKey, onEdited, onCloned, readOnly } = p;

  async function saveField(field: EditableField, value: string) {
    if (readOnly) return;
    if (!test) return;

    let backendValue: string;
    switch (field) {
      case "summary":
        backendValue = test.summary;
        break;
      case "description":
        backendValue = test.description;
        break;
      case "priority":
        backendValue = test.priority;
        break;
      case "labels":
        backendValue = (test.labels ?? []).join(" ");
        break;
      case "cucumber_scenario":
        backendValue = test.cucumberScenario ?? "";
        break;
      case "cucumber_type":
        backendValue = test.cucumberType ?? "";
        break;
      case "generic_definition":
        backendValue = test.genericDefinition ?? "";
        break;
    }
    if (value === backendValue) return;

    setSaveError("");
    try {
      await EditTestField(profileId, testKey, field, value);
      // Reflect the new persisted value locally so subsequent diffs work.
      const updated: TestCase = { ...test };
      switch (field) {
        case "summary":
          updated.summary = value;
          break;
        case "description":
          updated.description = value;
          break;
        case "priority":
          updated.priority = value;
          break;
        case "labels":
          updated.labels = value.split(/\s+/).filter(Boolean);
          break;
        case "cucumber_scenario":
          updated.cucumberScenario = value;
          break;
        case "cucumber_type":
          updated.cucumberType = value;
          break;
        case "generic_definition":
          updated.genericDefinition = value;
          break;
      }
      queryClient.setQueryData(
        keys.test(profileId, testKey),
        updated,
      );
      onEdited();
    } catch (e) {
      setSaveError(`Save failed: ${errMsg(e)}`);
    }
  }

  // saveComponents queues a components edit. The value is encoded in Go
  // (SetTestComponents), so it bypasses saveField's string path.
  async function saveComponents(names: string[]) {
    if (readOnly || !test) return;
    if (JSON.stringify(names) === JSON.stringify(test.components ?? [])) return;
    setSaveError("");
    try {
      await SetTestComponents(profileId, testKey, names);
      queryClient.setQueryData(keys.test(profileId, testKey), { ...test, components: names });
      onEdited();
    } catch (e) {
      setSaveError(`Save failed: ${errMsg(e)}`);
    }
  }

  // deallocateContainer removes this test from a Test Set / Plan / Execution
  // (FR-3.4–3.6) and refreshes the membership list.
  async function deallocateContainer(containerKey: string) {
    if (readOnly) return;
    if (
      !(await confirm({
        title: "Remove from container",
        message: `Remove ${testKey} from ${containerKey}? This change is queued and applied to Jira when you commit.`,
        confirmLabel: "Remove",
      }))
    )
      return;
    setSaveError("");
    try {
      await DeallocateTests(profileId, containerKey, [testKey]);
      const cons = await GetTestContainers(profileId, testKey);
      queryClient.setQueryData(
        keys.testContainers(profileId, testKey),
        cons ?? [],
      );
      onEdited();
    } catch (e) {
      setSaveError(`Remove failed: ${errMsg(e)}`);
    }
  }

  // moveToFolder relocates the test in the Test Repository (FR-13.3). The new
  // folder is reflected locally so the next diff works, then queued for commit.
  async function moveToFolder(folderId: string) {
    if (readOnly) return;
    if (!test || folderId === test.folderId) return;
    setSaveError("");
    try {
      await MoveTestToFolder(profileId, testKey, folderId);
      queryClient.setQueryData(
        keys.test(profileId, testKey),
        (prev: TestCase | undefined) => (prev ? { ...prev, folderId } : prev),
      );
      onEdited();
    } catch (e) {
      setSaveError(`Move failed: ${errMsg(e)}`);
    }
  }

  // cloneThisTest drafts a new local test copying this one's fields and steps
  // (RND_P_4TFINT_05-206), then opens the fresh draft in the detail panel.
  async function cloneThisTest() {
    if (readOnly) return;
    setCloning(true);
    setSaveError("");
    try {
      const tempKey = await CloneTest(profileId, testKey);
      onCloned?.(tempKey);
    } catch (e) {
      setSaveError(`Clone test failed: ${errMsg(e)}`);
      setCloning(false);
    }
  }

  // applyTransition records the workflow transition locally (FR-4.2). After
  // the local write, we re-query for the transitions available from the new
  // status so the next pick reflects the post-transition workflow position.
  async function applyTransition(targetStatus: string) {
    if (readOnly) return;
    if (!test || !targetStatus) return;
    setSaveError("");
    try {
      await TransitionTest(profileId, testKey, targetStatus);
      queryClient.setQueryData(
        keys.test(profileId, testKey),
        (prev: TestCase | undefined) =>
          prev ? { ...prev, status: targetStatus } : prev,
      );
      // FR-4.4: optionally capture a comment for this transition.
      const comment = await prompt({
        title: `Comment for moving to "${targetStatus}"`,
        placeholder: "Optional (leave blank to skip)",
        submitLabel: "Save",
      });
      if (comment && comment.trim()) {
        await AddTestComment(profileId, testKey, comment.trim());
      }
      // Xray (workflow): re-query the transitions available from the new
      // status. Kiwi (settable): there are no transitions to fetch.
      if (caps.supportsWorkflowTransitions) {
        try {
          const ts = await GetTestTransitions(profileId, testKey);
          setTransitions(ts ?? []);
        } catch (e) {
          console.error("re-fetch transitions:", errMsg(e));
        }
      }
      onEdited();
    } catch (e) {
      setSaveError(`Transition failed: ${errMsg(e)}`);
    }
  }

  // setVerdict records (or clears) a review verdict (test review). The reviewer
  // name is remembered across tests via localStorage.
  async function setVerdict(verdict: string) {
    if (readOnly) return;
    const who = reviewer.trim();
    localStorage.setItem(REVIEWER_KEY, who);
    setSaveError("");
    try {
      await SetTestReview(profileId, testKey, verdict, who, reviewNote.trim());
      const rev = await GetTestReview(profileId, testKey);
      queryClient.setQueryData(
        keys.testReview(profileId, testKey),
        rev,
      );
      setReviewNote(rev?.note ?? "");
      onEdited();
    } catch (e) {
      setSaveError(`Review failed: ${errMsg(e)}`);
    }
  }

  return { saveField, saveComponents, deallocateContainer, moveToFolder, cloneThisTest, applyTransition, setVerdict };
}
