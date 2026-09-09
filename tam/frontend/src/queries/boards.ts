import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { call } from "@agile-suite/core";
import {
  CanTransition,
  CompleteSprint,
  GetBoard,
  GetProfileSetting,
  JournalSprintMoves,
  ListBoardSprints,
  ListBoards,
  MoveIssueToColumn,
  MoveIssueToSprint,
  RankIssue,
  SETTING_BOARDS_UNAVAILABLE,
  StartSprint,
  SuggestSprintDates,
  SyncBoards,
} from "../api";
import type { BoardSummary, SprintCompletion } from "../api";
import { keys } from "./keys";
import { invalidateWrites } from "./invalidate";

// useBoards lists the profile's cached boards for the board picker.
export function useBoards(profileId: string) {
  return useQuery({
    queryKey: keys.boards(profileId),
    queryFn: () => call(() => ListBoards(profileId)),
    enabled: !!profileId,
  });
}

// useBoardSprints lists one board's cached sprints, active first. Only a
// scrum board has any, so the caller passes 0 for a kanban one.
export function useBoardSprints(profileId: string, boardId: number) {
  return useQuery({
    queryKey: keys.boardSprints(profileId, boardId),
    queryFn: () => call(() => ListBoardSprints(profileId, boardId)),
    enabled: !!profileId && boardId > 0,
  });
}

// useBoard composes the view itself. placeholderData keeps the drawn board
// on screen while a swimlane or sprint change refetches, the same way the
// tree keeps its rows, but falls back to the pending state on a profile
// switch so one profile's cards are never shown under another's name.
// ready gates a scrum board on its sprint list: until useBoardSprints
// settles, sprintId is not the real one yet, and firing the query anyway is
// a wasted round trip with a real risk of the whole board flashing before
// the sprint-scoped one replaces it.
export function useBoard(profileId: string, boardId: number, sprintId: string, swimlane: string, ready: boolean) {
  return useQuery({
    queryKey: keys.board(profileId, boardId, sprintId, swimlane),
    queryFn: () => call(() => GetBoard(profileId, boardId, sprintId, swimlane)),
    enabled: !!profileId && boardId > 0 && ready,
    placeholderData: (prev, prevQuery) => (prevQuery?.queryKey[0] === profileId ? prev : undefined),
  });
}

// useBoardsUnavailable reads the profile setting the boards sync writes when
// the instance answered with no Agile API. It is what lets the view tell
// "this Jira has no boards" from "nothing has been synced yet" on a cold
// start, before it has run a sync of its own.
export function useBoardsUnavailable(profileId: string) {
  return useQuery({
    queryKey: keys.boardsUnavailable(profileId),
    queryFn: () => call(() => GetProfileSetting(profileId, SETTING_BOARDS_UNAVAILABLE)),
    enabled: !!profileId,
    select: (v: string) => v === "true",
  });
}

// useSyncBoards is the Refresh button: it pulls the boards, their sprints,
// and their cards, then refreshes everything that read them, the sync state
// included, since the pass writes a fresh timestamp with the rest.
//
// run is SyncContext's runBoardsRefresh, injected rather than reached for so
// this module stays free of the context. It is what holds the sync lock for
// the duration: Go holds one per-profile lock for a refresh and a sync alike,
// so a refresh that did not take the frontend's lock left the shell offering
// a Sync the backend would refuse.
export function useSyncBoards(profileId: string, run: () => Promise<BoardSummary>) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: run,
    onSettled: () => {
      if (!profileId) return;
      for (const queryKey of [
        keys.boards(profileId),
        [profileId, "boardSprints"] as const,
        [profileId, "board"] as const,
        keys.boardsUnavailable(profileId),
        keys.syncState(profileId),
      ]) {
        qc.invalidateQueries({ queryKey });
      }
    },
  });
}

// The three board writes. Each one journals the move and moves the card in
// the local cache, so the board repaints where it was dropped; nothing here
// reaches Jira, which Commit does. All three refresh what a local write can
// change, the board included, through the one invalidation every write
// uses.
export function useMoveToColumn(profileId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ key, statusId }: { key: string; statusId: string }) =>
      call(() => MoveIssueToColumn(profileId, key, statusId)),
    onSuccess: (_, v) => invalidateWrites(qc, profileId, v.key),
  });
}

export function useRankIssue(profileId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ key, neighbourKey, before, boardId }: { key: string; neighbourKey: string; before: boolean; boardId: number }) =>
      call(() => RankIssue(profileId, key, neighbourKey, before, boardId)),
    onSuccess: (_, v) => invalidateWrites(qc, profileId, v.key),
  });
}

export function useMoveToSprint(profileId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ key, sprintId }: { key: string; sprintId: string }) =>
      call(() => MoveIssueToSprint(profileId, key, sprintId)),
    onSuccess: (_, v) => invalidateWrites(qc, profileId, v.key),
  });
}

// useJournalSprintMoves is the bulk move behind the selection bar: one
// journal row per card, written in one transaction, with nothing reaching
// Jira until Commit. It refreshes exactly what the single move refreshes,
// so the cards it touched wear the same pending marks a dragged one does.
export function useJournalSprintMoves(profileId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ keys, sprintId }: { keys: string[]; sprintId: string }) =>
      call(() => JournalSprintMoves(profileId, keys, sprintId)),
    onSuccess: () => invalidateWrites(qc, profileId),
  });
}

// useSprintSuggestion is what the start dialog opens with: the name the
// board's numbering implies and the dates its own history suggests. It reads
// the cache only, so it answers whether or not Jira can be reached, and it
// is not retried: a suggestion is a convenience, and the dialog is usable
// without one.
export function useSprintSuggestion(profileId: string, boardId: number, enabled: boolean) {
  return useQuery({
    queryKey: keys.sprintSuggestion(profileId, boardId),
    queryFn: () => call(() => SuggestSprintDates(profileId, boardId)),
    enabled: enabled && !!profileId && boardId > 0,
    retry: false,
    // A suggestion built from today's date must not be served from an hour
    // ago on a machine left open overnight.
    gcTime: 0,
    staleTime: 0,
  });
}

// The two ceremonies. Both push to Jira the moment they are called, so both
// go through run, which is SyncContext's runSprintCeremony: it holds the
// same per-profile lock a sync and a commit hold, injected rather than
// reached for so this module stays free of the context.
//
// Both refresh the board's sprint list, which the service itself re-read
// into the cache, and the board under it, whose cards have moved.
function invalidateSprints(qc: ReturnType<typeof useQueryClient>, profileId: string) {
  if (!profileId) return;
  for (const queryKey of [
    keys.boards(profileId),
    [profileId, "boardSprints"] as const,
    [profileId, "board"] as const,
    [profileId, "sprintSuggestion"] as const,
  ]) {
    qc.invalidateQueries({ queryKey });
  }
}

export interface StartSprintArgs {
  boardId: number;
  sprintId: number;
  name: string;
  goal: string;
  start: string;
  end: string;
}

export function useStartSprint(profileId: string, run: <T>(action: () => Promise<T>) => Promise<T>) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: StartSprintArgs) =>
      run(() => call(() => StartSprint(profileId, v.boardId, v.sprintId, v.name, v.goal, v.start, v.end))),
    onSettled: () => invalidateSprints(qc, profileId),
  });
}

export interface CompleteSprintArgs {
  boardId: number;
  sprintId: number;
  // moveTo is the destination sprint's id, empty for the backlog, which is a
  // destination and not an absence.
  moveTo: string;
}

// A completion refreshes on settle rather than on success, because a
// completion that failed partway has still moved cards: the board it leaves
// behind is not the board it started from.
export function useCompleteSprint(profileId: string, run: <T>(action: () => Promise<T>) => Promise<T>) {
  const qc = useQueryClient();
  return useMutation<SprintCompletion, Error, CompleteSprintArgs>({
    mutationFn: (v) => run(() => call(() => CompleteSprint(profileId, v.boardId, v.sprintId, v.moveTo))),
    onSettled: () => {
      invalidateSprints(qc, profileId);
      invalidateWrites(qc, profileId);
    },
  });
}

// useCanTransition is the one board call that reads Jira: it asks whether
// a card can reach the column it was just dropped in. It is a mutation
// rather than a query because it is asked once per drop, about a card and
// a target that will not be asked about again, and because it must not be
// retried or cached: an answer is only true of the moment it was given.
export function useCanTransition(profileId: string) {
  return useMutation({
    mutationFn: ({ key, statusId }: { key: string; statusId: string }) =>
      call(() => CanTransition(profileId, key, statusId)),
    retry: false,
  });
}
