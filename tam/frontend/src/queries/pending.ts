import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { call } from "@agile-suite/core";
import {
  AddLink,
  DiscardAllPendingChanges,
  DiscardPendingChange,
  EditIssue,
  ENTITY_SPRINT_CREATE,
  GetCreateFields,
  GetLinkTypes,
  ListActivity,
  ListPendingChanges,
} from "../api";
import { isMoveEntity } from "../api";
import type { DraftSprint, IssueDraft, LinkDraft, PendingChange } from "../api";
import { keys } from "./keys";
import { invalidateWrites } from "./invalidate";
import { invalidateSprintWrites } from "./sprints";

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

// CREATE_FIELDS_FRESH_FOR is how long a type's required-field list is served
// without asking Jira again. The list only changes when someone edits the
// project's screens, and the create dialog gates its submit button on this
// query, so refetching it on every open and every type toggle made the dialog
// wait on a round trip whose answer had not changed.
const CREATE_FIELDS_FRESH_FOR = 10 * 60 * 1000;

export function useCreateFields(profileId: string, type: string) {
  return useQuery({
    queryKey: keys.createFields(profileId, type),
    queryFn: () => call(() => GetCreateFields(profileId, type)),
    enabled: !!profileId && !!type,
    staleTime: CREATE_FIELDS_FRESH_FOR,
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
    onSuccess: (_, change) => {
      invalidateWrites(qc, profileId, change.entityKey);
      // Discarding a draft sprint removes it from every picker and puts the
      // cards moved into it back, so the sprint lists refresh too.
      if (change.entityType === ENTITY_SPRINT_CREATE) invalidateSprintWrites(qc, profileId);
    },
  });
}

export function useDiscardAll(profileId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => call(() => DiscardAllPendingChanges(profileId)),
    onSuccess: () => {
      invalidateWrites(qc, profileId);
      // A discarded draft sprint among the rows removes it from every
      // picker, so Discard all refreshes the sprint lists too.
      invalidateSprintWrites(qc, profileId);
    },
  });
}

export function useLinkTypes(profileId: string) {
  return useQuery({
    queryKey: keys.linkTypes(profileId),
    queryFn: () => call(() => GetLinkTypes(profileId)),
    enabled: !!profileId,
    staleTime: 10 * 60 * 1000,
  });
}

export function useAddLink(profileId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ key, link }: { key: string; link: LinkDraft }) => call(() => AddLink(profileId, key, link)),
    onSuccess: (_, v) => invalidateWrites(qc, profileId, v.key),
  });
}

// useDiscardById discards a journal row known only by id and issue key,
// which is what the Links tab has for a pending link.
export function useDiscardById(profileId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id }: { id: number; key: string }) => call(() => DiscardPendingChange(profileId, id)),
    onSuccess: (_, v) => invalidateWrites(qc, profileId, v.key),
  });
}

// A PendingGroup is one issue's rows, or one draft sprint. A draft group
// carries its decoded draft; a sprint group its decoded DraftSprint; an edit
// group carries one row per field; a link group one row per journaled link;
// a move group the board rows, which are their own kind because they are
// pushed their own way and read as places rather than as field values.
export interface PendingGroup {
  key: string;
  draft: IssueDraft | null;
  createRow: PendingChange | null;
  sprint: DraftSprint | null;
  sprintRow: PendingChange | null;
  edits: PendingChange[];
  links: { row: PendingChange; link: LinkDraft }[];
  moves: PendingChange[];
}

// groupPending folds the journal (newest first) into one group per key:
// draft sprints first, since Commit creates them first, then drafts, then
// keys in the order they first appear.
export function groupPending(rows: PendingChange[]): PendingGroup[] {
  const byKey = new Map<string, PendingGroup>();
  for (const row of rows) {
    let g = byKey.get(row.entityKey);
    if (!g) {
      g = { key: row.entityKey, draft: null, createRow: null, sprint: null, sprintRow: null, edits: [], links: [], moves: [] };
      byKey.set(row.entityKey, g);
    }
    if (row.entityType === "issue_create") {
      g.createRow = row;
      try {
        g.draft = JSON.parse(row.afterVal) as IssueDraft;
      } catch {
        g.draft = null;
      }
    } else if (row.entityType === ENTITY_SPRINT_CREATE) {
      g.sprintRow = row;
      try {
        g.sprint = JSON.parse(row.afterVal) as DraftSprint;
      } catch {
        g.sprint = null;
      }
    } else if (isMoveEntity(row.entityType)) {
      g.moves.push(row);
    } else if (row.entityType === "link") {
      try {
        g.links.push({ row, link: JSON.parse(row.afterVal) as LinkDraft });
      } catch {
        g.edits.push(row);
      }
    } else {
      g.edits.push(row);
    }
  }
  const groups = [...byKey.values()];
  return [
    ...groups.filter((g) => g.sprintRow),
    ...groups.filter((g) => !g.sprintRow && g.createRow),
    ...groups.filter((g) => !g.sprintRow && !g.createRow),
  ];
}
