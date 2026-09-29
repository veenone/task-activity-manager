import { useQuery } from "@tanstack/react-query";
import { call } from "@agile-suite/core";
import { GetBoardCapacity } from "../api";
import { keys } from "./keys";

// useBoardCapacity is one board's column heads with no sprint behind them.
//
// It is the Reports view's read for a kanban board. A kanban board has no
// sprint, so useSprintReport cannot answer for one, and column capacity is
// the figure that needs none: Go reads the board's own issue list out of the
// synced cache and counts it against the limits the boards sync stored.
//
// Nothing here takes the per-profile lock, which is the difference that
// matters against useSprintReport. That read pages Jira for changelogs and
// runs for minutes, so it holds the lock and the shell says so. This one
// touches the local store only, so it neither waits for a sync nor blocks
// one, and it needs no cancel path.
//
// staleTime is zero. The count is about now rather than about a closed
// window, and every card a commit pushes moves it, so a reader returning to
// the view gets the count as it is rather than as it was.
export function useBoardCapacity(profileId: string, boardId: number) {
  return useQuery({
    queryKey: keys.boardCapacity(profileId, boardId),
    queryFn: () => call(() => GetBoardCapacity(profileId, boardId)),
    enabled: !!profileId && boardId > 0,
    staleTime: 0,
  });
}
