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
import { ENTITY_BOARD_CREATE, SPRINT_ENTITIES, isMoveEntity } from "../api";
import type { DraftBoard, DraftSprint, IssueDraft, LinkDraft, PendingChange } from "../api";
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
      // cards moved into it back, and discarding a sprint edit puts the old
      // name and dates back, so the sprint lists refresh too.
      if (SPRINT_ENTITIES.includes(change.entityType)) invalidateSprintWrites(qc, profileId);
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

// A PendingGroup is one issue's rows, one sprint's, or one draft board's
// with its decoded DraftBoard. A draft group
// carries its decoded draft; a draft sprint group its decoded DraftSprint;
// a real sprint's group its edit or delete row in sprintChanges; an edit
// group carries one row per field; a link group one row per journaled link;
// a move group the board rows, which are their own kind because they are
// pushed their own way and read as places rather than as field values.
//
// id is the group's own identity and key the entity key it shows. They
// differ because a sprint's key is its id and a draft board's is its
// negative id, so a draft sprint -1 and a draft board -1 share a key and
// must not share a group.
export interface PendingGroup {
  id: string;
  key: string;
  draft: IssueDraft | null;
  createRow: PendingChange | null;
  sprint: DraftSprint | null;
  sprintRow: PendingChange | null;
  sprintChanges: PendingChange[];
  board: DraftBoard | null;
  boardRow: PendingChange | null;
  edits: PendingChange[];
  links: { row: PendingChange; link: LinkDraft }[];
  moves: PendingChange[];
}

// groupKind is the namespace a row's group lives in.
function groupKind(entityType: string): string {
  if (SPRINT_ENTITIES.includes(entityType)) return "sprint";
  if (entityType === ENTITY_BOARD_CREATE) return "board";
  return "issue";
}

// groupPending folds the journal (newest first) into one group per key and
// kind: draft boards first and draft sprints next, in the order Commit
// creates them, then the
// edits and deletes of real sprints, pushed right after, then drafts, then
// keys in the order they first appear.
export function groupPending(rows: PendingChange[]): PendingGroup[] {
  const byKey = new Map<string, PendingGroup>();
  for (const row of rows) {
    const id = `${groupKind(row.entityType)}:${row.entityKey}`;
    let g = byKey.get(id);
    if (!g) {
      g = { id, key: row.entityKey, draft: null, createRow: null, sprint: null, sprintRow: null, sprintChanges: [], board: null, boardRow: null, edits: [], links: [], moves: [] };
      byKey.set(id, g);
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
    } else if (row.entityType === ENTITY_BOARD_CREATE) {
      g.boardRow = row;
      try {
        g.board = JSON.parse(row.afterVal) as DraftBoard;
      } catch {
        g.board = null;
      }
    } else if (SPRINT_ENTITIES.includes(row.entityType)) {
      g.sprintChanges.push(row);
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
  const isSprintChange = (g: PendingGroup) => !g.sprintRow && g.sprintChanges.length > 0;
  const isIssue = (g: PendingGroup) => !g.boardRow && !g.sprintRow && !isSprintChange(g);
  return [
    ...groups.filter((g) => g.boardRow),
    ...groups.filter((g) => g.sprintRow),
    ...groups.filter(isSprintChange),
    ...groups.filter((g) => isIssue(g) && g.createRow),
    ...groups.filter((g) => isIssue(g) && !g.createRow),
  ];
}
