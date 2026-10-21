// The coverage module, gap analysis, versions, and change requests.

// Coverage module data shapes (mirror internal/coverage/*.go JSON tags).
export interface CanonicalRequirement {
  id: string;
  name: string;
  category: string;
  description: string;
  createdAt: string;
  updatedAt: string;
  memberCount: number;
}

export interface ReuseRow {
  canonicalId: string;
  requirementKey: string;
  projectKey: string;
  summary: string;
  status: string;
  acceptedVersionId: string;
}

export interface ParamValue {
  id: string;
  valueLabel: string;
  valueKind: string; // value | errorcode | boundary
  errorCode: string;
  isRequired: boolean;
  notes: string;
  sortOrder: number;
}

export interface Parameter {
  id: string;
  name: string;
  kind: string;
  description: string;
  sortOrder: number;
  values: ParamValue[];
}

export interface ParamGroup {
  id: string;
  name: string;
  sortOrder: number;
  parameters: Parameter[];
}

export interface ParamModel {
  versionId: string;
  groups: ParamGroup[];
}

// NodeEdit is the upsert payload for a group/parameter/value.
export interface NodeEdit {
  kind: string; // group | parameter | value
  canonicalId?: string;
  groupId?: string;
  parameterId?: string;
  id?: string;
  name: string;
  paramKind?: string;
  valueKind?: string;
  errorCode?: string;
  isRequired?: boolean;
  notes?: string;
  sortOrder?: number;
}

export interface ValueCoverage {
  valueId: string;
  testKeys: string[];
  tested: boolean;
  runStatus: string; // UNCOVERED | NOTRUN | PASSED | FAILED
  isRequired: boolean;
}

export interface GroupCoverage {
  groupId: string;
  name: string;
  total: number;
  tested: number;
  percent: number;
}

export interface CoverageReport {
  versionId: string;
  totalValues: number;
  testedValues: number;
  percent: number;
  groups: GroupCoverage[];
  values: Record<string, ValueCoverage>;
}

export interface CoverageGap {
  groupName: string;
  paramName: string;
  valueId: string;
  valueLabel: string;
  valueKind: string;
  errorCode: string;
}

export interface CandidateTest {
  testKey: string;
  summary: string;
  status: string;
}

export interface StaleMapping {
  valueId: string;
  valueLabel: string;
  testKey: string;
}

export interface CoverageImportSummary {
  groups: number;
  parameters: number;
  values: number;
  mappedTests: number;
  skipped: number;
  warnings: string[];
}

// Coverage publish to Xray: mirrors internal/coveragepublish's ReconcileState,
// GroupStatus (DetectDrift's per-group reconcile result) and Result/GroupResult
// (PublishGroups' per-run outcome). See reconcile.go's doc comments for exactly
// what each state means before touching how these render.
export type CoveragePublishState =
  "NotPublished" | "InSync" | "LocalChanges" | "Drift" | "Conflict";

export interface CoveragePublishGroupStatus {
  groupId: string;
  groupName: string;
  containerKey: string;
  state: CoveragePublishState;
  localAdded: string[];
  localRemoved: string[];
  remoteAdded: string[];
  remoteRemoved: string[];
}

export interface CoveragePublishGroupResult {
  groupId: string;
  groupName: string;
  containerKey: string;
  created: boolean;
  added: string[];
  removed: string[];
  error?: string;
}

export interface CoveragePublishResult {
  created: number;
  updated: number;
  failed: number;
  groups: CoveragePublishGroupResult[];
}

// ProjectConfig mirrors coverage.ProjectConfig — one in-scope project for the Coverage Map.
export interface ProjectConfig {
  projectKey: string;
  role: string; // "source" | "customer"
  label: string;
  sortOrder: number;
}

// ProjectCoverageRow mirrors coverage.ProjectCoverageRow — per-project coverage rollup.
export interface ProjectCoverageRow {
  projectKey: string;
  role: string;
  label: string;
  requirementCount: number;
  functionsReused: number;
  coveredValues: number;
  totalValues: number;
  percent: number;
}

export interface PKCSSeedSummary {
  features: number;
  requirements: number;
  tests: number;
  versions: number;
  changeRequests: number;
  mappings: number;
}

// GapTest mirrors testrepo.GapTest — one comparable test row.
export interface GapTest {
  summary: string;
  description: string;
  priority: string;
  labels: string[];
  components: string[];
  folder: string;
}

// FolderMismatch mirrors testrepo.FolderMismatch — a summary-matched test whose
// folder differs between reference and target.
export interface FolderMismatch {
  summary: string;
  referenceFolder: string;
  targetFolder: string;
}

// GapResult mirrors testrepo.GapResult — a comparison outcome.
export interface GapResult {
  referenceSource: string; // "project" | "file"
  referenceCount: number;
  targetCount: number;
  matched: number;
  missingFromReference: GapTest[];
  missingFromTarget: GapTest[];
  threeWay: boolean;
  projectCount: number;
  missingFromProject: GapTest[];
  folderMismatches: FolderMismatch[];
}

// Version mirrors coverage.Version — one named snapshot of a canonical requirement's model.
export interface Version {
  id: string;
  name: string;
  status: string; // planning | beta | stable | deprecated
  notes: string;
  sortOrder: number;
  createdAt: string;
}

// ChangeRequest mirrors coverage.ChangeRequest — a proposed change scoped to one canonical.
export interface ChangeRequest {
  id: string;
  crKey: string;
  title: string;
  status: string; // open | accepted | rejected | withdrawn
  targetVersionId: string;
  risk: string; // low | medium | high
  description: string;
  createdAt: string;
  updatedAt: string;
}

// CRDecision mirrors coverage.CRDecision — one member requirement's decision on a CR.
export interface CRDecision {
  requirementKey: string;
  projectKey: string;
  decision: string; // pending | can_accept | cannot_accept
  note: string;
}

// CRImpactResult mirrors coverage.CRImpactResult — the full impact picture for one CR.
export interface CRImpactResult {
  cr: ChangeRequest;
  decisions: CRDecision[];
  canAccept: number;
  cannotAccept: number;
  pending: number;
}

// VersionShare mirrors coverage.VersionShare — member count per version for the distribution chart.
export interface VersionShare {
  versionId: string;
  versionName: string;
  status: string;
  memberCount: number;
}

// CRShare mirrors coverage.CRShare — adoption rollup per CR for the dashboard.
export interface CRShare {
  crId: string;
  title: string;
  status: string;
  canAccept: number;
  cannotAccept: number;
  pending: number;
}
