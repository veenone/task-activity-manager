import type { QueryClient } from "@tanstack/react-query";
import { keys } from "./keys";

// invalidateProfileData refreshes everything a sync can change for one
// profile: every issues page, the sprint list, and the sync state. Issue
// details are left alone; the backend's own cache decides their freshness.
export function invalidateProfileData(qc: QueryClient, profileId: string) {
  if (!profileId) return;
  for (const queryKey of [
    [profileId, "issues"] as const,
    keys.sprints(profileId),
    keys.syncState(profileId),
  ]) {
    qc.invalidateQueries({ queryKey });
  }
}
