import { GetTestPreconditions, GetTestRequirements, SetTestRequirements, ListAllPreconditions, SetTestPreconditions, CreatePrecondition, CacheExternalPreconditions, errMsg } from "../../api";
import type { Precondition } from "../../api";
import { keys } from "../../queries/keys";

import type { TestDetailProps } from "./types";
import type { TestDetailState } from "./useTestDetailState";

// useLinkActions links and unlinks requirements and preconditions, including
// preconditions from another project and brand-new ones.
export function useLinkActions(s: TestDetailState, p: TestDetailProps) {
  const { profileId, prompt, confirm, setPrecondLoading, setPrecondError, setShowCrossPrecond, setSaveError, queryClient, requirements, preconditions } = s;
  const { testKey, onEdited, readOnly } = p;

  // applyRequirements replaces the test's covered-requirement set and refreshes
  // the displayed list. The link changes commit as Jira issue links.
  async function applyRequirements(nextKeys: string[]) {
    if (readOnly) return;
    setSaveError("");
    try {
      await SetTestRequirements(profileId, testKey, nextKeys);
      const refreshed = await GetTestRequirements(profileId, testKey);
      queryClient.setQueryData(
        keys.testRequirements(profileId, testKey),
        refreshed ?? [],
      );
      onEdited();
    } catch (e) {
      setSaveError(`Requirement update failed: ${errMsg(e)}`);
    }
  }

  // addRequirements links every picked requirement in a single apply: the union
  // of the already-linked keys and the newly ticked ones is sent to
  // applyRequirements once, so it coalesces into one pending change
  // (RND_P_4TFINT_05-224).
  function addRequirements(keys: string[]) {
    if (readOnly) return;
    const linked = new Set(requirements.map((r) => r.key));
    const additions = keys.filter((k) => k && !linked.has(k));
    if (additions.length === 0) return;
    applyRequirements([...linked, ...additions]);
  }

  async function removeRequirement(key: string) {
    if (readOnly) return;
    if (
      !(await confirm({
        title: "Unlink requirement",
        message: `Unlink ${key} from ${testKey}? This doesn't delete the requirement, just removes the coverage link when you commit.`,
        confirmLabel: "Unlink",
      }))
    )
      return;
    applyRequirements(requirements.map((r) => r.key).filter((k) => k !== key));
  }

  // refreshPreconditions re-reads the links from Xray, bypassing the cache.
  // The sync is the only other thing that fills them in, so this is the way out
  // when a sync dropped them (RND_P_4TFINT_05-339).
  async function refreshPreconditions() {
    setPrecondLoading(true);
    setPrecondError("");
    try {
      const pre = await GetTestPreconditions(profileId, testKey, true);
      queryClient.setQueryData(
        keys.testPreconditions(profileId, testKey),
        pre ?? [],
      );
      const all = await ListAllPreconditions(profileId);
      queryClient.setQueryData(
        keys.preconditionPool(profileId),
        all ?? [],
      );
    } catch (e) {
      setPrecondError(errMsg(e));
    } finally {
      setPrecondLoading(false);
    }
  }

  // applyPreconditions replaces the test's precondition set, then refreshes the
  // displayed list from the store (FR-13.5). Add/remove both route here.
  async function applyPreconditions(nextKeys: string[]) {
    if (readOnly) return;
    setSaveError("");
    try {
      await SetTestPreconditions(profileId, testKey, nextKeys);
      const refreshed = await GetTestPreconditions(profileId, testKey, false);
      queryClient.setQueryData(
        keys.testPreconditions(profileId, testKey),
        refreshed ?? [],
      );
      onEdited();
    } catch (e) {
      setSaveError(`Precondition update failed: ${errMsg(e)}`);
    }
  }

  async function removePrecondition(key: string) {
    if (readOnly) return;
    if (
      !(await confirm({
        title: "Unlink precondition",
        message: `Unlink ${key} from ${testKey}? This doesn't delete the precondition, just removes the link when you commit.`,
        confirmLabel: "Unlink",
      }))
    )
      return;
    applyPreconditions(
      preconditions.map((p) => p.key).filter((k) => k !== key),
    );
  }

  // addPreconditions links every picked precondition in a single apply: the
  // union of the already-linked keys and the newly ticked ones is sent to
  // applyPreconditions once, so it coalesces into one pending change
  // (RND_P_4TFINT_05-224).
  function addPreconditions(keys: string[]) {
    if (readOnly) return;
    const linked = new Set(preconditions.map((p) => p.key));
    const additions = keys.filter((k) => k && !linked.has(k));
    if (additions.length === 0) return;
    applyPreconditions([...linked, ...additions]);
  }

  // addCrossProjectPreconditions links one or more preconditions from another
  // project (RND_P_4TFINT_05-322). It first caches the foreign preconditions
  // locally so they display with their summaries, then links their keys.
  async function addCrossProjectPreconditions(pcs: Precondition[]) {
    if (readOnly) return;
    setShowCrossPrecond(false);
    const linked = new Set(preconditions.map((p) => p.key));
    const additions = pcs.filter((p) => p.key && !linked.has(p.key));
    if (additions.length === 0) return;
    setSaveError("");
    try {
      await CacheExternalPreconditions(profileId, additions);
      const all = await ListAllPreconditions(profileId);
      queryClient.setQueryData(
        keys.preconditionPool(profileId),
        all ?? [],
      );
      await applyPreconditions([
        ...preconditions.map((p) => p.key),
        ...additions.map((p) => p.key),
      ]);
    } catch (e) {
      setSaveError(`Link precondition failed: ${errMsg(e)}`);
    }
  }

  // createAndAssociatePrecondition creates a brand-new Precondition (FR-13.5)
  // and links it to this test. It gets a temporary key until commit creates
  // the issue in Jira.
  async function createAndAssociatePrecondition() {
    if (readOnly) return;
    const summary = await prompt({
      title: "New precondition",
      placeholder: "Precondition summary",
      submitLabel: "Create",
    });
    if (!summary || !summary.trim()) return;
    setSaveError("");
    try {
      const tempKey = await CreatePrecondition(profileId, summary.trim());
      const all = await ListAllPreconditions(profileId);
      queryClient.setQueryData(
        keys.preconditionPool(profileId),
        all ?? [],
      );
      await applyPreconditions([...preconditions.map((p) => p.key), tempKey]);
    } catch (e) {
      setSaveError(`Create precondition failed: ${errMsg(e)}`);
    }
  }

  return { applyRequirements, addRequirements, removeRequirement, refreshPreconditions, applyPreconditions, removePrecondition, addPreconditions, addCrossProjectPreconditions, createAndAssociatePrecondition };
}
