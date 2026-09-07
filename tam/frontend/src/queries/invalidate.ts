import type { QueryClient } from "@tanstack/react-query";
import { keys } from "./keys";

// invalidateProfileData refreshes everything a sync can change for one
// profile: every issues page, the sprint list, the sync state, and the
// boards, their sprint lists, and the composed board view, which a sync
// refreshes as surely as it does the grid. Issue details are left alone;
// the backend's own cache decides their freshness.
export function invalidateProfileData(qc: QueryClient, profileId: string) {
  if (!profileId) return;
  for (const queryKey of [
    [profileId, "issues"] as const,
    keys.sprints(profileId),
    keys.syncState(profileId),
    keys.pending(profileId),
    [profileId, "tree"] as const,
    keys.epics(profileId),
    keys.boards(profileId),
    [profileId, "boardSprints"] as const,
    [profileId, "board"] as const,
    keys.boardsUnavailable(profileId),
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
  qc.invalidateQueries({ queryKey: [profileId, "tree"] });
  qc.invalidateQueries({ queryKey: keys.epics(profileId) });
  // A pending edit shows on the card too, so the board repaints with it.
  qc.invalidateQueries({ queryKey: [profileId, "board"] });
}
