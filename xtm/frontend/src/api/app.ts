// Profiles, connections, settings, capabilities, and app-wide helpers.

import type { Precondition } from "./tests";

// TypeConversion is a plain-shape mirror of testrepo.TypeConversion returned
// by ChangeTestType. We define it here as an interface (consistent with how
// TestCase, Step etc. are defined) rather than re-exporting the generated
// class, which carries a constructor/convertValues that doesn't type-check
// cleanly against plain object literals.
export interface TypeConversion {
  oldType: string;
  newType: string;
  prefilled: boolean;
  canPrefill: boolean;
}

// CrossProjectTest is a cross-project Test search result for the link pickers
// (RND_P_4TFINT_05-322).
export interface CrossProjectTest {
  key: string;
  summary: string;
  status: string;
  projectKey: string;
}

// One page of cross-project browse/search results, plus the total match count.
export interface CrossProjectTestPage {
  tests: CrossProjectTest[];
  total: number;
}

export interface CrossProjectPreconditionPage {
  preconditions: Precondition[];
  total: number;
}

export interface HealthInfo {
  ok: boolean;
  error: string;
  dbPath: string;
  logPath: string;
}

// Settings mirrors settings.Settings — global app preferences (FR-12.2).
export interface Settings {
  defaultProfileId: string;
  theme: string; // "light" | "dark" | "system" | "" (= light)
  requirementLinkType: string; // Jira issue-link type for Test->Requirement coverage; default "tested by"
  showCoverage: boolean; // reveal the opt-in, hidden-by-default Coverage tab
  // Version of the onboarding tour this user has already been through, 0 when
  // never. A later release re-offers a rewritten tour by bumping TOUR_VERSION.
  tourSeenVersion: number;
  // showNavRail is the left navigation rail's visibility. One field shared
  // with TAM: turning the rail on in one app turns it on in the other.
  showNavRail: boolean;
}

// Capabilities mirrors backend.Capabilities — what the active profile's
// backend supports. Xray reports the full/permissive set today (see
// xray.Adapter.Capabilities); this is used to gate backend-specific UI once a
// non-Xray backend exists. Field names/casing match the generated
// wailsjs/go/models.ts backend.Capabilities shape (camelCase, driven by the
// json tags on the Go struct), matching the hand-defined-interface convention
// used by the other domain types in this file.
export interface Capabilities {
  name: string;
  idStyle: string;
  supportsJqlScope: boolean;
  stepModel: string;
  supportsTestTypes: boolean;
  // Can REPORT folders. Does not imply they can be edited: Kiwi surfaces
  // its per-product categories as folders it can read but not reshape.
  supportsFolders: boolean;
  // Can create / rename / delete folders and move tests between them.
  supportsFolderWrites: boolean;
  supportsPreconditionObjects: boolean;
  supportsRequirementObjects: boolean;
  supportsIssueLinkTypes: boolean;
  supportsEnvironments: boolean;
  supportsContainers: boolean;
  containerKinds: string[];
  supportsTestRuns: boolean;
  // The result vocabulary this backend accepts, in the order to offer it in.
  // Empty means the backend declares none, and Xray's six are offered.
  runStatuses: string[];
  statusModel: string;
  supportsWorkflowTransitions: boolean;
  supportsBugCreation: boolean;
  supportsBugLinks: boolean;
  // Reports that this profile files its defects into a Jira project via a
  // configured bug connection, even though the backend itself (e.g. Kiwi)
  // cannot create the Jira issue directly -- something else does it on the
  // backend's behalf. Kept distinct from supportsBugCreation so gates that
  // mean "this backend can create a Jira issue itself" don't accidentally
  // widen to cover routed creation, and vice versa.
  supportsBugRouting: boolean;
  supportsTags: boolean;
  // Create, edit and delete project components (Jira only).
  supportsComponentAdmin: boolean;
  // The backend models parameterized tests as a dataset GetTestDataset can
  // read. Means the backend HAS the concept, not that this instance permits
  // it: an Xray below 5.1 has no dataset endpoint and reads as no dataset.
  supportsDatasets: boolean;
  // The backend can create and replace a dataset, not only read one. Separate
  // from supportsDatasets so a read-only backend still shows the table while
  // the editor stays away.
  supportsDatasetWrites: boolean;
  supportsParameterizedRunDetail: boolean;
  supportsParameterizedRunWrites: boolean;
}

// ProjectComponent mirrors backend.Component: one Jira project component.
export interface ProjectComponent {
  id: string;
  name: string;
  description: string;
  leadName: string;
  leadDisplayName: string;
  assigneeType: string;
}

// ComponentInput mirrors backend.ComponentInput, what a create or update sends.
export interface ComponentInput {
  name: string;
  description: string;
  leadUserName: string;
  assigneeType: string;
}

// ComponentBatchRow mirrors componentadmin.BatchRow: one row of a batch
// create, cleaned, with why it cannot be created ("" when it can).
export interface ComponentBatchRow {
  input: ComponentInput;
  problem: string;
}

// ComponentBatchResult mirrors componentadmin.BatchResult. A failure's row
// counts from 1 within the rows sent.
export interface ComponentBatchResult {
  created: ProjectComponent[];
  failed: { row: number; name: string; error: string }[];
}

// JiraUser mirrors backend.User, a user search result.
export interface JiraUser {
  name: string;
  displayName: string;
  emailAddress: string;
}

export interface Profile {
  id: string;
  name: string;
  jiraUrl: string;
  projectKey: string;
  scopeJql: string;
  // crossProjectSources is a comma-separated list of source project keys this
  // profile may link preconditions, test calls, and cloned steps from
  // (RND_P_4TFINT_05-322). Empty disables cross-project linking.
  crossProjectSources: string;
  bugIssueType: string;
  bugProjectMode: string; // "test" | "execution" | "dedicated"
  bugProjectKey: string;
  caCert: string;
  allowUntrustedTls: boolean;
  // backend selects which system this profile connects to: "xray" (default,
  // Jira Data Center + Xray Server/DC) or "kiwi" (Kiwi TCMS). Settable from
  // the profile form's backend selector (P6.1b).
  backend: string;
  createdAt: string;
}

// Connection mirrors connection.Connection — one backend a workspace talks
// to (P6.3 bridge plumbing). A single-connection workspace's connection has
// id == workspaceId and role "both"; the bridge (B5/B6, not yet built) adds
// a second connection with role "source" or "target".
export interface Connection {
  id: string;
  workspaceId: string;
  name: string;
  backend: string; // "xray" | "kiwi"
  url: string;
  projectKey: string;
  scopeJql: string;
  bugIssueType: string;
  bugProjectMode: string; // "test" | "execution" | "dedicated"
  bugProjectKey: string;
  caCert: string;
  allowUntrustedTls: boolean;
  role: string; // "source" | "target" | "both"
  createdAt: string;
}

// BridgeGap mirrors bridge.Gap — one way the target connection's backend
// can't fully represent something the source connection's backend supports,
// returned by ComputeBridgeGap (Phase 6 bridge task B4). Feature is a stable
// machine key; Severity is "blocking" | "lossy" | "info".
export interface BridgeGap {
  feature: string;
  severity: string;
  message: string;
}

// BridgeMapping mirrors bridge.Mapping — the reversible status/step/field
// mapping used when publishing from a source connection to a target
// connection. Returned by GetBridgeMapping (the saved mapping, or a
// bridge.DefaultMapping when none is saved) and persisted by
// SaveBridgeMapping. The publish engine that applies it (B5) and the mapping
// editor UI (B6) are later tasks — B4 only computes/persists this shape.
export interface BridgeMapping {
  statusMap: Record<string, string>;
  stepMode: string; // "flatten" | "passthrough"
  fieldMap: Record<string, string>;
  unmappedPolicy: string; // "drop" | "keepInHub"
}

// BridgePublishResult mirrors bridge.PublishResult — the outcome of
// PublishToTarget (Phase 6 bridge task B5): every hub test newly created in
// the target this run, every one skipped because it was already published
// (resumability), and every one whose publish attempt failed. Containers/
// preconditions/requirements/links are not published by this call (B5b).
export interface BridgePublishResult {
  created: BridgePublishedTest[];
  alreadyPublished: string[];
  failed: BridgePublishFailure[];
}

export interface BridgePublishedTest {
  localKey: string;
  targetKey: string;
}

export interface BridgePublishFailure {
  localKey: string;
  error: string;
  // targetKey is set when CreateTest succeeded but a downstream step/status
  // write failed: the target test WAS created (with this key) and its
  // external_ref IS recorded, so a future PublishTests run will SKIP it
  // (resumability) rather than retry the failed steps/status. Empty means
  // CreateTest itself failed — nothing was created, and a retry will
  // correctly re-attempt CreateTest for this test.
  targetKey?: string;
}

// Diagnostics mirrors app.Diagnostics — the environment + state summary shown
// in the diagnostics view (FR-12.4).
export interface Diagnostics {
  version: string;
  dbPath: string;
  logPath: string;
  os: string;
  arch: string;
  goVersion: string;
  schemaVersion: number;
  profileCount: number;
  startupError: string;
}

// isDemoUrl reports whether a profile's Jira URL selects demo mode: "demo", a
// "demo:" / "mock:" prefix, or a "demo-" variant like "demo-pkcs" that picks a
// built-in dataset. Also matches "kiwi-demo" (and its "kiwi-demo:"/"kiwi-demo-"
// variants) — the offline Kiwi demo (internal/backend/kiwi/demo.go), so the
// DEMO chip shows for a kiwi-demo profile too. The single source of truth for
// the frontend — keep in sync with isDemoURL in the Go backend
// (internal/jira/demo.go) and the validation in ProfileForm.tsx.
export function isDemoUrl(url?: string): boolean {
  return /^(demo|demo[-:].*|mock:.*|kiwi-demo|kiwi-demo[-:].*)$/i.test(
    (url ?? "").trim(),
  );
}

// isKiwiDemoUrl reports whether a profile's Jira URL selects the offline Kiwi
// demo specifically (as opposed to the Xray demo). Kept separate from
// demoVariant, whose return type is pinned to the Xray-demo theme names
// ("pkcs" | "euicc" | "") consumed by CoverageMap/CoverageView.
export function isKiwiDemoUrl(url?: string): boolean {
  return /^(kiwi-demo|kiwi-demo[-:].*)$/i.test((url ?? "").trim());
}

// demoVariant returns the named demo dataset variant embedded in a profile's
// Jira URL. Returns "pkcs" for demo-pkcs (or demo-pkcs:...), "euicc" for
// demo-euicc (or demo-euicc:...), or "" for plain demo / non-demo profiles.
// Mirrors demoVariant in the Go backend (internal/jira/demo_theme.go).
export function demoVariant(url?: string): "pkcs" | "euicc" | "" {
  const s = (url ?? "").trim().toLowerCase();
  if (s === "demo-pkcs" || s.startsWith("demo-pkcs:")) return "pkcs";
  if (s === "demo-euicc" || s.startsWith("demo-euicc:")) return "euicc";
  return "";
}
