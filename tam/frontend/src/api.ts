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
import { issuerepo } from "../wailsjs/go/models";

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

// isDemoUrl mirrors suiteprofiles.IsDemoURL in the backend: "demo" on its own
// or a "demo:" / "demo-" variant selects the offline dataset.
export function isDemoUrl(url?: string): boolean {
  const u = (url ?? "").trim().toLowerCase();
  return u === "demo" || u.startsWith("demo:") || u.startsWith("demo-");
}
