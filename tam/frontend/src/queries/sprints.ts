import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { QueryClient } from "@tanstack/react-query";
import { call } from "@agile-suite/core";
import { DeleteSprint, EditSprint, ListBoardSprintDetails } from "../api";
import { keys } from "./keys";

// The Sprints view's read, and the two writes only it makes. Creating a
// sprint is the third of that set and stays in queries/boards.ts, where the
// Boards toolbar already reached it; all five sprint writes refresh the same
// list, through invalidateSprintWrites below.

// useBoardSprintDetails is one board's sprints with the cards in each. It is
// not gated on the sprint list the pickers read: this call composes the
// sprints itself, so waiting for a second list would only delay it.
//
// There is no placeholderData here, unlike the composed board. A board
// switch changes every row, so keeping the last board's sprints on screen
// would show one board's work under another board's name for as long as the
// read takes, and the view has a loading branch that says what it is doing.
export function useBoardSprintDetails(profileId: string, boardId: number) {
  return useQuery({
    queryKey: keys.boardSprintDetails(profileId, boardId),
    queryFn: () => call(() => ListBoardSprintDetails(profileId, boardId)),
    enabled: !!profileId && boardId > 0,
  });
}

// invalidateSprintWrites refreshes everything a sprint write can change, and
// every sprint write uses it: the two ceremonies, and the create, edit and
// delete beside them.
//
// The board's sprint rows and this view's composed list are the obvious two.
// The composed board goes with them because a card can leave a sprint on a
// completion, the suggestion because it is measured from the board's own
// closed sprints, and the profile-wide open list because a create adds a
// sprint to it, a completion closes one out of it, and an edit renames one
// inside it: the Backlog's Sprint field offers that list and has no other
// way to hear about any of the three.
export function invalidateSprintWrites(qc: QueryClient, profileId: string) {
  if (!profileId) return;
  for (const queryKey of [
    keys.boards(profileId),
    [profileId, "boardSprints"] as const,
    [profileId, "boardSprintDetails"] as const,
    [profileId, "board"] as const,
    [profileId, "sprintSuggestion"] as const,
    keys.openSprints(profileId),
  ]) {
    qc.invalidateQueries({ queryKey });
  }
}

export interface EditSprintArgs {
  boardId: number;
  sprintId: number;
  name: string;
  goal: string;
  start: string;
  end: string;
  // clearGoal is what tells an empty goal box left alone from one asking to
  // remove a goal that was there. The draft's blank Goal cannot say which,
  // so the dialog decides it from the goal the sprint arrived with.
  clearGoal: boolean;
}

// useEditSprint renames a sprint, rewrites its goal, or moves its dates. It
// reaches Jira the moment it is called, like the ceremonies, but it is not
// one: a rename is nothing the other views need announced, so run here is
// SyncContext's runQuietLock rather than runSprintCeremony, injected the
// same way and holding Go's per-profile lock just as tightly.
//
// That lock is also why the dialog reports its own failures in words.
// Suppressing the sync banner suppresses the banner and nothing else: the
// refusal comes from acquire in Go, so an edit during a boards refresh is
// refused whether or not anything on screen is showing a banner, and the
// dialog is then the only place that refusal can be read.
export function useEditSprint(profileId: string, run: <T>(action: () => Promise<T>) => Promise<T>) {
  const qc = useQueryClient();
  return useMutation<string, Error, EditSprintArgs>({
    mutationFn: (v) =>
      run(() => call(() => EditSprint(profileId, v.boardId, v.sprintId, v.name, v.goal, v.start, v.end, v.clearGoal))),
    onSettled: () => invalidateSprintWrites(qc, profileId),
  });
}

export interface DeleteSprintArgs {
  boardId: number;
  sprintId: number;
}

// useDeleteSprint destroys the sprint in Jira and then TAM's copies of it.
// It settles rather than succeeds into the invalidation for the same reason
// a completion does: the sprint can be gone from Jira while the cache
// bookkeeping after it fails, and the list left behind is not the list it
// started from.
export function useDeleteSprint(profileId: string, run: <T>(action: () => Promise<T>) => Promise<T>) {
  const qc = useQueryClient();
  return useMutation<string, Error, DeleteSprintArgs>({
    mutationFn: (v) => run(() => call(() => DeleteSprint(profileId, v.boardId, v.sprintId))),
    onSettled: () => invalidateSprintWrites(qc, profileId),
  });
}
