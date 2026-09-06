import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { call } from "@agile-suite/core";
import {
  DiscardAllPendingChanges,
  DiscardPendingChange,
  EditIssue,
  GetCreateFields,
  ListActivity,
  ListPendingChanges,
} from "../api";
import type { IssueDraft, PendingChange } from "../api";
import { keys } from "./keys";
import { invalidateWrites } from "./invalidate";

const ACTIVITY_LIMIT = 200;

export function usePendingChanges(profileId: string) {
  return useQuery({
    queryKey: keys.pending(profileId),
    queryFn: () => call(() => ListPendingChanges(profileId)),
    enabled: !!profileId,
  });
}

export function useActivity(profileId: string, key: string) {
  return useQuery({
    queryKey: keys.activity(profileId, key),
    queryFn: () => call(() => ListActivity(profileId, key, ACTIVITY_LIMIT)),
    enabled: !!profileId && !!key,
    retry: false,
  });
}

export function useCreateFields(profileId: string, type: string) {
  return useQuery({
    queryKey: keys.createFields(profileId, type),
    queryFn: () => call(() => GetCreateFields(profileId, type)),
    enabled: !!profileId && !!type,
    retry: false,
  });
}

export function useEditIssue(profileId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ key, field, value }: { key: string; field: string; value: string }) =>
      call(() => EditIssue(profileId, key, field, value)),
    onSuccess: (_, v) => invalidateWrites(qc, profileId, v.key),
  });
}

export function useDiscardChange(profileId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (change: PendingChange) => call(() => DiscardPendingChange(profileId, change.id)),
    onSuccess: (_, change) => invalidateWrites(qc, profileId, change.entityKey),
  });
}

export function useDiscardAll(profileId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => call(() => DiscardAllPendingChanges(profileId)),
    onSuccess: () => invalidateWrites(qc, profileId),
  });
}

// A PendingGroup is one issue's rows. A draft group carries its decoded
// draft; an edit group carries one row per field.
export interface PendingGroup {
  key: string;
  draft: IssueDraft | null;
  createRow: PendingChange | null;
  edits: PendingChange[];
}

// groupPending folds the journal (newest first) into one group per key,
// drafts first, then keys in the order they first appear.
export function groupPending(rows: PendingChange[]): PendingGroup[] {
  const byKey = new Map<string, PendingGroup>();
  for (const row of rows) {
    let g = byKey.get(row.entityKey);
    if (!g) {
      g = { key: row.entityKey, draft: null, createRow: null, edits: [] };
      byKey.set(row.entityKey, g);
    }
    if (row.entityType === "issue_create") {
      g.createRow = row;
      try {
        g.draft = JSON.parse(row.afterVal) as IssueDraft;
      } catch {
        g.draft = null;
      }
    } else {
      g.edits.push(row);
    }
  }
  const groups = [...byKey.values()];
  return [...groups.filter((g) => g.createRow), ...groups.filter((g) => !g.createRow)];
}
