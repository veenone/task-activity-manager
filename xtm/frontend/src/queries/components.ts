import { useQuery } from "@tanstack/react-query";
import {
  ListComponents,
  ListProjectComponentDetails,
  ListProjectComponents,
  SearchUsers,
} from "../api";
import { call } from "../lib/apiCall";
import { keys } from "./keys";

// useProjectComponents loads the project's components live from Jira for the
// Components view.
export function useProjectComponents(profileId: string) {
  return useQuery({
    queryKey: keys.projectComponents(profileId),
    queryFn: () => call(() => ListProjectComponentDetails(profileId)),
    enabled: !!profileId,
  });
}

// useComponentCounts maps each component name to how many synced tests carry
// it. It shares ListComponents' key with the group-by sidebar.
export function useComponentCounts(profileId: string) {
  return useQuery({
    queryKey: keys.components(profileId),
    queryFn: () => call(() => ListComponents(profileId)),
    enabled: !!profileId,
    select: (buckets) => new Map(buckets.map((b) => [b.label, b.count])),
  });
}

// useUserSearch looks users up for the lead picker once two characters are
// typed.
export function useUserSearch(profileId: string, query: string) {
  const q = query.trim();
  return useQuery({
    queryKey: keys.userSearch(profileId, q),
    queryFn: () => call(() => SearchUsers(profileId, q)),
    enabled: !!profileId && q.length >= 2,
    staleTime: 60_000,
  });
}

// useComponentOptions loads the project's cached component names for the
// component pickers.
export function useComponentOptions(profileId: string, projectKey: string) {
  return useQuery({
    queryKey: keys.componentOptions(profileId, projectKey),
    queryFn: () => call(() => ListProjectComponents(profileId, projectKey)),
    enabled: !!profileId && !!projectKey,
  });
}
