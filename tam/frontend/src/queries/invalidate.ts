import type { QueryClient } from "@tanstack/react-query";
import { keys } from "./keys";

// invalidateProfileData refreshes everything a sync can change for one
// profile: every issues page, the sprint list, the sync state, and the
// boards, their sprint lists, and the composed board view, which a sync
// refreshes as surely as it does the grid. openSprints is here too: a
// regular sync runs the boards pass as well as the issues pass, so it can
// bring sprints the profile-wide list has never seen, not only a Boards
// Refresh (which invalidates the same key itself, in queries/boards.ts).
// Issue details are left alone; the backend's own cache decides their
// freshness. The sprint report is here for the same reason the Boards
// view's own Refresh invalidates it: a regular sync runs the boards pass
// too, so it can move a status into or out of the board's last column,
// which is what a report counts as done.
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
    [profileId, "boardSprintDetails"] as const,
    keys.boardsUnavailable(profileId),
    keys.openSprints(profileId),
    [profileId, "sprintReport"] as const,
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
  // A pending edit shows on the card too, so the board repaints with it,
  // and so does the Sprints view: a journaled sprint move is replayed into
  // that read, which is how a bulk fill lands there before the next sync.
  qc.invalidateQueries({ queryKey: [profileId, "board"] });
  qc.invalidateQueries({ queryKey: [profileId, "boardSprintDetails"] });
}
