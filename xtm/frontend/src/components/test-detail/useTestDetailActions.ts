import type { TestDetailProps } from "./types";
import type { TestDetailState } from "./useTestDetailState";
import { useFieldActions } from "./useFieldActions";
import { useStepActions } from "./useStepActions";
import { useLinkActions } from "./useLinkActions";

// useTestDetailActions gathers the three action hooks so the sections take
// one object.
export function useTestDetailActions(s: TestDetailState, p: TestDetailProps) {
  return { ...useFieldActions(s, p), ...useStepActions(s, p), ...useLinkActions(s, p) };
}

export type TestDetailActions = ReturnType<typeof useTestDetailActions>;
