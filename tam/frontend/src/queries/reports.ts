import { useCallback, useRef } from "react";
import { useQuery } from "@tanstack/react-query";
import { call } from "@agile-suite/core";
import { GetSprintReport } from "../api";
import { isBusyRefusal } from "../lib/reportText";
import { keys } from "./keys";

// The Reports view's one read. It is one call rather than two because Go's
// per-profile lock refuses rather than waits, so a view asking for the
// sprint and the velocity table separately would have had one of them
// refused every time it opened.

// RunLocked is SyncContext's runReport, taken rather than reached for the
// same way queries/sprints.ts takes runQuietLock. The hook has no business
// knowing which of the context's lock paths it is on, and a test can hand
// it a plain passthrough.
export type RunLocked = <T>(action: () => Promise<T>) => Promise<T>;

// RETRY_AFTER_CANCEL_MS is how long the one retry below waits.
//
// A board or sprint switch cancels the report that was running and starts
// the next one in the same commit, so the new read can reach Go while the
// cancelled one is still unwinding and still holds the lock. The refusal
// that comes back is real, and it is also about to stop being true, so one
// retry turns a dead end the user would have to press Retry on into a read
// that simply starts a moment later. A refusal from a sync or a commit
// survives the retry and reaches the view as the busy state, a quarter of a
// second later than it otherwise would.
const RETRY_AFTER_CANCEL_MS = 250;

// useSprintReport is one sprint's report, with rebuild beside it for a read
// that ignores the stored series.
//
// There is no placeholderData, for the reason useBoardSprintDetails gives
// about its own read: a board or sprint switch changes every figure on
// screen, and showing one sprint's numbers under another sprint's name for
// the length of a read is worse than a loading state that says what it is
// doing.
//
// staleTime is infinite for a closed sprint because this is the heaviest read
// TAM makes and its answer rarely moves. A live sprint is stale immediately,
// with no polling: reopening the view reads its changing report again.
//
// Two things do move it, and neither is Jira's history. The board's last
// column is what done means, so a boards refresh that changes it changes
// every completed figure; and a sprint id of 0 is a question, not an
// answer, so completing a sprint makes it resolve to a different sprint.
// Both invalidate this key (queries/boards.ts, queries/sprints.ts,
// queries/invalidate.ts), which is what an infinite staleTime leans on.
// rebuild is the way a reader asks for it again for any other reason.
export function useSprintReport(profileId: string, boardId: number, sprintId: number, run: RunLocked, live = false) {
  // The rebuild intent rides on a ref rather than on the query key, so a
  // rebuilt report replaces the stored one in the same cache entry instead
  // of becoming a second one the next plain read would flip back from. The
  // scope keeps an in-flight rebuild from leaking to a new profile, board or
  // sprint, and the operation object lets an older request's cleanup leave a
  // newer rebuild alone.
  const scope = JSON.stringify(keys.sprintReport(profileId, boardId, sprintId));
  const rebuilding = useRef<{ scope: string } | null>(null);
  const query = useQuery({
    queryKey: keys.sprintReport(profileId, boardId, sprintId),
    queryFn: () => {
      const refresh = rebuilding.current?.scope === scope;
      return run(() => call(() => GetSprintReport(profileId, boardId, sprintId, refresh)));
    },
    // A sprint id of 0 is a real request: it asks for the board's most
    // recent closed sprint. A negative or fractional one is not, and the
    // api guard would refuse it, so the read is never made.
    enabled: !!profileId && boardId > 0 && Number.isInteger(sprintId) && sprintId >= 0,
    staleTime: live ? 0 : Infinity,
    retry: (failures, error) => failures < 1 && isBusyRefusal(error.message),
    retryDelay: RETRY_AFTER_CANCEL_MS,
  });

  // rebuild asks Jira for the whole table again, the only thing short of a
  // bump to reports.AlgoVersion that replaces a stored series. refetch is
  // stable across renders, so the callback is too.
  const { refetch } = query;
  const rebuild = useCallback(() => {
    const operation = { scope };
    rebuilding.current = operation;
    return refetch().finally(() => {
      if (rebuilding.current === operation) rebuilding.current = null;
    });
  }, [refetch, scope]);

  return { report: query, rebuild };
}
