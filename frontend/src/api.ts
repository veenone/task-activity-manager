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
  ListAllPreconditions,
  SetTestPreconditions,
  EditPreconditionField,
  CreatePrecondition,
  BulkAssociatePreconditions,
  GetTestContainers,
  ListContainers,
  AllocateTests,
  CreateContainerAndAllocate,
  SeedSampleContainers,
  GetTestPlanBoard,
  MoveTestToFolder,
  BulkMoveToFolder,
  ListTests,
  ListMatchingKeys,
  GetTest,
  EditTestField,
  DiscardPendingChange,
  ListPendingChanges,
  ListAuditEntries,
  CommitPendingChanges,
  BulkEditTests,
  GetTestTransitions,
  TransitionTest,
  GetBulkTransitionOptions,
  BulkTransitionTests,
  GetTestSteps,
  EditTestStepField,
  DeleteTestStep,
  AddTestStep,
  ReorderTestSteps,
  GetStatistics,
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
  containerKey: string;
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

// Container mirrors testrepo.Container — a Test Set, Test Plan or Test
// Execution (kind = "testset" / "testplan" / "testexec").
export interface Container {
  key: string;
  kind: string;
  summary: string;
  status: string;
}

// AllocateResult mirrors testrepo.AllocateResult — the outcome of a bulk
// allocation (FR-3.4–3.6).
export interface AllocateResult {
  added: string[];
  alreadyMembers: string[];
}

// CreateContainerResult mirrors testrepo.CreateContainerResult — the outcome
// of creating a new container and allocating Tests to it (FR-3.4–3.6).
export interface CreateContainerResult {
  tempKey: string;
  added: number;
}

// SeedResult mirrors testrepo.SeedResult — how much sample container data was
// generated.
export interface SeedResult {
  sets: number;
  plans: number;
  executions: number;
  linked: number;
}

// TestPlanBoardRow mirrors testrepo.TestPlanBoardRow — one Test on a Test Plan
// board (FR-13.7) with its consolidated execution status.
export interface TestPlanBoardRow {
  testKey: string;
  summary: string;
  status: string;
  runStatus: string;
}

// TestPlanBoard mirrors testrepo.TestPlanBoard — a Test Plan's member Tests
// with consolidated execution status, plus a run-status histogram.
export interface TestPlanBoard {
  key: string;
  summary: string;
  rows: TestPlanBoardRow[];
  runCounts: Bucket[];
}

// ContainerMembership mirrors testrepo.ContainerMembership — a Test Set, Test
// Plan or Test Execution a Test belongs to (FR-1.3). kind is
// "testset" / "testplan" / "testexec"; runStatus is the Test Run result for
// execution memberships, empty otherwise.
export interface ContainerMembership {
  key: string;
  kind: string;
  summary: string;
  status: string;
  runStatus: string;
}

// Step mirrors testrepo.Step — one ordered step in an Xray Test (FR-2.5).
// xrayId is Xray's per-step identifier, kept around so a future step
// editor can target each row individually.
export interface Step {
  xrayId: string;
  index: number;
  action: string;
  data: string;
  expected: string;
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

// BulkTransitionOptions is what the bulk-transition modal asks for on
// open: a histogram of current statuses across the selection, and the
// union of target statuses reachable from at least one of those.
export interface BulkTransitionOptions {
  currentStatusCounts: { [status: string]: number };
  reachableTargets: string[];
}

// BulkTransitionResult mirrors app.BulkTransitionResult — per-Test outcome
// of a bulk transition. Succeeded / Skipped / Failed are disjoint sets.
export interface BulkTransitionResult {
  succeeded: string[];
  skipped: BulkTransitionSkip[];
  failed: BulkFailure[];
}

export interface BulkTransitionSkip {
  testKey: string;
  reason: string;
}

// Bucket is one (label, count) pair in a dashboard distribution (FR-9).
export interface Bucket {
  label: string;
  count: number;
}

// Statistics mirrors testrepo.Statistics — the per-profile dashboard rollup
// computed from the local store (FR-9).
export interface Statistics {
  total: number;
  pendingChanges: number;
  executedTests: number;
  testSets: number;
  testPlans: number;
  testExecutions: number;
  testsInSet: number;
  testsInPlan: number;
  byStatus: Bucket[];
  byPriority: Bucket[];
  byLabel: Bucket[];
  byFolder: Bucket[];
  updatedTrend: Bucket[];
  byRunStatus: Bucket[];
}

// errMsg renders any thrown value (unknown in strict mode) as a string.
export function errMsg(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === "string" ? e : String(e);
}
