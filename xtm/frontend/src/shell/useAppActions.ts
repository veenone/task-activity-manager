import type { AppState } from "./useAppState";
import { useSyncActions } from "./useSyncActions";
import { useProfileActions } from "./useProfileActions";
import { useCommitActions } from "./useCommitActions";

// useAppActions is everything the shell does, gathered from the three action
// hooks so the layout pieces take one object.
export function useAppActions(s: AppState) {
  return { ...useSyncActions(s), ...useProfileActions(s), ...useCommitActions(s) };
}

export type AppActions = ReturnType<typeof useAppActions>;
