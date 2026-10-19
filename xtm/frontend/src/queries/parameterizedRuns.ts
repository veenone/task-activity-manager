import { useQuery } from "@tanstack/react-query";

import { GetParameterizedRun, type ParameterizedRun } from "../api";
import { call } from "../lib/apiCall";
import { keys } from "./keys";

export function useParameterizedRun(profileId: string, execKey: string, testKey: string, expanded: boolean) {
  return useQuery<ParameterizedRun>({
    queryKey: keys.parameterizedRun(profileId, execKey, testKey),
    queryFn: () => call(() => GetParameterizedRun(profileId, execKey, testKey, false)),
    enabled: !!profileId && !!execKey && !!testKey && expanded,
  });
}
