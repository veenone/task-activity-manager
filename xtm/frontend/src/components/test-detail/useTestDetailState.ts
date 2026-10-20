import { useEffect, useState } from "react";
import { useProfile } from "../../contexts/ProfileContext";
import { GetProfileCrossProjectSources, BrowserOpenURL, errMsg, isDemoUrl } from "../../api";
import type { TypeConversion, Transition, Step, CustomFieldValue, JiraStepInfo, TestRunEntry } from "../../api";
import { usePrompt } from "../usePrompt";
import { useConfirm } from "../useConfirm";
import { useCapabilities } from "../../features";
import { useQueryClient } from "@tanstack/react-query";
import { useTest, useTestBugs, useTestContainers, useTestMeta, useTestPreconditions, useTestRequirements, useTestReview, useTestRunHistory, useAllPreconditions, useRequirementCoverage } from "../../queries/testDetail";
import type { TestDetailProps } from "./types";
import { REVIEWER_KEY } from "./types";
import { useTestDataset } from "../../queries/datasets";

// useTestDetailState holds what the panel knows about one test: the queries
// behind each section, the draft buffers the fields edit, the steps and custom
// fields it loads itself, which pickers are open, and the panel width.
export function useTestDetailState(p: TestDetailProps) {
  const { testKey, version, pendingForTest, jiraUrl } = p;
  const { activeId: profileId, activeProfile } = useProfile();
  const { prompt } = usePrompt();
  const { confirm } = useConfirm();
  // Gates the Xray-shaped sections below (preconditions, requirements, exec
  // type, folder) to what the active profile's backend actually supports
  // (P6.2a). Status/step editing is left un-gated here — that's P6.2b.
  const caps = useCapabilities(profileId);
  const datasetQuery = useTestDataset(profileId, testKey, caps.supportsDatasets);
  const dataset = datasetQuery.data ?? { parameters: [], rows: [] };

  // The test key links to its real Jira issue, opened in the system browser
  // (RND_P_4TFINT_05-211). Suppressed for demo profiles and for uncommitted
  // "NEW-" drafts, which have no Jira URL yet.
  const isDemoProfile = isDemoUrl(jiraUrl);
  const canLinkToJira =
    !!jiraUrl && !isDemoProfile && !testKey.startsWith("NEW-");
  function openInJira() {
    const base = (jiraUrl ?? "").trim().replace(/\/+$/, "");
    if (base) BrowserOpenURL(`${base}/browse/${testKey}`);
  }
  function openBugInJira(bugKey: string) {
    const base = (jiraUrl ?? "").trim().replace(/\/+$/, "");
    if (base) BrowserOpenURL(`${base}/browse/${bugKey}`);
  }

  // Resizeable panel width (FR-11) — drag the left edge to widen for long
  // descriptions / steps; the width persists across sessions.
  const [width, setWidth] = useState<number>(() => {
    const saved = Number(localStorage.getItem("xtm.detailWidth"));
    return saved >= 320 && saved <= 900 ? saved : 440;
  });
  useEffect(() => {
    localStorage.setItem("xtm.detailWidth", String(width));
  }, [width]);

  // Load the profile's cross-project source projects once, to gate the
  // "Other project" link buttons (RND_P_4TFINT_05-322).
  useEffect(() => {
    let cancelled = false;
    GetProfileCrossProjectSources(profileId)
      .then((s) => {
        if (!cancelled) setCrossProjectSources(s ?? "");
      })
      .catch(() => {
        if (!cancelled) setCrossProjectSources("");
      });
    return () => {
      cancelled = true;
    };
  }, [profileId]);

  function startResize(e: React.MouseEvent) {
    e.preventDefault();
    const startX = e.clientX;
    const startW = width;
    // The panel is anchored to the right, so dragging left (negative delta)
    // widens it.
    const onMove = (ev: MouseEvent) =>
      setWidth(Math.min(900, Math.max(320, startW - (ev.clientX - startX))));
    const onUp = () => {
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
    };
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
  }

  const [precondLoading, setPrecondLoading] = useState(false);
  const [precondError, setPrecondError] = useState("");
  // Cross-project precondition picker open state (RND_P_4TFINT_05-322).
  const [showCrossPrecond, setShowCrossPrecond] = useState(false);
  const [swapKind, setSwapKind] = useState<"precondition" | "requirement" | null>(
    null,
  );
  const [customFields, setCustomFields] = useState<CustomFieldValue[]>([]);
  const [reviewer, setReviewer] = useState(
    () => localStorage.getItem(REVIEWER_KEY) ?? "",
  );
  const [reviewNote, setReviewNote] = useState("");
  const [transitions, setTransitions] = useState<Transition[]>([]);
  // The settable-status dropdown's option list (P6.2b, Kiwi statusModel
  // "settable"): every valid status, same source TestTable's workflow-status
  // filter uses. Xray never reads this — it uses `transitions` instead.
  const [allStatuses, setAllStatuses] = useState<string[]>([]);
  const [steps, setSteps] = useState<Step[]>([]);
  const [stepsLoading, setStepsLoading] = useState(false);
  const [stepsError, setStepsError] = useState("");
  // Custom Fields section is collapsed by default: it holds many rarely-needed
  // Xray fields that clutter the test-case review (RND_P_4TFINT_05-321). A
  // triangle toggle expands it on demand.
  const [customFieldsOpen, setCustomFieldsOpen] = useState(false);
  const [showCloneSteps, setShowCloneSteps] = useState(false);
  const [showCallPicker, setShowCallPicker] = useState(false);
  // Cross-project link affordances (RND_P_4TFINT_05-322): the profile's
  // configured source projects gate the "Other project" buttons; separate
  // modal flags open the pickers in cross-project-only mode.
  const [crossProjectSources, setCrossProjectSources] = useState("");
  const crossProjectSourceList = crossProjectSources
    .split(/[\s,;]+/)
    .map((s) => s.trim())
    .filter(Boolean);
  const crossProjectEnabled = crossProjectSourceList.length > 0;
  const [showCrossCall, setShowCrossCall] = useState(false);
  const [showCrossClone, setShowCrossClone] = useState(false);
  const [cloning, setCloning] = useState(false);
  // What Jira itself reports about this Test's steps — used to warn when the
  // panel is empty but Jira actually has steps (a load/shape problem), so the
  // user doesn't add a blank step that Xray rejects.
  const [jiraStepInfo, setJiraStepInfo] = useState<JiraStepInfo | null>(null);
  // Sort state for the run history table. Default: updatedAt descending.
  const [runHistorySort, setRunHistorySort] = useState<{ field: string; desc: boolean }>(
    { field: "updatedAt", desc: true },
  );

  function toggleRunHistorySort(field: string) {
    setRunHistorySort((prev) =>
      prev.field === field
        ? { field, desc: !prev.desc }
        : { field, desc: true },
    );
  }

  function sortedRunHistory(runs: TestRunEntry[]): TestRunEntry[] {
    return [...runs].sort((a, b) => {
      let cmp = 0;
      switch (runHistorySort.field) {
        case "runStatus":
          cmp = (a.runStatus ?? "").localeCompare(b.runStatus ?? "");
          break;
        case "createdAt":
          cmp = (a.createdAt ?? "").localeCompare(b.createdAt ?? "");
          break;
        case "updatedAt":
          cmp = (a.updatedAt ?? "").localeCompare(b.updatedAt ?? "");
          break;
        default:
          cmp = 0;
      }
      return runHistorySort.desc ? -cmp : cmp;
    });
  }
  const [saveError, setSaveError] = useState("");

  // Local editable state — initialised from the loaded Test, then driven by
  // the user. Each blur compares against `test` (the last persisted value)
  // and saves only on a real change.
  const [summary, setSummary] = useState("");
  const [description, setDescription] = useState("");
  const [priority, setPriority] = useState("");
  const [labels, setLabels] = useState("");
  const [execType, setExecType] = useState("");
  const [cucumberScenario, setCucumberScenario] = useState("");
  const [cucumberType, setCucumberType] = useState("");
  const [genericDefinition, setGenericDefinition] = useState("");
  // Set when ChangeTestType reports canPrefill && !prefilled (destination body
  // already had content, so nothing was auto-filled). Cleared on dismiss.
  const [prefillNotice, setPrefillNotice] = useState<TypeConversion | null>(null);
  // Internal counter that forces the main load effect to re-run after a type
  // change so any pre-filled body is hydrated from the backend.
  const [localReloadKey, setLocalReloadKey] = useState(0);

  // The detail sections come from the query cache with STABLE keys (Phase 4a).
  // The parent's reload signal — `version` (bumped by App after commits/syncs/
  // conflict resolution) and `localReloadKey` (bumped locally on a type change)
  // — is translated into a targeted invalidation below rather than folded into
  // every key. `reloadSig` is that signal; the seed guard still keys off it so a
  // reload re-seeds the draft buffers.
  const reloadSig = `${version}:${localReloadKey}`;
  const metaQuery = useTestMeta(profileId, testKey);
  const meta = metaQuery.data ?? null;
  const runHistoryQuery = useTestRunHistory(profileId, testKey);
  const runHistory = runHistoryQuery.data ?? null;
  const runHistoryLoading = runHistoryQuery.isFetching;
  const runHistoryError = runHistoryQuery.error
    ? errMsg(runHistoryQuery.error)
    : "";

  // The `test` itself now lives in the query cache (audit A3, Phase 2b),
  // decoupled from the Promise.all below. Optimistic edits (field save, folder
  // move, status transition) patch it via queryClient.setQueryData on this key.
  const queryClient = useQueryClient();
  const testQuery = useTest(profileId, testKey);
  const test = testQuery.data ?? null;
  // The review verdict is its own query (Phase 2d), carrying an optimistic patch
  // in setVerdict. Its `reviewNote` draft is seeded once per load below.
  const reviewQuery = useTestReview(profileId, testKey);
  const review = reviewQuery.data ?? null;
  // This Test's container memberships (Phase 2e), with an optimistic patch in
  // deallocateContainer.
  const containersQuery = useTestContainers(profileId, testKey);
  const containers = containersQuery.data ?? [];
  // This Test's covered requirements (Phase 2f), with an optimistic patch in
  // applyRequirements.
  const requirementsQuery = useTestRequirements(profileId, testKey);
  const requirements = requirementsQuery.data ?? [];
  // This Test's linked preconditions (Phase 2g), with optimistic patches in
  // applyPreconditions / refreshPreconditions. The profile-wide pool the picker
  // draws from is a separate profile-scoped query (see allPreconditions below).
  const preconditionsQuery = useTestPreconditions(profileId, testKey);
  const preconditions = preconditionsQuery.data ?? [];
  const allPreconditions = useAllPreconditions(profileId).data ?? [];
  // The panel is loading until the test read and every interactive/decision-
  // relevant section resolve — the review verdict, container memberships,
  // covered requirements, and linked preconditions. Either finishing first must
  // not flash empty content, a stale "none" verdict, or a misleading "no links
  // yet". isPending is true only on first load (no data), so background
  // refetches don't re-hide the panel. (The supplementary picker pools — bugs,
  // requirement coverage, precondition pool — are intentionally not gated.)
  const panelLoading =
    testQuery.isPending ||
    reviewQuery.isPending ||
    containersQuery.isPending ||
    requirementsQuery.isPending ||
    preconditionsQuery.isPending;
  // A test-read failure surfaces through the panel's error banner so it never
  // goes silently blank on a failed load (the read is now its own query rather
  // than a Promise.all whose rejection used to land in local error state).
  const displayError = testQuery.error ? errMsg(testQuery.error) : "";

  // Read-only secondary sections come from the query cache too (Phase 2c),
  // decoupled from the load effect. `bugs` is test-scoped; the requirement
  // coverage list is profile-scoped and caches across test switches.
  const bugs = useTestBugs(profileId, testKey).data ?? [];
  const allRequirements = useRequirementCoverage(profileId).data ?? [];

  const isDirty = (field: string) =>
    pendingForTest.some((p) => p.field === field);

  return { profileId, activeProfile, prompt, confirm, width, setWidth, precondLoading, setPrecondLoading, precondError, setPrecondError, showCrossPrecond, setShowCrossPrecond, swapKind, setSwapKind, customFields, setCustomFields, reviewer, setReviewer, reviewNote, setReviewNote, transitions, setTransitions, allStatuses, setAllStatuses, steps, setSteps, stepsLoading, setStepsLoading, stepsError, setStepsError, customFieldsOpen, setCustomFieldsOpen, showCloneSteps, setShowCloneSteps, showCallPicker, setShowCallPicker, crossProjectSources, setCrossProjectSources, showCrossCall, setShowCrossCall, showCrossClone, setShowCrossClone, cloning, setCloning, jiraStepInfo, setJiraStepInfo, runHistorySort, setRunHistorySort, saveError, setSaveError, summary, setSummary, description, setDescription, priority, setPriority, labels, setLabels, execType, setExecType, cucumberScenario, setCucumberScenario, cucumberType, setCucumberType, genericDefinition, setGenericDefinition, prefillNotice, setPrefillNotice, localReloadKey, setLocalReloadKey, caps, datasetQuery, dataset, isDemoProfile, canLinkToJira, crossProjectSourceList, crossProjectEnabled, reloadSig, metaQuery, meta, runHistoryQuery, runHistory, runHistoryLoading, runHistoryError, queryClient, testQuery, test, reviewQuery, review, containersQuery, containers, requirementsQuery, requirements, preconditionsQuery, preconditions, allPreconditions, panelLoading, displayError, bugs, allRequirements, isDirty, openInJira, openBugInJira, startResize, toggleRunHistorySort, sortedRunHistory };
}

export type TestDetailState = ReturnType<typeof useTestDetailState>;
