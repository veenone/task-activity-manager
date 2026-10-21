// Dashboard statistics, duplicate reports, and the traceability Sankey.

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
  byComponent: Bucket[];
  updatedTrend: Bucket[];
  byRunStatus: Bucket[];
  byCoverage: Bucket[];
  byRequirement: Bucket[];
}

// Duplicate management (mirrors testrepo shapes).
export interface DuplicateMember {
  key: string;
  summary: string;
  description: string;
  status: string;
  folderId: string;
}

export interface DuplicateGroup {
  normalizedSummary: string;
  displaySummary: string;
  stepsVerdict: "identical" | "differ" | "unscanned";
  members: DuplicateMember[];
}

export interface DuplicateReport {
  groups: DuplicateGroup[];
  groupCount: number;
  testCount: number;
  stepsIdentical: number;
  stepsDiffer: number;
  stepsUnscanned: number;
  excluded: number;
  scannedAt: string;
}

// Precondition duplicate management (RND_P_4TFINT_05-323). Mirrors the testrepo
// PreconditionDuplicate* shapes. Preconditions have no object-level steps, so
// the verdict compares definition (condition + description) text and there is
// no "unscanned" state.
export interface PreconditionDuplicateMember {
  key: string;
  summary: string;
  type: string;
  condition: string;
  description: string;
  testCount: number;
}

export interface PreconditionDuplicateGroup {
  normalizedSummary: string;
  displaySummary: string;
  definitionVerdict: "identical" | "differ";
  members: PreconditionDuplicateMember[];
}

export interface PreconditionDuplicateReport {
  groups: PreconditionDuplicateGroup[];
  groupCount: number;
  preconditionCount: number;
  definitionIdentical: number;
  definitionDiffer: number;
  excluded: number;
  scannedAt: string;
}

// Traceability Sankey (FR-9): Plan -> Execution -> run-status flow.
export interface SankeyNode {
  id: string;
  label: string;
  layer: number;
  value: number;
}

export interface SankeyLink {
  source: string;
  target: string;
  value: number;
}

export interface Sankey {
  nodes: SankeyNode[];
  links: SankeyLink[];
}
