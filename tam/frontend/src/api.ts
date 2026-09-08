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

export type IssueType = "task" | "epic" | "story" | "bug" | "requirement" | "subtask";

// ISSUE_TYPES is the six logical types in display order, with the chip
// label the grid and filter bar use. "Sub-task" is what TAM calls the level;
// what the instance calls it is discovered per project (GetSubtaskTypeName),
// so the label here is the concept, not the Jira type name.
export const ISSUE_TYPES: { id: IssueType; label: string; short: string }[] = [
  { id: "task", label: "Task", short: "Task" },
  { id: "epic", label: "Epic", short: "Epic" },
  { id: "story", label: "Story", short: "Story" },
  { id: "bug", label: "Bug", short: "Bug" },
  { id: "requirement", label: "Requirement", short: "Req" },
  { id: "subtask", label: "Sub-task", short: "Sub" },
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
  pending?: boolean;
  pendingId?: number;
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
  // "" is the store's default rank order. Sorting is a backend concern
  // because the grid is paged: the frontend holds one page, so sorting here
  // would order 25 rows out of a project's thousands.
  sort: SortColumn | "";
  desc: boolean;
}

// SortColumn is the set issuerepo.SortColumns accepts. A value outside it
// falls back to rank order rather than erroring, but the union keeps the
// header list and the store's whitelist from drifting apart.
export type SortColumn =
  | "key"
  | "type"
  | "summary"
  | "status"
  | "assignee"
  | "sprint"
  | "storyPoints";

// GRID_COLUMNS is the Backlog's seven columns in display order: the header
// label, whether the column can be sorted by, and which way a first click
// takes it. Text reads best ascending; a number or a status the user is
// hunting for reads best with the largest first.
export const GRID_COLUMNS: {
  id: SortColumn;
  label: string;
  firstClickDesc: boolean;
}[] = [
  { id: "key", label: "Key", firstClickDesc: false },
  { id: "type", label: "Type", firstClickDesc: false },
  { id: "summary", label: "Summary", firstClickDesc: false },
  { id: "status", label: "Status", firstClickDesc: false },
  { id: "assignee", label: "Assignee", firstClickDesc: false },
  { id: "sprint", label: "Sprint", firstClickDesc: true },
  { id: "storyPoints", label: "Pts", firstClickDesc: true },
];

export interface IssuePage {
  issues: Issue[];
  total: number;
}

export interface TreeQuery {
  text: string;
  sprintId: string;
  showDone: boolean;
}

export interface EpicNode {
  issue: Issue;
  children: Issue[];
  total: number;
  done: number;
  points: number;
  donePoints: number;
}

// EpicTreeData is named to avoid clashing with the EpicTree component;
// truncated mirrors issuerepo.Tree.truncated, set when the cache holds more
// issues than the tree will render.
export interface EpicTreeData {
  epics: EpicNode[];
  orphans: Issue[];
  truncated: boolean;
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
  // boards is the boards pass's own summary, absent when the pass did not
  // run at all (syncer.Summary marks it omitempty). It is the only way a
  // dropped board, or a Jira with no Agile API, reaches the user from an
  // automatic sync, which is the sync most users ever run.
  boards?: BoardSummary;
}

// Board, Sprint, ColumnView, LaneView, and BoardView mirror the Go shapes in
// internal/boardrepo field for field. A type that quietly omits a field the
// view needs is how a shape drifts from its Go original.
export interface Board {
  id: number;
  name: string;
  // Jira's own board type: "scrum" or "kanban".
  type: string;
}

export interface Sprint {
  id: number;
  boardId: number;
  name: string;
  // Jira's own lowercase value: active, future, or closed.
  state: string;
  startDate: string;
  endDate: string;
}

export interface ColumnView {
  name: string;
  // statusIds are the statuses the column collects, in the board's own
  // order. A drop journals the first of them, and a column with none
  // collects nothing and cannot be a drop target.
  statusIds: string[];
  total: number;
  points: number;
}

export interface LaneView {
  // id is the raw grouping value (the assignee, the epic key, empty for the
  // catch-all lane); label is what the view prints.
  id: string;
  label: string;
  // count is every card in the lane, whether it was rendered or not.
  count: number;
  // cells holds one card list per column, and overflow is parallel to it:
  // overflow[c] is how many cards cell c holds beyond the ones it rendered.
  cells: Issue[][];
  overflow: number[];
}

export interface BoardView {
  boardId: number;
  sprintId: string;
  swimlane: string;
  columns: ColumnView[];
  lanes: LaneView[];
  // donePoints is the story points of the mapped cards whose status is
  // done, summed over every card the board holds rather than the ones a
  // capped cell drew.
  donePoints: number;
  unmapped: number;
  unmappedStatuses: string[];
  notSynced: number;
  capped: boolean;
  needsStatusSync: boolean;
}

export interface BoardSummary {
  boards: number;
  columns: number;
  sprints: number;
  cards: number;
  dropped: string[];
  unavailable: boolean;
  elapsed: string;
}

export type Swimlane = "none" | "assignee" | "epic";

// SWIMLANES is the grouping the board offers, in picker order. The ids match
// boardrepo's SwimlaneNone, SwimlaneAssignee, and SwimlaneEpic.
export const SWIMLANES: { id: Swimlane; label: string }[] = [
  { id: "none", label: "None" },
  { id: "assignee", label: "Assignee" },
  { id: "epic", label: "Epic" },
];

// SETTING_BOARDS_UNAVAILABLE is the profile setting the boards sync writes
// when the instance answered with no Agile API at all. It matches
// syncer.settingBoardsUnavailable.
export const SETTING_BOARDS_UNAVAILABLE = "boards_unavailable";

export interface LinkedTest {
  key: string;
  summary: string;
  linkType: string;
}

// DRAFT_PREFIX starts the temporary key of an issue created locally and
// not yet committed. It matches issuerepo.DraftPrefix.
export const DRAFT_PREFIX = "TAM-NEW-";

export type EditableField = "summary" | "description" | "priority" | "labels" | "storyPoints" | "assignee" | "parentKey";

export const EDITABLE_FIELDS: { id: EditableField; label: string }[] = [
  { id: "summary", label: "Summary" },
  { id: "description", label: "Description" },
  { id: "priority", label: "Priority" },
  { id: "labels", label: "Labels" },
  { id: "storyPoints", label: "Story points" },
  { id: "assignee", label: "Assignee" },
  { id: "parentKey", label: "Epic" },
];

// The three journal entity types a board move writes, mirroring
// issuerepo.EntityTransition, EntitySprintMove, and EntityRank. Both
// dialogs that show pending work branch on them, and the board reads them
// to know which of its cards carry a pending move.
export const ENTITY_TRANSITION = "issue_transition";
export const ENTITY_SPRINT_MOVE = "issue_sprint";
export const ENTITY_RANK = "issue_rank";
export const MOVE_ENTITIES: string[] = [ENTITY_TRANSITION, ENTITY_SPRINT_MOVE, ENTITY_RANK];

// isMoveEntity says whether a journal row or an audit entry is a board
// move rather than an edit, a link, or a create.
export function isMoveEntity(entityType: string): boolean {
  return MOVE_ENTITIES.includes(entityType);
}

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
  // The epic the draft is created under, "" for none and always "" for an
  // epic. The Go draft has carried this since Phase 2 and both the Jira
  // backend and the repository validate it; only the form was missing it, so
  // a story could not be born under its epic.
  parentKey: string;
  extra: Record<string, string>;
}

// JiraUser is one person the assignee picker can offer. name is the username
// the write path sends; displayName is what the reader sees.
export interface JiraUser {
  name: string;
  displayName: string;
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

export interface LinkType {
  name: string;
  inward: string;
  outward: string;
}

export interface LinkDraft {
  type: string;
  direction: "outward" | "inward";
  toKey: string;
  toSummary: string;
  toType: string;
}

// CommitFailure is one push that did not land. entityType and rowId name
// the journal row it was, which is what an Undo on a board failure
// discards; retryable is false for the ones that will fail identically
// forever, and reachable carries the statuses a refused transition could
// have reached instead. The four are optional for the reason Issue.pending
// is: the backend always sends them, but fixtures written before the board
// pass existed do not spell them out.
export interface CommitFailure {
  key: string;
  error: string;
  entityType?: string;
  rowId?: number;
  retryable?: boolean;
  reachable?: string[];
}

// CommitMove is one board write a Commit settled: a card transitioned,
// moved to a sprint, or ranked. target is named rather than numbered, and
// side says which side of the neighbour a rank went; satisfied marks a row
// Jira already agreed with.
export interface CommitMove {
  key: string;
  entityType: string;
  target: string;
  side: string;
  satisfied: boolean;
}

export interface CommitResult {
  committed: string[];
  created: { tempKey: string; key: string }[];
  linked: { key: string; toKey: string; type: string }[];
  // moved is optional for the same reason CommitFailure's fields are.
  moved?: CommitMove[];
  conflicts: Conflict[];
  failures: CommitFailure[];
  remaining: number;
}

// TransitionCheck is what CanTransition answers with. It is best effort: an
// error means the check could not be made, never that the move is illegal.
export interface TransitionCheck {
  reachable: string[];
  allowed: boolean;
}

export interface ImportPreview {
  headers: string[];
  rowCount: number;
  sample: string[];
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
export const GetEpicTree = (profileId: string, q: TreeQuery): Promise<EpicTreeData> =>
  App.GetEpicTree(profileId, issuerepo.TreeQuery.createFrom(q)) as Promise<EpicTreeData>;
export const ListEpics: (profileId: string) => Promise<Issue[]> = App.ListEpics as (profileId: string) => Promise<Issue[]>;
export const GetProfileSetting: (profileId: string, key: string) => Promise<string> =
  App.GetProfileSetting;

// The board bindings. GetBoard takes its arguments plainly: all four are
// scalars, so nothing has to go through a generated class's createFrom. The
// view is cast for the same reason ListIssues is: the generated cards type
// their issue type as a plain string.
export const ListBoards: (profileId: string) => Promise<Board[]> = App.ListBoards;
export const ListBoardSprints: (profileId: string, boardId: number) => Promise<Sprint[]> =
  App.ListBoardSprints;
export const GetBoard = (
  profileId: string,
  boardId: number,
  sprintId: string,
  swimlane: string,
): Promise<BoardView> =>
  App.GetBoard(profileId, boardId, sprintId, swimlane) as Promise<BoardView>;
export const SyncBoards: (profileId: string) => Promise<BoardSummary> = App.SyncBoards;

// The three board writes. Each one journals and moves the card locally;
// none of them touches Jira, which Commit does. CanTransition is the one
// board binding that reads Jira, and only to check a drop.
export const MoveIssueToColumn: (profileId: string, key: string, statusId: string) => Promise<void> =
  App.MoveIssueToColumn;
export const MoveIssueToSprint: (profileId: string, key: string, sprintId: string) => Promise<void> =
  App.MoveIssueToSprint;
export const RankIssue: (
  profileId: string,
  key: string,
  neighbourKey: string,
  before: boolean,
  boardId: number,
) => Promise<void> = App.RankIssue;
export const CanTransition: (profileId: string, key: string, statusId: string) => Promise<TransitionCheck> =
  App.CanTransition;
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

export const SearchUsers: (profileId: string, query: string) => Promise<JiraUser[]> =
  App.SearchUsers;
export const ListPriorities: (profileId: string) => Promise<string[]> = App.ListPriorities;
// The Jira name of this profile's sub-task type, "" when the project has
// none. TAM's own word for the level is "Sub-task"; the instance may call it
// anything, and this is how the forms say which.
export const GetSubtaskTypeName: (profileId: string) => Promise<string> =
  App.GetSubtaskTypeName;

export const GetLinkTypes: (profileId: string) => Promise<LinkType[]> = App.GetLinkTypes;
// LookupIssue is cast the same way ListIssues is above: the generated
// binding types the issue type as a plain string, narrowed to IssueType here.
export const LookupIssue = (profileId: string, key: string): Promise<Issue> =>
  App.LookupIssue(profileId, key) as Promise<Issue>;
export const AddLink = (profileId: string, key: string, link: LinkDraft): Promise<void> =>
  App.AddLink(profileId, key, backend.LinkDraft.createFrom(link));

// isDemoUrl mirrors suiteprofiles.IsDemoURL in the backend: "demo" on its own
// or a "demo:" / "demo-" variant selects the offline dataset.
export function isDemoUrl(url?: string): boolean {
  const u = (url ?? "").trim().toLowerCase();
  return u === "demo" || u.startsWith("demo:") || u.startsWith("demo-");
}
