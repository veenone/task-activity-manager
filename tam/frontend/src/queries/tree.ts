import { useQuery } from "@tanstack/react-query";
import { call } from "@agile-suite/core";
import { GetEpicTree, ListEpics } from "../api";
import type { TreeQuery } from "../api";
import { keys } from "./keys";

// useEpicTree loads the Epics view's tree. placeholderData keeps the
// previous tree on screen while a filter change refetches, the same way
// useIssues keeps the Backlog's page, but falls back to the pending state on
// a profile switch so one profile's tree is never shown under another's name.
export function useEpicTree(profileId: string, q: TreeQuery) {
  return useQuery({
    queryKey: keys.tree(profileId, q),
    queryFn: () => call(() => GetEpicTree(profileId, q)),
    enabled: !!profileId,
    placeholderData: (prev, prevQuery) => (prevQuery?.queryKey[0] === profileId ? prev : undefined),
  });
}

export function useEpics(profileId: string) {
  return useQuery({
    queryKey: keys.epics(profileId),
    queryFn: () => call(() => ListEpics(profileId)),
    enabled: !!profileId,
  });
}
