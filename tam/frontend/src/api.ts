// api.ts is the frontend's typed access to the Go backend. It re-exports the
// generated bindings and defines plain shapes for what they return, so state
// and test fixtures can be object literals.
//
// The bindings are re-exported through those shapes rather than raw: the
// generated wailsjs models are classes carrying every column of the shared
// Go struct, so a raw re-export would force fixtures to spell out fields TAM
// never reads. Arguments that are Go structs go through the generated
// class's createFrom so the binding receives the shape it declares.

import * as App from "../wailsjs/go/main/App";
import { backend, importer, issuerepo } from "../wailsjs/go/models";

export { EventsOn, BrowserOpenURL } from "../wailsjs/runtime/runtime";
export type { SyncProgress } from "@agile-suite/core";

export interface Profile {
  id: string;
  name: string;
  jiraUrl: string;
  projectKey: string;
  backend: string;
  createdAt: string;
}

export interface Settings {
  defaultProfileId: string;
  theme: string;
}

export interface HealthInfo {
  ok: boolean;
  error: string;
  dbPath: string;
  sharedPath: string;
  logPath: string;
}

export interface Diagnostics {
  version: string;
  dbPath: string;
  sharedPath: string;
  logPath: string;
  os: string;
  arch: string;
  goVersion: string;
  schemaVersion: number;
  profileCount: number;
  startupError: string;
}

export type IssueType = "task" | "epic" | "story" | "bug" | "requirement";

// ISSUE_TYPES is the five logical types in display order, with the chip
// label the grid and filter bar use.
export const ISSUE_TYPES: { id: IssueType; label: string; short: string }[] = [
  { id: "task", label: "Task", short: "Task" },
  { id: "epic", label: "Epic", short: "Epic" },
  { id: "story", label: "Story", short: "Story" },
  { id: "bug", label: "Bug", short: "Bug" },
  { id: "requirement", label: "Requirement", short: "Req" },
];

export interface Issue {
  key: string;
  id: string;
  project: string;
  type: IssueType | "";
  summary: string;
  status: string;
  assignee: string;
  reporter: string;
  priority: string;
  labels: string[];
  sprintId: string;
  sprintName: string;
  parentKey: string;
  storyPoints?: number | null;
  rank: string;
  created: string;
  updated: string;
  // Computed by the store on every read. Optional so fixtures that predate
  // plan 1b still type-check; the backend always sends both.
  pending?: boolean;
  draft?: boolean;
}

export interface Link {
  direction: "inward" | "outward" | string;
  type: string;
  key: string;
  summary: string;
  issueType: string;
}

export interface IssueDetail {
  key: string;
  description: string;
  links: Link[];
  fields: Record<string, unknown>;
}

export interface IssueQuery {
  text: string;
  types: string[];
  sprintId: string;
  offset: number;
  limit: number;
}

export interface IssuePage {
  issues: Issue[];
  total: number;
}

export interface SprintRef {
  id: string;
  name: string;
}

export interface SyncState {
  lastSynced: string;
  lastFull: string;
  lastError: string;
  issueCount: number;
}

export interface SyncSummary {
  fetched: number;
  upserted: number;
  skipped: number;
  full: boolean;
  elapsed: string;
}

export interface LinkedTest {
  key: string;
  summary: string;
  linkType: string;
}

// DRAFT_PREFIX starts the temporary key of an issue created locally and
// not yet committed. It matches issuerepo.DraftPrefix.
export const DRAFT_PREFIX = "TAM-NEW-";

export type EditableField = "summary" | "description" | "priority" | "labels" | "storyPoints" | "assignee";

export const EDITABLE_FIELDS: { id: EditableField; label: string }[] = [
  { id: "summary", label: "Summary" },
  { id: "description", label: "Description" },
  { id: "priority", label: "Priority" },
  { id: "labels", label: "Labels" },
  { id: "storyPoints", label: "Story points" },
  { id: "assignee", label: "Assignee" },
];

export function fieldLabel(field: string): string {
  return EDITABLE_FIELDS.find((f) => f.id === field)?.label ?? field;
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

export interface IssueDraft {
  type: IssueType;
  summary: string;
  description: string;
  priority: string;
  labels: string[];
  assignee: string;
  storyPoints: number | null;
  extra: Record<string, string>;
}

export interface FieldOption {
  id: string;
  value: string;
}

export interface FieldSpec {
  id: string;
  name: string;
  type: "string" | "option" | "number" | "date" | "array" | string;
  required: boolean;
  allowedValues: FieldOption[];
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

export interface CommitResult {
  committed: string[];
  created: { tempKey: string; key: string }[];
  conflicts: Conflict[];
  failures: { key: string; error: string }[];
  remaining: number;
}

export interface ImportPreview {
  headers: string[];
  rowCount: number;
}

export interface ImportMapping {
  type: string;
  summary: string;
  description: string;
  priority: string;
  labels: string;
  assignee: string;
  storyPoints: string;
  parentKey: string;
}

export interface ImportRowError {
  row: number;
  message: string;
}

export interface ImportResult {
  rows: number;
  created: string[];
  errors: ImportRowError[];
}

// IMPORT_FIELDS are the draft fields a column can feed, in dialog order.
export const IMPORT_FIELDS: { id: keyof ImportMapping; label: string }[] = [
  { id: "type", label: "Type" },
  { id: "summary", label: "Summary" },
  { id: "description", label: "Description" },
  { id: "priority", label: "Priority" },
  { id: "labels", label: "Labels" },
  { id: "assignee", label: "Assignee" },
  { id: "storyPoints", label: "Story points" },
  { id: "parentKey", label: "Parent key" },
];

// readFileAsBase64 reads a browser File into the base64 the import
// bindings take (the data URL's payload, after the comma).
export function readFileAsBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(reader.error ?? new Error("The file could not be read."));
    reader.onload = () => {
      const url = String(reader.result ?? "");
      resolve(url.slice(url.indexOf(",") + 1));
    };
    reader.readAsDataURL(file);
  });
}

export const Health: () => Promise<HealthInfo> = App.Health;
export const GetDiagnostics: () => Promise<Diagnostics> = App.GetDiagnostics;
export const ListProfiles: () => Promise<Profile[]> = App.ListProfiles;
export const CreateProfile: (
  name: string,
  jiraUrl: string,
  projectKey: string,
  token: string,
  makeDefault: boolean,
) => Promise<Profile> = App.CreateProfile;
export const DeleteProfile: (id: string) => Promise<void> = App.DeleteProfile;
export const GetSettings: () => Promise<Settings> = App.GetSettings;
export const SetTheme: (theme: string) => Promise<void> = App.SetTheme;
export const SetDefaultProfile: (id: string) => Promise<void> =
  App.SetDefaultProfile;

export const SyncIssues: (profileId: string, full: boolean) => Promise<SyncSummary> =
  App.SyncIssues;
export const GetSyncState: (profileId: string) => Promise<SyncState> = App.GetSyncState;
// The generated page types the issue type as a plain string, because Go has
// no string unions. The narrowing to IssueType happens here, at the one
// boundary: the backend's logicalType already maps every issue to one of the
// five or to "".
export const ListIssues = (profileId: string, q: IssueQuery): Promise<IssuePage> =>
  App.ListIssues(profileId, issuerepo.IssueQuery.createFrom(q)) as Promise<IssuePage>;
export const GetIssueDetail: (profileId: string, key: string) => Promise<IssueDetail> =
  App.GetIssueDetail;
export const ListLinkedTests: (profileId: string, key: string) => Promise<LinkedTest[]> =
  App.ListLinkedTests;
export const ListSprints: (profileId: string) => Promise<SprintRef[]> = App.ListSprints;
export const GetProfileSetting: (profileId: string, key: string) => Promise<string> =
  App.GetProfileSetting;
export const SetProfileSetting: (
  profileId: string,
  key: string,
  value: string,
) => Promise<void> = App.SetProfileSetting;

export const EditIssue: (profileId: string, key: string, field: string, value: string) => Promise<void> =
  App.EditIssue;
export const CreateIssue = (profileId: string, draft: IssueDraft): Promise<string> =>
  App.CreateIssue(profileId, backend.IssueDraft.createFrom(draft));
export const GetCreateFields: (profileId: string, typeName: string) => Promise<FieldSpec[]> =
  App.GetCreateFields;
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

export const PreviewImport: (contentB64: string, isXlsx: boolean) => Promise<ImportPreview> = App.PreviewImport;
export const AutoMapImport: (headers: string[]) => Promise<ImportMapping> = App.AutoMapImport;
export const ImportIssues = (
  profileId: string,
  contentB64: string,
  isXlsx: boolean,
  fileName: string,
  mapping: ImportMapping,
  dryRun: boolean,
): Promise<ImportResult> =>
  App.ImportIssues(profileId, contentB64, isXlsx, fileName, importer.Mapping.createFrom(mapping), dryRun) as Promise<ImportResult>;
export const SaveImportTemplate: () => Promise<string> = App.SaveImportTemplate;

// isDemoUrl mirrors suiteprofiles.IsDemoURL in the backend: "demo" on its own
// or a "demo:" / "demo-" variant selects the offline dataset.
export function isDemoUrl(url?: string): boolean {
  const u = (url ?? "").trim().toLowerCase();
  return u === "demo" || u.startsWith("demo:") || u.startsWith("demo-");
}
