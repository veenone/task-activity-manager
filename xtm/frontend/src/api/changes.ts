// Pending changes, Commit, conflicts, bulk edits, and transitions.

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
// phase is "" / "tests" for the Test pull or "folders" for the Test Repository
// membership pass. stage is a human-readable label for the running step.
export interface SyncProgress {
  phase: string;
  fetched: number;
  total: number;
  done: boolean;
  // stage is a human-readable label for the running sync step ("Fetching tests",
  // "Mapping folder membership", "Syncing containers", …), shown in the status bar.
  stage?: string;
}

// CommitResult mirrors syncer.CommitResult — per-Test outcome of pushing
// pending changes to Jira. Succeeded / Conflicted / Failed are disjoint sets.
export interface CommitResult {
  succeeded: string[];
  conflicted: Conflict[];
  failed: FailedCommit[];
  // created maps each newly-created Test's temporary "NEW-N" key to the real
  // Jira key it was assigned (FR-1). Optional so error-path literals can omit it.
  created?: CreatedTest[];
}

// CreatedTest mirrors syncer.CreatedTest — a locally-created Test's temp key and
// the real Jira key it received on commit.
export interface CreatedTest {
  tempKey: string;
  key: string;
}

// Conflict means the remote `updated` has advanced since the user's earliest
// pending edit on that Test, and at least one field overlaps — the Test was held
// back so they can resolve. fields lists the genuinely overlapping edits (empty
// when remoteDeleted).
export interface Conflict {
  testKey: string;
  testSummary: string;
  baseVersion: string;
  remoteVersion: string;
  remoteDeleted: boolean;
  fields: ConflictField[];
}

// ConflictField is one overlapping edit shown three-way in the resolution UI.
export interface ConflictField {
  pendingId: number;
  entityType: string;
  entityKey: string;
  field: string;
  label: string;
  base: string;
  remote: string;
  mine: string;
}

// ConflictDecision is the user's per-field choice sent to ResolveConflictMerge.
export interface ConflictDecision {
  pendingId: number;
  entityType: string;
  entityKey: string;
  field: string;
  choice: "mine" | "theirs";
  remoteValue: string;
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

// Bulk step find and replace (#311). The query is sent twice: once to scan,
// once to apply, because the backend recomputes each replacement from the step
// text the store holds rather than trusting the preview's after text.
export interface StepTextQuery {
  find: string;
  replace: string;
  fields: string[];
  matchCase: boolean;
  wholeWord: boolean;
}

export interface StepTextMatch {
  testKey: string;
  xrayId: string;
  index: number;
  field: string;
  before: string;
  after: string;
  hits: number;
}

export interface StepTextScan {
  matches: StepTextMatch[];
  skipped: string[];
  failed: BulkFailure[];
}

export interface StepTextTarget {
  testKey: string;
  xrayId: string;
  field: string;
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
