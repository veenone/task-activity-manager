// Test runs, roll-ups, and JUnit import.

import {
  GetRunStepResults as getRunStepResultsBinding,
  SetRunStepStatus as setRunStepStatusBinding,
  AnalyzeJUnitImport as analyzeJUnitImportBinding,
  ApplyJUnitImport as applyJUnitImportBinding,
  AnalyzeJUnitImportNewExec as analyzeJUnitImportNewExecBinding,
  ApplyJUnitImportNewExec as applyJUnitImportNewExecBinding,
} from "../../wailsjs/go/main/App";
import { testrepo } from "../../wailsjs/go/models";
import type { BulkEditResult } from "./changes";
import type { IterationParameter } from "./datasets";

// RunStepResult mirrors backend.RunStepResult: one step's recorded result on
// one test's execution, with no dataset and no iteration involved. Only a
// backend whose step results carry their own ids reports these, which today
// means a Zephyr profile (#371 Phase 6).
export interface RunStepResult {
  // The result's own id on the backend, which is what a write addresses. The
  // step it belongs to is definitionId, and two executions of the same test
  // have two results for the same step.
  id: string;
  definitionId: string;
  rank: number;
  status: string;
  action: string;
  data: string;
  expected: string;
  // What the executor recorded against this step. Read only: the write that
  // sets it is one of the endpoints #371 Phase 4 has still to measure.
  comment: string;
  // True while the result is journalled locally and has not been committed.
  // Same meaning, and same journal-derived source, as on an iteration (#366).
  uncommitted: boolean;
}

export async function GetRunStepResults(
  profileId: string,
  execKey: string,
  testKey: string,
  forceRefresh: boolean,
): Promise<RunStepResult[]> {
  return getRunStepResultsBinding(
    profileId,
    execKey,
    testKey,
    forceRefresh,
  ) as unknown as Promise<RunStepResult[]>;
}

export async function SetRunStepStatus(
  profileId: string,
  execKey: string,
  testKey: string,
  stepResultId: string,
  status: string,
): Promise<void> {
  return setRunStepStatusBinding(profileId, execKey, testKey, stepResultId, status);
}

export async function AnalyzeJUnitImport(
  profileId: string,
  execKey: string,
  xmlBase64: string,
): Promise<JUnitImportPreview> {
  return analyzeJUnitImportBinding(profileId, execKey, xmlBase64) as unknown as Promise<JUnitImportPreview>;
}

export async function ApplyJUnitImport(
  profileId: string,
  execKey: string,
  matches: JUnitMatch[],
): Promise<BulkEditResult> {
  return applyJUnitImportBinding(
    profileId,
    execKey,
    matches.map((match) => testrepo.JUnitMatch.createFrom(match)),
  ) as Promise<BulkEditResult>;
}

export async function AnalyzeJUnitImportNewExec(
  profileId: string,
  xmlBase64: string,
  createMissing: boolean,
): Promise<JUnitNewExecPreview> {
  return analyzeJUnitImportNewExecBinding(
    profileId,
    xmlBase64,
    createMissing,
  ) as unknown as Promise<JUnitNewExecPreview>;
}

export async function ApplyJUnitImportNewExec(
  profileId: string,
  summary: string,
  rows: JUnitNewExecRow[],
): Promise<JUnitNewExecResult> {
  return applyJUnitImportNewExecBinding(
    profileId,
    summary,
    rows.map((row) => testrepo.JUnitNewExecRow.createFrom(row)),
  ) as Promise<JUnitNewExecResult>;
}

// TestRunEntry mirrors testrepo.TestRunEntry — one execution-run of a test,
// with the execution's context (plan keys, fix versions, defects, run detail).
// createdAt and updatedAt are ISO-8601 strings from Xray (empty when unknown)
// and drive sort order (newest updated first).
export interface TestRunEntry {
  execKey: string;
  execSummary: string;
  planKeys: string[];
  environment: string;
  fixVersions: string[];
  runStatus: string;
  startedAt: string;
  finishedAt: string;
  executedBy: string;
  defects: string[];
  createdAt: string;
  updatedAt: string;
  // execIssueType is the execution's Jira issue type ("Test Execution" or "Sub
  // Test Execution"); execParentKey / execParentSummary identify a sub-task
  // execution's parent issue (empty for standalone executions).
  execIssueType: string;
  execParentKey: string;
  execParentSummary: string;
  // execCreated, execUpdated, execResolved are the Test Execution issue's
  // created, updated, and resolution timestamps (ISO-8601, empty when unknown
  // or unresolved).
  execCreated: string;
  execUpdated: string;
  execResolved: string;
}

// RunRollup mirrors testrepo.RunRollup — run-result roll-up for a Test Plan or
// Test Set across the executions that ran its member tests.
export interface RunRollup {
  passed: number;
  failed: number;
  notRun: number;
  executing: number;
  aborted: number;
  blocked: number;
  total: number;
  execCount: number;
}

// RollupRun mirrors testrepo.RollupRun — one execution's result for a member
// test in the roll-up breakdown.
export interface RollupRun {
  execKey: string;
  execSummary: string;
  status: string;
}

// RollupMember mirrors testrepo.RollupMember — a member test with its
// consolidated result and the per-execution results behind it.
export interface RollupMember {
  testKey: string;
  summary: string;
  consolidated: string;
  runs: RollupRun[];
}

// ExecMemberRun mirrors testrepo.ExecMemberRun — one member test of an
// execution enriched with run details.
export interface ExecMemberRun {
  testKey: string;
  summary: string;
  status: string;
  runStatus: string;
  startedAt: string;
  finishedAt: string;
  executedBy: string;
  environment: string;
  /** Jira Fix Version(s) of this member Test issue (from test_case), not the execution's. */
  fixVersions: string[];
  /** Bug/defect keys linked to this run result. */
  defects: string[];
  /** Free-text remark/comment on this run result. */
  comment: string;
}

// JUnitMatch mirrors testrepo.JUnitMatch -- a testcase matched to an execution member.
export interface JUnitMatch {
  testcase: string;
  testKey: string;
  summary: string;
  result: string; // "PASS" | "FAIL"
  currentRun: string;
  targetKind: "run" | "iteration";
  iterationId: string;
  parameters: IterationParameter[];
  datasetSourceKind: "test" | "plan" | "execution" | "";
  datasetSourceKey: string;
}

export interface JUnitIterationCandidate {
  iterationId: string;
  parameters: IterationParameter[];
  currentRun: string;
}

export interface JUnitMapping {
  testcase: string;
  testKey: string;
  summary: string;
  result: string;
  included: boolean;
  reason: string;
  datasetSourceKind: "test" | "plan" | "execution" | "";
  datasetSourceKey: string;
  candidateIterations: JUnitIterationCandidate[];
  match?: JUnitMatch;
}

// JUnitSkip mirrors testrepo.JUnitSkip -- a testcase skipped with a reason.
export interface JUnitSkip {
  testcase: string;
  reason: string;
}

// JUnitImportPreview mirrors testrepo.JUnitImportPreview -- the analysis result.
export interface JUnitImportPreview {
  execKey: string;
  total: number;
  matched: JUnitMatch[];
  skipped: JUnitSkip[];
  mappings: JUnitMapping[];
}

// JUnitNewExecRow mirrors testrepo.JUnitNewExecRow -- one testcase row in a
// new-execution JUnit import. When Create is true the test does not yet exist
// and will be created on commit. Result is "PASS", "FAIL", or "" (skipped in
// the report; the test is allocated but the run result is left unset).
export interface JUnitNewExecRow {
  testcase: string;
  testKey: string;
  summary: string;
  result: string;
  create: boolean;
  targetKind: "run" | "iteration";
  parameters: IterationParameter[];
  datasetSourceKind: "test" | "plan" | "execution" | "";
  datasetSourceKey: string;
  reason: string;
  candidateIterations: JUnitIterationCandidate[];
}

// JUnitNewExecPreview mirrors testrepo.JUnitNewExecPreview -- the analysis of
// a JUnit report for creating a brand-new Test Execution.
export interface JUnitNewExecPreview {
  total: number;
  rows: JUnitNewExecRow[];
  skipped: JUnitSkip[];
}

// JUnitNewExecResult mirrors testrepo.JUnitNewExecResult -- the outcome of
// ApplyJUnitImportNewExec. ExecKey is the temporary key of the new execution
// (replaced with the real Jira key on commit).
export interface JUnitNewExecResult {
  execKey: string;
  created: number;
  allocated: number;
  resultsSet: number;
  failed: string[];
}
