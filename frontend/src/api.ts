// api.ts — typed access to the Wails Go backend.
//
// The data interfaces below are plain shapes that match the JSON the backend
// returns. They are intentionally separate from the generated wailsjs model
// classes so plain object literals (initial state, query objects) type-check.

export {
  Health,
  ListProfiles,
  CreateProfile,
  DeleteProfile,
  TestConnection,
  SyncProfile,
  GetSyncState,
  ListFolders,
  GetTestPreconditions,
  ListTests,
  GetTest,
  EditTestField,
  DiscardPendingChange,
  ListPendingChanges,
  ListAuditEntries,
  CommitPendingChanges,
  BulkEditTests,
  GetTestTransitions,
  TransitionTest,
} from "../wailsjs/go/main/App";
export { EventsOn } from "../wailsjs/runtime/runtime";

export interface HealthInfo {
  ok: boolean;
  error: string;
  dbPath: string;
  logPath: string;
}

export interface Profile {
  id: string;
  name: string;
  jiraUrl: string;
  projectKey: string;
  createdAt: string;
}

export interface TestCase {
  key: string;
  id: string;
  summary: string;
  description: string;
  status: string;
  priority: string;
  labels: string[];
  updated: string;
  folderId: string;
}

export interface TestPage {
  tests: TestCase[];
  total: number;
}

export interface TestQuery {
  search: string;
  status: string;
  folderId: string;
  sortBy: string;
  desc: boolean;
  limit: number;
  offset: number;
}

export interface SyncState {
  profileId: string;
  lastSyncedAt: string;
  testCount: number;
}

export interface Folder {
  id: string;
  parentId: string;
  name: string;
}

export interface Precondition {
  key: string;
  summary: string;
  type: string;
  description: string;
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

// SyncProgress mirrors the Go syncer.Progress payload emitted on "sync:progress".
export interface SyncProgress {
  fetched: number;
  total: number;
  done: boolean;
}

// CommitResult mirrors syncer.CommitResult — per-Test outcome of pushing
// pending changes to Jira. Succeeded / Conflicted / Failed are disjoint sets.
export interface CommitResult {
  succeeded: string[];
  conflicted: Conflict[];
  failed: FailedCommit[];
}

// Conflict means the remote `updated` has advanced since the user's earliest
// pending edit on that Test — the PUT was held back so they can resolve.
export interface Conflict {
  testKey: string;
  baseVersion: string;
  remoteVersion: string;
}

export interface FailedCommit {
  testKey: string;
  error: string;
}

// Bulk-edit (FR-3) operation descriptor and result types.
export interface BulkEdit {
  operation: string;
  field: string;
  value: string;
}

export interface BulkEditResult {
  succeeded: string[];
  failed: BulkFailure[];
}

export interface BulkFailure {
  testKey: string;
  error: string;
}

// Transition is one workflow move available from a Test's current status
// (FR-4.2). The detail panel uses {name → to} as the dropdown label.
export interface Transition {
  id: string;
  name: string;
  to: string;
}

// errMsg renders any thrown value (unknown in strict mode) as a string.
export function errMsg(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === "string" ? e : String(e);
}
