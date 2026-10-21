// Spreadsheet imports, the New Test draft, and reviews.

// Import types (FR-10) mirror the testrepo shapes.
export interface ImportPreview {
  headers: string[];
  rowCount: number;
}

// PreconditionImportMapping mirrors testrepo.PreconditionImportMapping: the
// column header each precondition field reads from ("" when unmapped).
export interface PreconditionImportMapping {
  summary: string;
  type: string;
  condition: string;
  description: string;
  labels: string;
}

// PreconditionImportResult mirrors testrepo.PreconditionImportResult.
export interface PreconditionImportResult {
  created: number;
  newCount: number;
  existingCount: number;
  rows: { row: number; summary: string; type: string; status: string }[];
  errors: { row: number; message: string }[];
  unknownTypes: string[];
}

export interface ImportMapping {
  summary: string;
  description: string;
  priority: string;
  labels: string;
  components: string;
  folder: string;
  action: string;
  data: string;
  expected: string;
  testType: string;
  cucumberScenario: string;
  cucumberType: string;
  genericDefinition: string;
}

export interface ImportError {
  row: number;
  message: string;
}

// UnknownComponent mirrors testrepo.UnknownComponent — a component name in an
// import file that the target project does not have. suggestion holds a known
// component differing only in case, and is empty when there is no near match.
export interface UnknownComponent {
  name: string;
  suggestion: string;
}

export interface ImportResult {
  created: number;
  skipped: number;
  errors: ImportError[];
  unknownComponents: UnknownComponent[];
}

// RequirementImportRow mirrors testrepo.RequirementImportRow — one parsed file
// row with its existence classification. status is "new" or "existing".
export interface RequirementImportRow {
  summary: string;
  description: string;
  priority: string;
  components: string;
  fixVersions: string;
  status: string;
}

// RequirementImportPreview mirrors testrepo.RequirementImportPreview — the
// parse + classify result shown before the user commits.
export interface RequirementImportPreview {
  rows: RequirementImportRow[];
  newCount: number;
  existingCount: number;
}

// RequirementImportResult mirrors testrepo.RequirementImportResult — the
// outcome of a completed import.
export interface RequirementImportResult {
  created: number;
  skippedExisting: number;
  errors: ImportError[];
}

// StepDraft mirrors testrepo.StepDraft — one step in the New Test form (FR-1).
export interface StepDraft {
  action: string;
  data: string;
  expected: string;
}

// TestDraft mirrors testrepo.TestDraft — the New Test form payload (FR-1).
// labels is space-separated, components comma-separated, matching import.
export interface TestDraft {
  summary: string;
  description: string;
  priority: string;
  labels: string;
  components: string;
  execType: string;
  cucumberScenario: string;
  cucumberType: string;
  genericDefinition: string;
  folderId: string;
  steps: StepDraft[];
  precondKeys: string[];
  // The create screen's required fields, Jira-shaped and keyed by field id.
  fields?: Record<string, unknown>;
}

// Review mirrors testrepo.Review — a Test's review state. An empty verdict
// means it hasn't been reviewed.
export interface Review {
  verdict: string; // "approved" | "rejected" | "pending" | ""
  reviewer: string;
  note: string;
  reviewedAt: string;
}
