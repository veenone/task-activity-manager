// The pending-change journal: entity names, Commit's results, and conflicts.

import * as App from "../../wailsjs/go/main/App";
import { EDITABLE_FIELDS } from "./edits";

// The four journal entity types a board move writes, mirroring
// issuerepo.EntityTransition, EntitySprintMove, EntityRank, and
// EntityIssueBoard. Both dialogs that show pending work branch on them, and
// the board reads them to know which of its cards carry a pending move.
export const ENTITY_TRANSITION = "issue_transition";
export const ENTITY_SPRINT_MOVE = "issue_sprint";
export const ENTITY_RANK = "issue_rank";
export const ENTITY_ISSUE_BOARD = "issue_board";
export const MOVE_ENTITIES: string[] = [ENTITY_TRANSITION, ENTITY_SPRINT_MOVE, ENTITY_RANK, ENTITY_ISSUE_BOARD];

// The field each of those rows carries, mirroring issuerepo.FieldStatusID,
// FieldSprintID, and FieldRank. An issue_board row has no field constant
// here: unlike a status or a sprint, an issue can be queued onto more than
// one board, so issuerepo.BoardField folds the board's id into the field
// itself and there is no one fixed string for MOVE_FIELDS to name.
export const FIELD_STATUS_ID = "statusId";
export const FIELD_SPRINT_ID = "sprintId";
export const FIELD_RANK = "rank";

// MOVE_LABELS is the one word each board move goes by, in the Pending
// changes dialog, the Activity tab, and the conflict table. One vocabulary
// for the four moves: a surface that invented its own would have the same
// card read "Status" in one dialog and "statusId" in the next.
export const MOVE_LABELS: Record<string, string> = {
  [ENTITY_TRANSITION]: "Status",
  [ENTITY_SPRINT_MOVE]: "Sprint",
  [ENTITY_RANK]: "Rank",
  [ENTITY_ISSUE_BOARD]: "Board",
};

// MOVE_FIELDS names the entity behind a field, for the surfaces that hold
// only the field: a held board write reaches the conflict card as
// "statusId" or "sprintId" and has to read as Status or Sprint there too.
// An issue_board row is never held (committer.heldBoardRow), so it never
// reaches a surface that has only its field and needs this map.
export const MOVE_FIELDS: Record<string, string> = {
  [FIELD_STATUS_ID]: ENTITY_TRANSITION,
  [FIELD_SPRINT_ID]: ENTITY_SPRINT_MOVE,
  [FIELD_RANK]: ENTITY_RANK,
};

// isMoveEntity says whether a journal row or an audit entry is a board
// move rather than an edit, a link, or a create.
export function isMoveEntity(entityType: string): boolean {
  return MOVE_ENTITIES.includes(entityType);
}

export function fieldLabel(field: string): string {
  return EDITABLE_FIELDS.find((f) => f.id === field)?.label ?? MOVE_LABELS[MOVE_FIELDS[field]] ?? field;
}

// UnpushableEdit mirrors issuerepo.UnpushableEdit: one journalled edit whose
// field is not on the edit screen Jira reports for its issue. Commit would be
// refused for it. It is named where the pending work is shown and never
// discarded on the user's behalf.
export interface UnpushableEdit {
  id: number;
  key: string;
  field: string;
}

export interface PendingChange {
  id: number;
  entityType: string;
  entityKey: string;
  field: string;
  beforeVal: string;
  afterVal: string;
  baseVersion: string;
  createdAt: string;
}

// ENTITY_SPRINT_CREATE is the journal entity of a sprint drafted in TAM. Its
// entityKey is the draft's negative id and its afterVal a DraftSprint.
export const ENTITY_SPRINT_CREATE = "sprint_create";

// ENTITY_WORKLOG is the journal entity of work logged against an issue. Its
// key is the issue key, its field the entry's started stamp, and its afterVal
// the entry as JSON.
export const ENTITY_WORKLOG = "worklog";

// ENTITY_BOARD_CREATE is the journal entity of a board drafted in TAM,
// mirroring issuerepo.EntityBoardCreate. Its entityKey is the draft's
// negative id and its afterVal a DraftBoard.
export const ENTITY_BOARD_CREATE = "board_create";

// DraftBoard mirrors issuerepo.DraftBoard: what a board_create row carries.
export interface DraftBoard {
  name: string;
  type: string;
  filterName: string;
  jql: string;
}

// ENTITY_SPRINT_EDIT and ENTITY_SPRINT_DELETE are the journal entities of an
// edit and a delete of a sprint Jira holds, mirroring issuerepo's. Their
// entityKey is the sprint id. An edit's afterVal is a SprintEdit and its
// beforeVal the cached name, goal and dates; a delete's afterVal is
// { boardId, name } and its beforeVal the name.
export const ENTITY_SPRINT_EDIT = "sprint_edit";
export const ENTITY_SPRINT_DELETE = "sprint_delete";
// ENTITY_SPRINT_START and ENTITY_SPRINT_COMPLETE are a start and a completion
// waiting for Commit, keyed by the sprint id (a draft's negative id for a
// start). A start's afterVal is a SprintStart, a completion's a
// SprintComplete; neither has a beforeVal.
export const ENTITY_SPRINT_START = "sprint_start";
export const ENTITY_SPRINT_COMPLETE = "sprint_complete";
export const SPRINT_ENTITIES: string[] = [
  ENTITY_SPRINT_CREATE, ENTITY_SPRINT_EDIT, ENTITY_SPRINT_DELETE, ENTITY_SPRINT_START, ENTITY_SPRINT_COMPLETE,
];

// SprintStart mirrors issuerepo.SprintStart: what a sprint_start row carries.
export interface SprintStart {
  boardId: number;
  name: string;
  goal: string;
  startDate: string;
  endDate: string;
}

// SprintComplete mirrors issuerepo.SprintComplete: what a sprint_complete row
// carries. moveTo is empty for the backlog.
export interface SprintComplete {
  boardId: number;
  name: string;
  moveTo: string;
  moveToName: string;
}

// SprintEdit mirrors issuerepo.SprintEdit: what a sprint_edit row carries.
export interface SprintEdit {
  boardId: number;
  name: string;
  goal: string;
  startDate: string;
  endDate: string;
  clearGoal: boolean;
}

// DraftSprint mirrors issuerepo.DraftSprint: what a sprint_create row carries.
export interface DraftSprint {
  boardId: number;
  boardName: string;
  name: string;
  goal: string;
  startDate: string;
  endDate: string;
}

export interface AuditEntry {
  id: number;
  occurredAt: string;
  actor: string;
  entityType: string;
  entityKey: string;
  action: string;
  field: string;
  beforeVal: string;
  afterVal: string;
  note: string;
}

export interface FieldConflict {
  field: string;
  base: string;
  mine: string;
  remote: string;
}

export interface Conflict {
  key: string;
  summary: string;
  remoteVersion: string;
  fields: FieldConflict[];
}

// CommitFailure is one push that did not land. entityType and rowId name
// the journal row it was, which is what an Undo on a board failure
// discards, and retryable is false for the ones that will fail identically
// forever. The three are optional for the reason Issue.pending is: the
// backend always sends them, but fixtures written before the board pass
// existed do not spell them out. The statuses a refused transition could
// have reached instead are in the error sentence itself, so nothing here
// reads them a second time.
export interface CommitFailure {
  key: string;
  error: string;
  entityType?: string;
  rowId?: number;
  retryable?: boolean;
}

// CommitMove is one board write a Commit settled: a card transitioned,
// moved to a sprint, or ranked. target is named rather than numbered, and
// side says which side of the neighbour a rank went; satisfied marks a row
// Jira already agreed with.
export interface CommitMove {
  key: string;
  entityType: string;
  target: string;
  side: string;
  satisfied: boolean;
}

// CommitHeld mirrors committer.Held: a row the Commit did not send because
// something it names was not created in Jira. It stays pending; reason is
// the sentence that says what it waits for.
export interface CommitHeld {
  key: string;
  entityType: string;
  rowId: number;
  waitsFor: string;
  reason: string;
}

export interface CommitResult {
  committed: string[];
  // leftOut names the draft's extra fields the create did not send. TAM
  // leaves out a field nothing could confirm is on the issue type's create
  // screen, rather than letting Jira refuse the whole create over it, and
  // the banner is where that stops being silent. Optional for the reason
  // Issue.pending is: fixtures written before it do not carry it.
  created: { tempKey: string; key: string; leftOut?: string[] }[];
  // createdSprints and held are optional for the same reason CommitFailure's
  // fields are: fixtures written before phased Commit do not spell them out.
  createdSprints?: { draftId: number; id: number; name: string }[];
  // sprintsChanged names each pushed sprint edit, start, completion or
  // delete, "Sprint 12 edited"; optional for the same reason.
  sprintsChanged?: string[];
  linked: { key: string; toKey: string; type: string }[];
  // logged names each worklog entry pushed; optional for the reason
  // createdSprints is, since fixtures written before it do not carry it.
  logged?: { key: string; timeSpent: string }[];
  // moved is optional for the same reason CommitFailure's fields are.
  moved?: CommitMove[];
  conflicts: Conflict[];
  failures: CommitFailure[];
  held?: CommitHeld[];
  remaining: number;
}

export const ListUnpushableEdits: (profileId: string) => Promise<UnpushableEdit[]> =
  App.ListUnpushableEdits;
export const ListPendingChanges: (profileId: string) => Promise<PendingChange[]> = App.ListPendingChanges;
export const DiscardPendingChange: (profileId: string, id: number) => Promise<void> =
  App.DiscardPendingChange;
export const DiscardAllPendingChanges: (profileId: string) => Promise<number> =
  App.DiscardAllPendingChanges;
export const CommitPendingChanges: (profileId: string) => Promise<CommitResult> =
  App.CommitPendingChanges;
export const ResolveConflictOverride: (profileId: string, key: string, remoteVersion: string) => Promise<void> =
  App.ResolveConflictOverride;
export const ResolveConflictKeepRemote: (profileId: string, key: string) => Promise<void> =
  App.ResolveConflictKeepRemote;
export const ListActivity: (profileId: string, key: string, limit: number) => Promise<AuditEntry[]> =
  App.ListActivity;
