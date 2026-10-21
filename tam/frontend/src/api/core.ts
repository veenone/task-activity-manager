// Profiles, settings, diagnostics, and the profile settings other modules read.

import * as App from "../../wailsjs/go/main/App";

export interface Profile {
  id: string;
  name: string;
  jiraUrl: string;
  projectKey: string;
  backend: string;
  createdAt: string;
  // The three the Manage Profiles form edits beyond the four above. Optional
  // for the same reason Issue.pending is: the backend always sends them, but
  // fixtures written before the form existed do not spell them out.
  scopeJql?: string;
  caCert?: string;
  allowUntrustedTls?: boolean;
}

export interface Settings {
  defaultProfileId: string;
  theme: string;
  // Whether the left nav rail is shown. Views are switched from the View
  // menu, so the rail is optional and off unless asked for; older settings
  // rows have no value, which reads as off. Optional here for the fixtures
  // that predate it.
  showNavRail?: boolean;
  // Which version of the onboarding tour this user has been through, 0 or
  // absent when they never have. A version rather than a flag, so a later
  // release can offer a rewritten tour by raising TOUR_VERSION.
  tourSeenVersion?: number;
  // The folder a report export's save dialog starts in. Empty, and absent in
  // fixtures written before it existed, means the app data folder.
  reportExportDir?: string;
  // Whether the profile form checks a project key against Jira's documented
  // shape. Absent means on, both in a fixture and in a settings row written
  // before the switch existed, so a reader tests it against false rather than
  // for truthiness.
  checkProjectKeyPattern?: boolean;
}

// The rituals space and root, and the pair a sprint report is published to.
// Both reports fields empty means the report goes where it always did: the
// rituals space, under the sprint's own page or the rituals root. Optional
// for the fixtures written before they existed.
export interface ConfluenceConfig {
  baseURL: string;
  spaceKey: string;
  rootPageID: string;
  reportsSpaceKey?: string;
  reportsRootPageID?: string;
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

// SETTING_JIRA_USERNAME and SETTING_JIRA_DISPLAY_NAME are the profile
// settings TestProfileConnection and the start of every sync write, and
// what the Assigned to me tab reads to know who "me" is. They match
// settingJiraUsername and settingJiraDisplayName in app_profiles.go and
// internal/syncer/syncer.go.
export const SETTING_JIRA_USERNAME = "jira_username";
export const SETTING_JIRA_DISPLAY_NAME = "jira_display_name";

export const Health: () => Promise<HealthInfo> = App.Health;
export const GetDiagnostics: () => Promise<Diagnostics> = App.GetDiagnostics;
// The log reader takes no path on purpose: it answers with the tail of the
// file TAM itself opened at startup, and nothing the frontend names.
export const ReadLog: () => Promise<string> = App.ReadLog;
export const ExportDiagnostics: () => Promise<string> = App.ExportDiagnostics;
export const ListProfiles: () => Promise<Profile[]> = App.ListProfiles;
export const CreateProfile: (
  name: string,
  jiraUrl: string,
  projectKey: string,
  scopeJql: string,
  token: string,
  caCert: string,
  allowUntrustedTls: boolean,
) => Promise<Profile> = App.CreateProfile;
export const CreateProfileReusingToken: (
  name: string,
  jiraUrl: string,
  projectKey: string,
  scopeJql: string,
  sourceProfileId: string,
) => Promise<Profile> = App.CreateProfileReusingToken;
export const UpdateProfile: (
  id: string,
  name: string,
  jiraUrl: string,
  projectKey: string,
  scopeJql: string,
  token: string,
  caCert: string,
  allowUntrustedTls: boolean,
) => Promise<Profile> = App.UpdateProfile;
export const DeleteProfile: (id: string) => Promise<void> = App.DeleteProfile;
export const TestConnection: (
  jiraUrl: string,
  token: string,
  caCert: string,
  allowUntrustedTls: boolean,
) => Promise<string> = App.TestConnection;
export const TestProfileConnection: (
  profileId: string,
  jiraUrl: string,
  caCert: string,
  allowUntrustedTls: boolean,
) => Promise<string> = App.TestProfileConnection;
// ExportProfile resolves to the path written, or "" when the save dialog was
// cancelled. ImportProfile resolves to a profile with an empty id on cancel.
export const ExportProfile: (id: string) => Promise<string> = App.ExportProfile;
export const ImportProfile: () => Promise<Profile> = App.ImportProfile;
export const GetSettings: () => Promise<Settings> = App.GetSettings;
export const SetTheme: (theme: string) => Promise<void> = App.SetTheme;
export const SetDefaultProfile: (id: string) => Promise<void> =
  App.SetDefaultProfile;
export const SetNavRailVisible: (visible: boolean) => Promise<void> =
  App.SetNavRailVisible;
// SetTourSeenVersion records that the onboarding tour has been seen, so it
// is offered once rather than at every launch.
export const SetTourSeenVersion: (version: number) => Promise<void> = App.SetTourSeenVersion;
// SetReportExportDirectory rejects a path that is not a folder; "" clears the
// setting. ChooseReportExportDirectory resolves to "" when the picker was
// closed without choosing.
export const SetReportExportDirectory: (dir: string) => Promise<void> =
  App.SetReportExportDirectory;
export const ChooseReportExportDirectory: () => Promise<string> =
  App.ChooseReportExportDirectory;
// SetProjectKeyPatternCheck turns the profile form's project key pattern check
// on and off. It is on for anyone who has never set it.
export const SetProjectKeyPatternCheck: (on: boolean) => Promise<void> =
  App.SetProjectKeyPatternCheck;

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
