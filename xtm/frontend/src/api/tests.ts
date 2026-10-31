// Tests, folders, preconditions, requirements, containers, and steps.

import type { Bucket } from "./quality";

export interface TestCase {
  key: string;
  id: string;
  summary: string;
  description: string;
  status: string;
  priority: string;
  labels: string[];
  components: string[];
  updated: string;
  folderId: string;
  execType: string;
  cucumberScenario?: string;
  cucumberType?: string;
  genericDefinition?: string;
  /** Jira Fix Version(s) assigned to this Test issue. Empty when none are set. */
  fixVersions: string[];
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
  component: string;
  execType: string;
  review: string;
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

// StageFailure mirrors testrepo.StageFailure — one sync stage that errored
// without aborting the whole run (RND_P_4TFINT_05-336).
export interface StageFailure {
  stage: string;
  message: string;
}

// SyncLogEntry mirrors testrepo.SyncLogEntry — one sync run's outcome (FR-1.7).
export interface SyncLogEntry {
  id: number;
  startedAt: string;
  finishedAt: string;
  // "success", "partial" or "error". "partial" means the run finished and its
  // data is usable, but at least one stage did not complete.
  outcome: string;
  fetched: number;
  error: string;
  stageFailures: StageFailure[];
}

// TestSummary mirrors testrepo.TestSummary — a Test's key and current summary,
// the minimum the bulk-rename preview needs (-354).
export interface TestSummary {
  key: string;
  summary: string;
}

// TestRename mirrors testrepo.TestRename — one Test's precomputed new summary,
// plus the summary the preview was computed from so the backend can reject a
// rename a sync has since made stale.
export interface TestRename {
  key: string;
  summary: string;
  expectedBefore: string;
}

// SavedView mirrors testrepo.SavedView — a named browse filter (FR-11.4). The
// query is an opaque JSON string the frontend owns.
export interface SavedView {
  id: string;
  name: string;
  query: string;
  createdAt: string;
}

export interface Folder {
  id: string;
  parentId: string;
  name: string;
  testCount: number;
  totalTestCount: number;
}

export interface Precondition {
  key: string;
  summary: string;
  type: string;
  description: string;
  // condition is the Xray precondition definition text, distinct from the Jira
  // issue description. Empty when not set or when synced from live Jira.
  condition: string;
  // status is the Jira workflow status. Empty on a local draft that has not
  // reached Jira, and on a row cached before schema 51 until the next sync.
  status: string;
}

// PreconditionUsage mirrors testrepo.PreconditionUsage — a Precondition plus
// how many Tests reference it, for the dedicated management view (FR-13.4).
export interface PreconditionUsage {
  key: string;
  summary: string;
  type: string;
  description: string;
  // condition is the Xray precondition definition text, distinct from the Jira
  // issue description. Empty when not set or when synced from live Jira.
  condition: string;
  // status is the Jira workflow status, so the view can show and sort by it.
  status: string;
  testCount: number;
}

// PreconditionTest mirrors testrepo.PreconditionTest — one Test linked to a
// Precondition, with its summary and workflow status.
export interface PreconditionTest {
  key: string;
  summary: string;
  status: string;
}

// Requirement mirrors testrepo.Requirement — a requirement issue (possibly in
// another project) covered by Tests.
export interface Requirement {
  key: string;
  projectKey: string;
  issueType: string;
  summary: string;
  status: string;
  updated: string;
  priority: string;
  components: string;
  fixVersions: string;
  sprint: string;
  description: string;
  epicKey: string;
}

// RequirementCoverage mirrors testrepo.RequirementCoverage — a requirement plus
// its derived coverage (PASSED | FAILED | NOTRUN | UNCOVERED).
export interface RequirementCoverage {
  key: string;
  projectKey: string;
  issueType: string;
  summary: string;
  status: string;
  testCount: number;
  coverage: string;
  priority: string;
  components: string;
  fixVersions: string;
  sprint: string;
  description: string;
}

// RequirementTest mirrors testrepo.RequirementTest — one Test covering a
// requirement, with its consolidated run status.
export interface RequirementTest {
  key: string;
  summary: string;
  status: string;
  runStatus: string;
}

// RequirementSource mirrors testrepo.RequirementSource — a project to browse
// requirements from.
export interface RequirementSource {
  projectKey: string;
  issueTypes: string;
  scopeJql: string;
}

// ReqReqLink mirrors testrepo.ReqReqLink — one Requirement->Requirement
// directional issue link (e.g. "requires").
export interface ReqReqLink {
  fromKey: string;
  toKey: string;
  linkType: string;
  linkId: string;
}

// Container mirrors testrepo.Container — a Test Set, Test Plan or Test
// Execution (kind = "testset" / "testplan" / "testexec").
export interface Container {
  key: string;
  kind: string;
  summary: string;
  status: string;
  parentKey: string; // parent issue key for a sub-task Test Execution; "" for standalone
  parentSummary: string; // parent issue summary; "" when no parent or not fetched
  issueType: string; // Jira issuetype name (e.g. "Sub Test Execution"); informational
  labels: string[]; // Standard Jira labels on the container issue (any kind)
  environments: string[]; // Xray Test Environments (Test Executions only; empty otherwise)
  fixVersions: string[]; // Jira Fix Version(s), read-only (Test Executions only; empty otherwise)
  description: string; // Jira issue description (plain text)
}

// AllocateResult mirrors testrepo.AllocateResult — the outcome of a bulk
// allocation (FR-3.4–3.6).
export interface AllocateResult {
  added: string[];
  alreadyMembers: string[];
}

// DeallocateResult mirrors testrepo.DeallocateResult — the outcome of removing
// Tests from a container (FR-3.4–3.6).
export interface DeallocateResult {
  removed: string[];
  notMembers: string[];
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
  // isExternal is true when the member Test lives in a different Jira project
  // (no local test_case row) and its summary/status come from the external_test
  // cache.
  isExternal: boolean;
}

// TestPlanBoard mirrors testrepo.TestPlanBoard — a Test Plan's member Tests
// with consolidated execution status, plus a run-status histogram.
export interface TestPlanBoard {
  key: string;
  summary: string;
  description: string;
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

// JiraStepInfo mirrors app.JiraStepInfo — what Jira reports about a Test's
// steps, used to detect "Jira has steps but the tool shows none" (FR-2.5).
export interface JiraStepInfo {
  count: number;
  allBlank: boolean;
}

// Step mirrors testrepo.Step — one ordered step in an Xray Test (FR-2.5).
// xrayId is Xray's per-step identifier, kept around so a future step
// editor can target each row individually.
// TestMeta mirrors jira.TestMeta — issue-level metadata for the detail summary.
export interface TestMeta {
  created: string;
  creator: string;
  updated: string;
  updatedBy: string;
}

export interface Step {
  xrayId: string;
  index: number;
  action: string;
  data: string;
  expected: string;
  // Set when the step calls another test (Xray "test call") instead of holding
  // manual action/data/expected content.
  calledTestKey: string;
}

// TestCallLink mirrors testrepo.TestCallLink — one "call test" relationship
// (callerKey's step calls calledKey). calledExists is false for a dangling /
// cross-project call whose target isn't in the local cache.
export interface TestCallLink {
  callerKey: string;
  callerSummary: string;
  calledKey: string;
  calledSummary: string;
  calledExists: boolean;
  stepIndex: number;
}

// CustomFieldValue mirrors testrepo.CustomFieldValue: one custom field the
// backend reports, with this Test's value for it (FR-2.6).
//
// scope says what the field is attached to, and only "issue" can be edited
// here. It mirrors backend.FieldScope* and is empty on a definition cached
// before the scope existed, which customFieldScopes.ts reads as "issue".
export interface CustomFieldValue {
  fieldId: string;
  name: string;
  type: string;
  value: string;
  scope: string;
}
