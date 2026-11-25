import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { call } from "@agile-suite/core";
import { GetDependencyGraph, RemoveLink, RetypeLink } from "../api";
import type { Link } from "../api";
import { keys } from "./keys";
import { invalidateWrites } from "./invalidate";

// useDependencyGraph is the Dependencies view's one read (#612): every link
// of the profile with the issues it joins, filtered in the view.
export function useDependencyGraph(profileId: string) {
  return useQuery({
    queryKey: keys.dependencies(profileId),
    queryFn: () => call(() => GetDependencyGraph(profileId)),
    enabled: !!profileId,
  });
}

// useRemoveLink journals the removal of a cached link; Commit deletes it.
export function useRemoveLink(profileId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ key, link }: { key: string; link: Link }) => call(() => RemoveLink(profileId, key, link)),
    onSuccess: (_, v) => invalidateWrites(qc, profileId, v.key),
  });
}

// useRetypeLink journals a new type for a cached link as a removal and an add.
export function useRetypeLink(profileId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ key, link, type }: { key: string; link: Link; type: string }) => call(() => RetypeLink(profileId, key, link, type)),
    onSuccess: (_, v) => invalidateWrites(qc, profileId, v.key),
  });
}
