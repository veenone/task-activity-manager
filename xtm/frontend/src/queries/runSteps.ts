import { useQuery } from "@tanstack/react-query";

import { GetRunStepResults, type RunStepResult } from "../api";
import { call } from "../lib/apiCall";
import { keys } from "./keys";

// The step results of one test's execution. Kept separate from
// useParameterizedRun because these do not belong to an iteration: a backend
// has one or the other, never both (#371 Phase 6).
export function useRunStepResults(profileId: string, execKey: string, testKey: string, enabled: boolean) {
  return useQuery<RunStepResult[]>({
    queryKey: keys.runStepResults(profileId, execKey, testKey),
    queryFn: () => call(() => GetRunStepResults(profileId, execKey, testKey, false)),
    enabled: !!profileId && !!execKey && !!testKey && enabled,
  });
}
