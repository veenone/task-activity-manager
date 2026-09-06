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
    keys.pending(profileId),
  ]) {
    qc.invalidateQueries({ queryKey });
  }
}

// invalidateWrites refreshes what a local write can change: the Backlog
// rows, the pending list, and one issue's detail, tests, and activity when
// a key is given, or every issue's when it is not (a discard-all).
export function invalidateWrites(qc: QueryClient, profileId: string, key?: string) {
  if (!profileId) return;
  qc.invalidateQueries({ queryKey: [profileId, "issues"] });
  qc.invalidateQueries({ queryKey: keys.pending(profileId) });
  qc.invalidateQueries({ queryKey: key ? [profileId, "issue", key] : [profileId, "issue"] });
}
