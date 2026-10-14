import { useEffect, useLayoutEffect, useRef } from "react";
import { GetTestTransitions, ListStatuses, GetTestSteps, CheckJiraTestSteps, GetTestCustomFields, EditTestField, errMsg } from "../../api";
import { keys } from "../../queries/keys";
import type { TestDetailProps } from "./types";
import type { TestDetailState } from "./useTestDetailState";

// useTestDetailLoad runs the panel's load effects: it turns the parent's
// reload signal into a query invalidation, loads steps, custom fields and the
// status options, and seeds the draft buffers once per load.
export function useTestDetailLoad(s: TestDetailState, p: TestDetailProps) {
  const { profileId, setCustomFields, setReviewNote, setTransitions, setAllStatuses, setSteps, setStepsLoading, setStepsError, setJiraStepInfo, setSaveError, setSummary, setDescription, setPriority, setLabels, setExecType, setCucumberScenario, setCucumberType, setGenericDefinition, setPrefillNotice, localReloadKey, caps, reloadSig, queryClient, testQuery, reviewQuery } = s;
  const { testKey, version, readOnly } = p;

  // Guards the one-time seed of the editable draft buffers (summary, labels, …)
  // from a freshly-loaded test. Keyed on testKey:reloadSig so a new test or a
  // reload re-seeds, but an optimistic setQueryData (same key) does NOT — which
  // is what keeps a folder/status change from wiping the user's unsaved drafts.
  const seededRef = useRef<string>("");
  const reviewSeededRef = useRef<string>("");
  const seedKey = `${testKey}:${reloadSig}`;

  // Tracks the previously-shown key so we can detect a just-committed new Test
  // (its key flips from a "NEW-N" placeholder to the real Jira key) and force a
  // fresh pull from Jira — the local cache still holds temporary step IDs (FR-1).
  const prevKeyRef = useRef<string>("");

  // Translate the parent's reload signal into a targeted invalidation (Phase
  // 4a). A reloadSig bump on the SAME test (App bumped detailVersion after a
  // commit/sync/conflict, or a type change bumped localReloadKey) refetches this
  // test's sections plus the two profile-scoped pools. A test SWITCH is skipped
  // — the new test's queries mount fresh, so invalidating would double-fetch.
  // This is a LAYOUT effect declared BEFORE the seed layout effects below on
  // purpose: within the commit that bumps reloadSig, the invalidation must run
  // first so the queries are already fetchStatus === "fetching" when the seeds
  // run — the seeds read that live fetch state to wait for the fresh data.
  const reloadTestRef = useRef<string>("");
  const reloadSigRef = useRef<string>("");
  useLayoutEffect(() => {
    const testId = `${profileId} ${testKey}`;
    const sameTest = reloadTestRef.current === testId;
    const sigChanged = reloadSigRef.current !== reloadSig;
    reloadTestRef.current = testId;
    reloadSigRef.current = reloadSig;
    if (sameTest && sigChanged) {
      queryClient.invalidateQueries({ queryKey: keys.test(profileId, testKey) });
      queryClient.invalidateQueries({
        queryKey: keys.requirementCoverage(profileId),
      });
      queryClient.invalidateQueries({
        queryKey: keys.preconditionPool(profileId),
      });
    }
  }, [profileId, testKey, reloadSig, queryClient]);

  useEffect(() => {
    let cancelled = false;
    setSaveError("");
    setPrefillNotice(null);
    const justCommitted =
      prevKeyRef.current.startsWith("NEW-") && !testKey.startsWith("NEW-");
    prevKeyRef.current = testKey;
    // The test and every panel section now load via their own queries
    // (Phase 2b–2g) — the load Promise.all is gone. What remains here are the
    // two lazy per-detail reads with their own inline loading/error state:
    // steps and custom fields.
    setStepsLoading(true);
    setStepsError("");
    setJiraStepInfo(null);
    GetTestSteps(profileId, testKey, justCommitted)
      .then((s) => {
        if (cancelled) return;
        setSteps(s ?? []);
        // If nothing loaded, ask Jira whether this Test actually has steps,
        // so we can warn instead of letting the user add a blank one.
        if ((s ?? []).length === 0) {
          CheckJiraTestSteps(profileId, testKey)
            .then((info) => {
              if (!cancelled) setJiraStepInfo(info);
            })
            .catch((e) => console.error("check jira steps:", errMsg(e)));
        }
      })
      .catch((e) => {
        if (!cancelled) setStepsError(errMsg(e));
      })
      .finally(() => {
        if (!cancelled) setStepsLoading(false);
      });
    // Custom fields load lazily too — definitions come from sync, values on
    // first open. Failure is non-blocking.
    GetTestCustomFields(profileId, testKey, justCommitted)
      .then((cf) => {
        if (!cancelled) setCustomFields(cf ?? []);
      })
      .catch((e) => {
        if (!cancelled) console.error("custom fields:", errMsg(e));
      });
    return () => {
      cancelled = true;
    };
  }, [
    profileId,
    testKey,
    version,
    localReloadKey,
    // Setters from useState are stable; listed because they arrive through s.
    setCustomFields, setJiraStepInfo, setPrefillNotice, setSaveError, setSteps, setStepsError, setStepsLoading,
  ]);

  // Seed the editable draft buffers from a freshly-loaded test, exactly once per
  // load (see seededRef). This replaces the in-.then seeding that the useTest
  // migration removed. It runs when testQuery.data arrives; the seededRef guard
  // makes optimistic setQueryData patches (folder/status/edit) NOT re-seed, so
  // they never clobber the user's unsaved drafts.
  //
  // useLayoutEffect (not useEffect) so seeding runs before paint: the body
  // renders under `test && !panelLoading`, and if GetTest resolves after the
  // secondary Promise.all the drafts would otherwise flash empty for one frame.
  useLayoutEffect(() => {
    const t = testQuery.data;
    if (!t) return;
    if (seededRef.current === seedKey) return;
    // On a reload of the SAME test, the stable key means the stale copy lingers
    // while the translation effect's invalidation refetches. Wait for that
    // refetch to settle so we seed the FRESH data (e.g. a type-change prefill),
    // not the stale copy. We must read the query's LIVE fetch state from the
    // cache, not testQuery.isFetching (a render snapshot that is still false in
    // this layout-effect pass, before the invalidation's re-render lands). A
    // test SWITCH (different testKey) seeds immediately from cache — no wait.
    if (
      seededRef.current.startsWith(`${testKey}:`) &&
      queryClient.getQueryState(keys.test(profileId, testKey))?.fetchStatus ===
        "fetching"
    )
      return;
    seededRef.current = seedKey;
    let seeded = t;
    // Default cucumberType to "Scenario" for Cucumber tests with no stored type,
    // reflecting it in the cache (matches the old setTest patch) and persisting
    // it once when editable so a commit won't omit it.
    if (t.execType === "Cucumber" && (t.cucumberType ?? "") === "") {
      setCucumberType("Scenario");
      seeded = { ...t, cucumberType: "Scenario" };
      queryClient.setQueryData(keys.test(profileId, testKey), seeded);
      if (!readOnly) {
        EditTestField(profileId, testKey, "cucumber_type", "Scenario").catch(
          (e) => console.error("default cucumberType:", errMsg(e)),
        );
      }
    } else {
      setCucumberType(t.cucumberType ?? "");
    }
    setSummary(seeded.summary);
    setDescription(seeded.description);
    setPriority(seeded.priority);
    setLabels((seeded.labels ?? []).join(" "));
    setExecType(seeded.execType ?? "");
    setCucumberScenario(seeded.cucumberScenario ?? "");
    setGenericDefinition(seeded.genericDefinition ?? "");
  }, [
    testQuery.data,
    testQuery.isFetching,
    seedKey,
    profileId,
    testKey,
    reloadSig,
    readOnly,
    queryClient,
    setCucumberScenario, setCucumberType, setDescription, setExecType, setGenericDefinition, setLabels, setPriority, setSummary,
  ]);

  // Seed the review-note draft from the loaded review, once per load (its own
  // ref, same guard rationale as the test drafts): an optimistic setVerdict
  // patch keeps the same seedKey, so it never re-seeds and clobbers a note the
  // user is typing — setVerdict updates the note explicitly instead.
  useLayoutEffect(() => {
    if (!reviewQuery.isSuccess) return;
    if (reviewSeededRef.current === seedKey) return;
    // Same reload-vs-switch rule as the draft seed above (and the same live
    // fetch-state read, not the isFetching snapshot): on a reload of the same
    // test, wait for the invalidation refetch to settle before re-seeding.
    if (
      reviewSeededRef.current.startsWith(`${testKey}:`) &&
      queryClient.getQueryState(keys.testReview(profileId, testKey))
        ?.fetchStatus === "fetching"
    )
      return;
    reviewSeededRef.current = seedKey;
    setReviewNote(reviewQuery.data?.note ?? "");
  }, [
    reviewQuery.isSuccess,
    reviewQuery.isFetching,
    reviewQuery.data,
    seedKey,
    testKey,
    profileId,
    queryClient,
    setReviewNote,
  ]);

  // Status source (P6.2b): Xray (workflow) loads the transitions available
  // from the current status; Kiwi (settable) loads every valid status once
  // per profile, the same source TestTable's workflow-status filter uses.
  // Gated on caps.supportsWorkflowTransitions so a Kiwi profile never calls
  // GetTestTransitions (there are no transitions to fetch).
  useEffect(() => {
    let cancelled = false;
    if (caps.supportsWorkflowTransitions) {
      // Transitions load alongside the rest of the panel but can fail
      // without blocking it — workflow may not be set up yet, or the user
      // may not have edit permission.
      GetTestTransitions(profileId, testKey)
        .then((ts) => {
          if (!cancelled) setTransitions(ts ?? []);
        })
        .catch((e) => {
          if (!cancelled) console.error("transitions:", errMsg(e));
        });
    } else {
      ListStatuses(profileId)
        .then((s) => {
          if (!cancelled) setAllStatuses(s ?? []);
        })
        .catch((e) => {
          if (!cancelled) console.error("list statuses:", errMsg(e));
        });
    }
    return () => {
      cancelled = true;
    };
  }, [profileId, testKey, caps.supportsWorkflowTransitions, setAllStatuses, setTransitions]);
}
