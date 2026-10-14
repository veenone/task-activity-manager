import { AddCalledTestStep, CloneTestSteps } from "../../api";
import { CloneStepsModal } from "../CloneStepsModal";
import { PickTestModal } from "../PickTestModal";
import { PickPreconditionModal } from "../PickPreconditionModal";
import { SwapModal } from "./SwapModal";
import type { TestDetailState } from "./useTestDetailState";
import type { TestDetailActions } from "./useTestDetailActions";
import type { TestDetailProps } from "./types";

// DetailModals are the pickers the panel opens: call a test, clone steps,
// link a precondition from another project, and the two swap dialogs.
export function DetailModals({ s, a, p }: { s: TestDetailState; a: TestDetailActions; p: TestDetailProps }) {
  const { profileId, showCrossPrecond, setShowCrossPrecond, swapKind, setSwapKind, setSteps, showCloneSteps, setShowCloneSteps, showCallPicker, setShowCallPicker, showCrossCall, setShowCrossCall, showCrossClone, setShowCrossClone, crossProjectSourceList, requirements, preconditions, allPreconditions, allRequirements } = s;
  const { applyRequirements, applyPreconditions, addCrossProjectPreconditions } = a;
  const { testKey, onEdited, readOnly } = p;
  return (
    <>
      {!readOnly && showCallPicker && (
        <PickTestModal
          heading={`Call a test from ${testKey}`}
          excludeKey={testKey}
          onCancel={() => setShowCallPicker(false)}
          onPick={async (calledKey) => {
            const s = await AddCalledTestStep(profileId, testKey, calledKey);
            setSteps((prev) => [...prev, s]);
            setShowCallPicker(false);
            onEdited();
          }}
        />
      )}

      {!readOnly && showCrossCall && (
        <PickTestModal
          heading={`Call a test from another project`}
          excludeKey={testKey}
          crossProjectOnly
          sourceProjects={crossProjectSourceList}
          onCancel={() => setShowCrossCall(false)}
          onPick={async (calledKey) => {
            const s = await AddCalledTestStep(profileId, testKey, calledKey);
            setSteps((prev) => [...prev, s]);
            setShowCrossCall(false);
            onEdited();
          }}
        />
      )}

      {!readOnly && showCrossClone && (
        <CloneStepsModal
          targetLabel={testKey}
          excludeKey={testKey}
          crossProjectOnly
          sourceProjects={crossProjectSourceList}
          onCancel={() => setShowCrossClone(false)}
          onConfirm={async (sourceKey, stepIds) => {
            const newSteps = await CloneTestSteps(
              profileId,
              testKey,
              sourceKey,
              stepIds,
            );
            setSteps(newSteps ?? []);
            setShowCrossClone(false);
            onEdited();
          }}
        />
      )}

      {!readOnly && showCrossPrecond && (
        <PickPreconditionModal
          excludeKeys={preconditions.map((p) => p.key)}
          sourceProjects={crossProjectSourceList}
          onCancel={() => setShowCrossPrecond(false)}
          onPick={addCrossProjectPreconditions}
        />
      )}

      {!readOnly && showCloneSteps && (
        <CloneStepsModal
          targetLabel={testKey}
          excludeKey={testKey}
          onCancel={() => setShowCloneSteps(false)}
          onConfirm={async (sourceKey, stepIds) => {
            const newSteps = await CloneTestSteps(
              profileId,
              testKey,
              sourceKey,
              stepIds,
            );
            setSteps(newSteps ?? []);
            setShowCloneSteps(false);
            onEdited();
          }}
        />
      )}
      {!readOnly && swapKind === "precondition" && (
        <SwapModal
          title="Swap preconditions"
          current={preconditions.map((p) => ({
            key: p.key,
            label: `${p.key} — ${p.summary}`,
          }))}
          candidates={allPreconditions
            .filter((p) => !preconditions.some((lp) => lp.key === p.key))
            .map((p) => ({ key: p.key, label: `${p.key} — ${p.summary}` }))}
          onCancel={() => setSwapKind(null)}
          onConfirm={async (next) => {
            await applyPreconditions(next);
            setSwapKind(null);
          }}
        />
      )}
      {!readOnly && swapKind === "requirement" && (
        <SwapModal
          title="Swap requirements"
          current={requirements.map((rq) => ({
            key: rq.key,
            label: `${rq.key} — ${rq.summary}`,
          }))}
          candidates={allRequirements
            .filter((r) => !requirements.some((lr) => lr.key === r.key))
            .map((r) => ({ key: r.key, label: `${r.key} — ${r.summary}` }))}
          onCancel={() => setSwapKind(null)}
          onConfirm={async (next) => {
            await applyRequirements(next);
            setSwapKind(null);
          }}
        />
      )}
    </>
  );
}
