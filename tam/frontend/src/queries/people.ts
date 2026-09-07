import { useQuery } from "@tanstack/react-query";
import { call } from "@agile-suite/core";
import { GetSubtaskTypeName, ListPriorities, SearchUsers } from "../api";

// The instance's priority names change about as often as its workflow, and
// both forms gate a control on this, so it is held for the session rather
// than refetched every time a dialog opens.
const PRIORITIES_FRESH_FOR = 30 * 60 * 1000;
// A user search is answered from TAM's own cache when Jira is unreachable, so
// a repeat of the same query inside a minute is not worth a round trip.
const USERS_FRESH_FOR = 60 * 1000;

export const peopleKeys = {
  users: (profileId: string, query: string) => ["users", profileId, query] as const,
  priorities: (profileId: string) => ["priorities", profileId] as const,
  subtaskType: (profileId: string) => ["subtaskType", profileId] as const,
};

// useUserSearch backs the assignee picker. The query is already debounced by
// the caller; enabled is gated only on the profile, because the empty query
// is the picker's opening list rather than a reason not to ask.
export function useUserSearch(profileId: string, query: string) {
  return useQuery({
    queryKey: peopleKeys.users(profileId, query),
    queryFn: () => call(() => SearchUsers(profileId, query)),
    enabled: !!profileId,
    staleTime: USERS_FRESH_FOR,
    retry: false,
  });
}

export function usePriorities(profileId: string) {
  return useQuery({
    queryKey: peopleKeys.priorities(profileId),
    queryFn: () => call(() => ListPriorities(profileId)),
    enabled: !!profileId,
    staleTime: PRIORITIES_FRESH_FOR,
    retry: false,
  });
}

// useSubtaskType is the instance's own word for the sub-task level, held for
// the session: it comes from the project's configuration, which does not
// change while the app is open.
export function useSubtaskType(profileId: string) {
  return useQuery({
    queryKey: peopleKeys.subtaskType(profileId),
    queryFn: () => call(() => GetSubtaskTypeName(profileId)),
    enabled: !!profileId,
    staleTime: PRIORITIES_FRESH_FOR,
    retry: false,
  });
}
