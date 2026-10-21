// Field definitions and defects linked to Tests.

// FieldOption mirrors jira.FieldOption, one allowed value of a FieldDef.
export interface FieldOption {
  id: string;
  value: string;
}

// FieldDef mirrors jira.FieldDef: one Jira field as a screen offers it, not
// specific to any one screen. Bug create and requirement create use it today.
// Type is: "text" | "option" | "version" | "versions" | "number" | "date" |
// "array".
export interface FieldDef {
  id: string;
  name: string;
  required: boolean;
  type: string;
  allowedValues: FieldOption[];
}

// BugDetail mirrors jira.BugDetail - the extended fields for a defect issue
// fetched lazily on detail-panel open (description + three custom fields).
export interface BugDetail {
  description: string;
  defectOrigin: string;
  defectAnalysis: string;
  correctionDetails: string;
  reporter: string;
  severity: string;
}

// Bug mirrors testrepo.Bug - a cached defect issue (possibly cross-project)
// linked to Tests. Returned by ListBugsForContainer for an execution's related
// defects.
export interface Bug {
  key: string;
  projectKey: string;
  issueType: string;
  summary: string;
  status: string;
  priority: string;
  updated: string;
}

// BugWithTests mirrors testrepo.BugWithTests — a bug plus the Test keys it
// affects, for the Bugs panel.
export interface BugWithTests {
  key: string;
  projectKey: string;
  // issueType is the Jira issue type of the bug (e.g. "Bug", "Defect").
  issueType: string;
  summary: string;
  status: string;
  priority: string;
  // updated is the Jira last-updated timestamp for the bug issue.
  updated: string;
  testKeys: string[];
}

// TestBug mirrors testrepo.TestBug — a bug linked to one Test, for the
// test-detail section.
export interface TestBug {
  key: string;
  projectKey: string;
  summary: string;
  status: string;
  priority: string;
}

// BugTest mirrors testrepo.BugTest — a Test affected by a bug, with its
// consolidated run status, for the bug detail pane.
export interface BugTest {
  key: string;
  project: string;
  summary: string;
  status: string;
  runStatus: string;
}
