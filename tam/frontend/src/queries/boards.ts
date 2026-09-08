import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { call } from "@agile-suite/core";
import {
  CanTransition,
  GetBoard,
  GetProfileSetting,
  ListBoardSprints,
  ListBoards,
  MoveIssueToColumn,
  MoveIssueToSprint,
  RankIssue,
  SETTING_BOARDS_UNAVAILABLE,
  SyncBoards,
} from "../api";
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
export function useSyncBoards(profileId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => call(() => SyncBoards(profileId)),
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
