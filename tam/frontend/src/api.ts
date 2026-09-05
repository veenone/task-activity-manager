// api.ts is the frontend's typed access to the Go backend. It re-exports the
// generated bindings and defines plain shapes for what they return, so state
// and test fixtures can be object literals.
//
// The bindings are re-exported through those shapes rather than raw: the
// generated wailsjs models are classes carrying every column of the shared
// Go struct, so a raw re-export would force fixtures to spell out fields TAM
// never reads.

import * as App from "../wailsjs/go/main/App";

export { EventsOn, BrowserOpenURL } from "../wailsjs/runtime/runtime";

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

// isDemoUrl mirrors suiteprofiles.IsDemoURL in the backend: "demo" on its own
// or a "demo:" / "demo-" variant selects the offline dataset.
export function isDemoUrl(url?: string): boolean {
  const u = (url ?? "").trim().toLowerCase();
  return u === "demo" || u.startsWith("demo:") || u.startsWith("demo-");
}
