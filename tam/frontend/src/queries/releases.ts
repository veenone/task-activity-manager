import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { QueryClient } from "@tanstack/react-query";
import { call } from "@agile-suite/core";
import { ArchiveVersion, CreateVersion, DeleteVersion, EditVersion, ListVersionSummaries, ReleaseVersion, VersionIssues } from "../api";
import type { ReleaseMove, Version, VersionFields } from "../api";
import { keys } from "./keys";

// The Releases view's reads and writes. Both reads live under the catalog's
// versions key, so whatever refreshes the pickers' release list (a sync, a
// write here) refreshes these too.

export function useVersionSummaries(profileId: string) {
  return useQuery({
    queryKey: keys.versionSummaries(profileId),
    queryFn: () => call(() => ListVersionSummaries(profileId)),
    enabled: !!profileId,
  });
}

export function useVersionIssues(profileId: string, id: number) {
  return useQuery({
    queryKey: keys.versionIssues(profileId, id),
    queryFn: () => call(() => VersionIssues(profileId, id)),
    enabled: !!profileId && id !== 0,
  });
}

// invalidateReleaseWrites refreshes what a release write changes: the
// releases and their counts, the issues a release moved, and the journal
// the Commit badge counts.
export function invalidateReleaseWrites(qc: QueryClient, profileId: string) {
  if (!profileId) return;
  for (const queryKey of [keys.versions(profileId), keys.pending(profileId), [profileId, "issues"] as const, [profileId, "issue"] as const]) {
    qc.invalidateQueries({ queryKey });
  }
}

type Run = <T>(action: () => Promise<T>) => Promise<T>;

// Each write goes through run, SyncContext's runQuietLock, since the Go side
// takes a per-profile lock; and settles into the invalidation, so a refusal
// still refreshes a list that may have moved.

export function useCreateRelease(profileId: string, run: Run) {
  const qc = useQueryClient();
  return useMutation<Version, Error, VersionFields>({
    mutationFn: (f) => run(() => call(() => CreateVersion(profileId, f.name, f.description, f.startDate, f.releaseDate))),
    onSettled: () => invalidateReleaseWrites(qc, profileId),
  });
}

export function useEditRelease(profileId: string, run: Run) {
  const qc = useQueryClient();
  return useMutation<void, Error, VersionFields & { id: number }>({
    mutationFn: (f) => run(() => call(() => EditVersion(profileId, f.id, f.name, f.description, f.startDate, f.releaseDate))),
    onSettled: () => invalidateReleaseWrites(qc, profileId),
  });
}

export interface ReleaseArgs {
  id: number;
  date: string;
  moveTo: number;
  moves: ReleaseMove[];
}

export function useReleaseRelease(profileId: string, run: Run) {
  const qc = useQueryClient();
  return useMutation<void, Error, ReleaseArgs>({
    mutationFn: (a) => run(() => call(() => ReleaseVersion(profileId, a.id, a.date, a.moveTo, a.moves))),
    onSettled: () => invalidateReleaseWrites(qc, profileId),
  });
}

export function useArchiveRelease(profileId: string, run: Run) {
  const qc = useQueryClient();
  return useMutation<void, Error, { id: number; archived: boolean }>({
    mutationFn: (a) => run(() => call(() => ArchiveVersion(profileId, a.id, a.archived))),
    onSettled: () => invalidateReleaseWrites(qc, profileId),
  });
}

export function useDeleteRelease(profileId: string, run: Run) {
  const qc = useQueryClient();
  return useMutation<void, Error, { id: number; moveTo: number }>({
    mutationFn: (a) => run(() => call(() => DeleteVersion(profileId, a.id, a.moveTo))),
    onSettled: () => invalidateReleaseWrites(qc, profileId),
  });
}
