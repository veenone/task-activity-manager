import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { QueryClient } from "@tanstack/react-query";
import { call } from "@agile-suite/core";
import { ComponentIssues, CreateComponent, DeleteComponent, EditComponent, ListComponentSummaries } from "../api";
import type { Component, ComponentFields } from "../api";
import { keys } from "./keys";

// The Components view's reads and writes. Both reads live under the
// catalog's components key, so whatever refreshes the pickers' component
// list (a sync, a write here) refreshes these too.

export function useComponentSummaries(profileId: string) {
  return useQuery({
    queryKey: keys.componentSummaries(profileId),
    queryFn: () => call(() => ListComponentSummaries(profileId)),
    enabled: !!profileId,
  });
}

export function useComponentIssues(profileId: string, id: number) {
  return useQuery({
    queryKey: keys.componentIssues(profileId, id),
    queryFn: () => call(() => ComponentIssues(profileId, id)),
    enabled: !!profileId && id !== 0,
  });
}

// invalidateComponentWrites refreshes what a component write changes: the
// components and their counts, the issues a delete or discard moved, and
// the journal the Commit badge counts.
export function invalidateComponentWrites(qc: QueryClient, profileId: string) {
  if (!profileId) return;
  for (const queryKey of [keys.components(profileId), keys.pending(profileId), [profileId, "issues"] as const, [profileId, "issue"] as const]) {
    qc.invalidateQueries({ queryKey });
  }
}

type Run = <T>(action: () => Promise<T>) => Promise<T>;

// Each write goes through run, SyncContext's runQuietLock, and settles into
// the invalidation, as the release writes do.

export function useCreateComponent(profileId: string, run: Run) {
  const qc = useQueryClient();
  return useMutation<Component, Error, ComponentFields>({
    mutationFn: (f) => run(() => call(() => CreateComponent(profileId, f))),
    onSettled: () => invalidateComponentWrites(qc, profileId),
  });
}

export function useEditComponent(profileId: string, run: Run) {
  const qc = useQueryClient();
  return useMutation<void, Error, ComponentFields & { id: number }>({
    mutationFn: ({ id, ...f }) => run(() => call(() => EditComponent(profileId, id, f))),
    onSettled: () => invalidateComponentWrites(qc, profileId),
  });
}

export function useDeleteComponent(profileId: string, run: Run) {
  const qc = useQueryClient();
  return useMutation<void, Error, { id: number; moveTo: number }>({
    mutationFn: (a) => run(() => call(() => DeleteComponent(profileId, a.id, a.moveTo))),
    onSettled: () => invalidateComponentWrites(qc, profileId),
  });
}
