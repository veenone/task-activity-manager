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
import { backend, importer, issuerepo, profile, reportout } from "../wailsjs/go/models";

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

export interface ConfluenceConfig { baseURL: string; spaceKey: string; rootPageID: string }

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

// DraftType is a logical type or an instance's own type name. The string
// half keeps the six autocompleting where one is meant.
export type DraftType = IssueType | (string & {});

// ProjectType is one issue type a project offers, under the project's own
// name for it. logical is TAM's type for it, "" when TAM has none, which is
// the state the dialog carries rather than rounding to "task".
export interface ProjectType {
  id: string;
  name: string;
  subtask: boolean;
  // TAM's logical type for this one, "" when TAM has none, which is the
  // state the dialog carries rather than rounding to "task". Not narrowed
  // to IssueType: this crosses the binding, so the value is whatever the Go
  // side sent, and the dialog treats anything it does not recognise as the
  // project's own type rather than trusting the string.
  logical: string;
}

export interface Issue {
  key: string;
  id: string;
  project: string;
  type: IssueType | "";
  summary: string;
  status: string;
  // The status's own Jira id, which is what a board buckets a card into a
  // column by and what lib/unfinished reads. Optional so fixtures written
  // before anything on this side needed it still type-check; the backend
  // has always sent it.
  statusId?: string;
  assignee: string;
  // The Jira username sync, edits, and drafts cache alongside assignee
  // (schema 14). Optional so fixtures written before assigned-to-me still
  // type-check; a row cached before schema 14 sends "" until the next sync.
  assigneeName?: string;
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
  // The description the sync caches on the row (schema 17), so the panel
  // draws it from the local store. null and undefined both mean nothing has
  // read this issue's description yet: a row cached before schema 17, or a
  // fixture written before this field existed. An empty string means Jira
  // says the issue has none. The panel words the two differently, so no
  // reader is told an issue has no description when TAM has never looked.
  description?: string | null;
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

// One comment on an issue. Named IssueComment because Comment is a DOM
// global. author and authorName are both empty for a comment Jira answered
// with no author (anonymous, or a user deleted since); restriction is the
// role or group a restricted comment is limited to, empty for an ordinary
// one. created and updated are RFC 3339 unless Jira sent something that
// could not be read, in which case they are exactly what it sent.
export interface IssueComment {
  id: string;
  author: string;
  authorName: string;
  created: string;
  updated: string;
  body: string;
  restriction: string;
}

export interface IssueDetail {
  key: string;
  description: string;
  links: Link[];
  fields: Record<string, unknown>;
  // Oldest first, so the newest are the last few. commentTotal is how many
  // the issue has, which is more than comments.length once commentsTruncated
  // is true: the read keeps the newest 500, and a comment page that failed
  // leaves the rest unread rather than failing the whole detail.
  comments: IssueComment[];
  commentTotal: number;
  commentsTruncated: boolean;
  // When the store cached this detail, RFC 3339, empty for one that never
  // went through the cache. A detail served while Jira is unreachable is the
  // cached one, and this is what lets the panel say so.
  fetchedAt: string;
}

export interface IssueQuery {
  // Page parent groups and return each group's subtasks beneath its parent.
  groupSubtasks?: boolean;
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
  // Narrows to issues assigned to this Jira username, case-insensitively,
  // with assigneeDisplayName as the fallback for a row cached before schema
  // 14 (see issuerepo.IssueQuery.AssigneeName). Empty leaves the query
  // unfiltered, which is what Backlog passes by leaving both unset.
  assigneeName?: string;
  assigneeDisplayName?: string;
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
  // draft marks a board drafted in TAM and not yet created in Jira, the way
  // Sprint.draft does for a sprint. Absent on fixtures written before board
  // drafts existed.
  draft?: boolean;
}

export interface Sprint {
  id: number;
  boardId: number;
  name: string;
  // Jira's own lowercase value: active, future, or closed.
  state: string;
  startDate: string;
  endDate: string;
  // Absent on older cached sprints and fixtures; Reports then uses endDate.
  completeDate?: string;
  // What the sprint is for. Empty for a sprint cached before schema version
  // 7 added the column, since nothing back-fills it until the next boards
  // refresh, which is why a reader cannot tell an absent goal from one Jira
  // has none set on.
  goal: string;
  // A sprint drafted in TAM and not yet created in Jira: its id is negative,
  // its state is future, and Commit creates it before any card moved into it
  // is sent. Absent on fixtures written before drafts existed.
  draft?: boolean;
}

// SprintChoice mirrors boardrepo.SprintChoice field for field: every open
// sprint across every board the profile has synced, active ones first then
// by start date, with the board's name since a caller without a board of its
// own has no other way to tell two same-named sprints apart.
export interface SprintChoice {
  id: number;
  name: string;
  boardName: string;
  state: string;
  // A sprint drafted in TAM, named so beside its name.
  draft?: boolean;
}

// SprintOption is the smallest shape the Sprint field actually reads. Sprint
// and SprintChoice are both structurally assignable to it with no mapping
// and no adapter, so a caller with either list passes it straight through.
export interface SprintOption {
  id: number;
  name: string;
  // The board this sprint belongs to, given only by the profile-wide list
  // (SprintChoice); a board's own list needs no such disambiguation.
  boardName?: string;
  // A sprint drafted in TAM, named so beside its name.
  draft?: boolean;
}

// SprintChoice mirrors boardrepo.SprintChoice field for field: every open
// sprint across every board the profile has synced, active ones first then
// by start date, with the board's name since a caller without a board of its
// own has no other way to tell two same-named sprints apart.
export interface SprintChoice {
  id: number;
  name: string;
  boardName: string;
  state: string;
  // A sprint drafted in TAM, named so beside its name.
  draft?: boolean;
}

// SprintOption is the smallest shape the Sprint field actually reads. Sprint
// and SprintChoice are both structurally assignable to it with no mapping
// and no adapter, so a caller with either list passes it straight through.
export interface SprintOption {
  id: number;
  name: string;
  // The board this sprint belongs to, given only by the profile-wide list
  // (SprintChoice); a board's own list needs no such disambiguation.
  boardName?: string;
  // A sprint drafted in TAM, named so beside its name.
  draft?: boolean;
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

// SprintSuggestion is what the start dialog opens with, mirroring
// sprints.Suggestion. fromHistory is the field that keeps the dates honest:
// a plausible wrong end date nobody checks is this feature's failure mode,
// so the dialog says whether the length came from the board's own closed
// sprints or from the fortnight default.
export interface SprintSuggestion {
  name: string;
  // start and end are bare days, the shape a date input takes and writes.
  start: string;
  end: string;
  length: number;
  fromHistory: boolean;
}

// SprintCreated is what CreateSprint answers with, mirroring the App-level
// SprintCreated struct: the sprint Jira made, whose id is what the dialog
// switches the board's picker to, and the note beside it when the board's
// own re-read did not land.
export interface SprintCreated {
  sprint: Sprint;
  note: string;
}

// SprintDetail is one node of the Sprints view, mirroring
// boardrepo.SprintDetail field for field: a sprint's own row (the embedded
// Sprint's fields are inlined by Go's encoder), the cards it holds, and the
// four numbers a progress reading is drawn from. The board's own unassigned
// work arrives as one more node of the same shape, with state
// "unassigned" and the name "Board backlog".
export interface SprintDetail extends Sprint {
  // issues is the scope's cards, capped by the one budget the whole call
  // spends across every node together. It can be shorter than total, which
  // is why nothing on screen counts this array.
  issues: Issue[];
  // total, done, points and donePoints are counted over every card the
  // scope holds, whether or not the cap let it into issues.
  total: number;
  done: number;
  points: number;
  donePoints: number;
  // membershipCached says whether the sync fetches this scope's membership
  // at all, which it does for an active or future sprint and never for a
  // closed one. It cannot say whether the last attempt landed; notSynced is
  // the field that says the read came up short.
  membershipCached: boolean;
  // notSynced counts the keys this scope names that the issue cache does
  // not hold, so a sprint of thirty on a board synced before its issues
  // does not quietly report a total of ten.
  notSynced: number;
  // truncated is set when the shared cap stopped this node's issues short.
  // The four numbers above are unaffected: they are counted before the cap.
  truncated: boolean;
}

// The sprint report, as internal/sprintreport hands it over. Every number
// is a float64 in Go, because a board counting story points routinely
// carries halves.

export interface ReportDay {
  // The local calendar date, bucketed in the zone the backend built the
  // series in, as 2006-01-02.
  date: string;
  scope: number;
  completed: number;
  remaining: number;
  ideal: number;
}

export interface ReportSeries {
  sprintId: number;
  sprintName: string;
  // "points" or "cards". unitReason says why when it is cards, and is one
  // of REPORT_UNIT_REASONS; it is empty for points.
  unit: string;
  unitReason: string;
  committed: number;
  added: number;
  removed: number;
  completed: number;
  carriedOver: number;
  // The day by day line behind the totals, which the burndown draws.
  days: ReportDay[];
  // The keys of the issues whose changelog came back cut short, so the
  // numbers above rest in part on a partial history.
  truncated: string[];
}

export interface VelocityRow {
  sprintId: number;
  sprintName: string;
  // The unit rides on the row and not on the table, because a board that
  // moved from points to cards mid-history holds two different quantities.
  unit: string;
  unitReason: string;
  committed: number;
  completed: number;
  truncated: boolean;
}

export interface SprintReport {
  series: ReportSeries;
  velocity: VelocityRow[];
  // When the series was reconstructed, in RFC 3339, and the chosen sprint's
  // stamp only. The velocity rows carry no age of their own and most of
  // them are usually served from the store, so nothing here describes how
  // old the table is.
  builtAt: string;
  // One of REPORT_UNAVAILABLE_REASONS when there is no report, and empty
  // otherwise. When it is set, series and velocity are empty.
  unavailable: string;
}

// The four reasons a report has nothing to show, mirroring the constants in
// internal/sprintreport. Each is a property of the data rather than a
// failure of the call, which is why they travel in the answer.
export const REPORT_UNAVAILABLE_REASONS = [
  "boardNotSynced",
  "sprintNotFound",
  "sprintHasNoDates",
  "noClosedSprint",
] as const;

// The two reasons a series counts cards, mirroring internal/reports.
export const REPORT_UNIT_REASONS = ["nothingEstimated", "noPointsFieldSeen"] as const;

// REPORT_PROGRESS_EVENT carries sprintreport.Progress while a report runs.
// It is not the sync's event: the shell's banner reads that one, and a
// report announcing itself there would say a sync was running.
export const REPORT_PROGRESS_EVENT = "tam:report-progress";

export interface ReportProgress {
  // "sprint" for the sprint the user asked for, "velocity" for one of the
  // older sprints the table needs and the store did not hold.
  phase: string;
  sprintId: number;
  sprintName: string;
  // Issues of this one sprint, not of the whole report.
  fetched: number;
  total: number;
  // The last frame of one sprint's fetch, not of the report.
  done: boolean;
}

// MAX_CARDS_PER_VIEW mirrors boardrepo.MaxCardsPerView, so a line saying
// what a read stopped at names the number the backend actually stopped at.
// Two reads spend it: a board spends it over its cells, and the Sprints
// view's list spends one of these budgets across every sprint together.
export const MAX_CARDS_PER_VIEW = 2000;

// UNASSIGNED_SPRINT_STATE mirrors boardrepo.UnassignedSprintState. It is
// deliberately not one of Jira's three states, so the one node in the list
// that is not a sprint can be told from the ones that are without matching
// on its name.
export const UNASSIGNED_SPRINT_STATE = "unassigned";

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

// SETTING_JIRA_USERNAME and SETTING_JIRA_DISPLAY_NAME are the profile
// settings TestProfileConnection and the start of every sync write, and
// what the Assigned to me tab reads to know who "me" is. They match
// settingJiraUsername and settingJiraDisplayName in app_profiles.go and
// internal/syncer/syncer.go.
export const SETTING_JIRA_USERNAME = "jira_username";
export const SETTING_JIRA_DISPLAY_NAME = "jira_display_name";

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

// The four journal entity types a board move writes, mirroring
// issuerepo.EntityTransition, EntitySprintMove, EntityRank, and
// EntityIssueBoard. Both dialogs that show pending work branch on them, and
// the board reads them to know which of its cards carry a pending move.
export const ENTITY_TRANSITION = "issue_transition";
export const ENTITY_SPRINT_MOVE = "issue_sprint";
export const ENTITY_RANK = "issue_rank";
export const ENTITY_ISSUE_BOARD = "issue_board";
export const MOVE_ENTITIES: string[] = [ENTITY_TRANSITION, ENTITY_SPRINT_MOVE, ENTITY_RANK, ENTITY_ISSUE_BOARD];

// The field each of those rows carries, mirroring issuerepo.FieldStatusID,
// FieldSprintID, and FieldRank. An issue_board row has no field constant
// here: unlike a status or a sprint, an issue can be queued onto more than
// one board, so issuerepo.BoardField folds the board's id into the field
// itself and there is no one fixed string for MOVE_FIELDS to name.
export const FIELD_STATUS_ID = "statusId";
export const FIELD_SPRINT_ID = "sprintId";
export const FIELD_RANK = "rank";

// MOVE_LABELS is the one word each board move goes by, in the Pending
// changes dialog, the Activity tab, and the conflict table. One vocabulary
// for the four moves: a surface that invented its own would have the same
// card read "Status" in one dialog and "statusId" in the next.
export const MOVE_LABELS: Record<string, string> = {
  [ENTITY_TRANSITION]: "Status",
  [ENTITY_SPRINT_MOVE]: "Sprint",
  [ENTITY_RANK]: "Rank",
  [ENTITY_ISSUE_BOARD]: "Board",
};

// MOVE_FIELDS names the entity behind a field, for the surfaces that hold
// only the field: a held board write reaches the conflict card as
// "statusId" or "sprintId" and has to read as Status or Sprint there too.
// An issue_board row is never held (committer.heldBoardRow), so it never
// reaches a surface that has only its field and needs this map.
export const MOVE_FIELDS: Record<string, string> = {
  [FIELD_STATUS_ID]: ENTITY_TRANSITION,
  [FIELD_SPRINT_ID]: ENTITY_SPRINT_MOVE,
  [FIELD_RANK]: ENTITY_RANK,
};

// isMoveEntity says whether a journal row or an audit entry is a board
// move rather than an edit, a link, or a create.
export function isMoveEntity(entityType: string): boolean {
  return MOVE_ENTITIES.includes(entityType);
}

export function fieldLabel(field: string): string {
  return EDITABLE_FIELDS.find((f) => f.id === field)?.label ?? MOVE_LABELS[MOVE_FIELDS[field]] ?? field;
}

// UnpushableEdit mirrors issuerepo.UnpushableEdit: one journalled edit whose
// field is not on the edit screen Jira reports for its issue. Commit would be
// refused for it. It is named where the pending work is shown and never
// discarded on the user's behalf.
export interface UnpushableEdit {
  id: number;
  key: string;
  field: string;
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

// ENTITY_SPRINT_CREATE is the journal entity of a sprint drafted in TAM. Its
// entityKey is the draft's negative id and its afterVal a DraftSprint.
export const ENTITY_SPRINT_CREATE = "sprint_create";

// ENTITY_BOARD_CREATE is the journal entity of a board drafted in TAM,
// mirroring issuerepo.EntityBoardCreate. Its entityKey is the draft's
// negative id and its afterVal a DraftBoard.
export const ENTITY_BOARD_CREATE = "board_create";

// DraftBoard mirrors issuerepo.DraftBoard: what a board_create row carries.
export interface DraftBoard {
  name: string;
  type: string;
  filterName: string;
  jql: string;
}

// ENTITY_SPRINT_EDIT and ENTITY_SPRINT_DELETE are the journal entities of an
// edit and a delete of a sprint Jira holds, mirroring issuerepo's. Their
// entityKey is the sprint id. An edit's afterVal is a SprintEdit and its
// beforeVal the cached name, goal and dates; a delete's afterVal is
// { boardId, name } and its beforeVal the name.
export const ENTITY_SPRINT_EDIT = "sprint_edit";
export const ENTITY_SPRINT_DELETE = "sprint_delete";
// ENTITY_SPRINT_START and ENTITY_SPRINT_COMPLETE are a start and a completion
// waiting for Commit, keyed by the sprint id (a draft's negative id for a
// start). A start's afterVal is a SprintStart, a completion's a
// SprintComplete; neither has a beforeVal.
export const ENTITY_SPRINT_START = "sprint_start";
export const ENTITY_SPRINT_COMPLETE = "sprint_complete";
export const SPRINT_ENTITIES: string[] = [
  ENTITY_SPRINT_CREATE, ENTITY_SPRINT_EDIT, ENTITY_SPRINT_DELETE, ENTITY_SPRINT_START, ENTITY_SPRINT_COMPLETE,
];

// SprintStart mirrors issuerepo.SprintStart: what a sprint_start row carries.
export interface SprintStart {
  boardId: number;
  name: string;
  goal: string;
  startDate: string;
  endDate: string;
}

// SprintComplete mirrors issuerepo.SprintComplete: what a sprint_complete row
// carries. moveTo is empty for the backlog.
export interface SprintComplete {
  boardId: number;
  name: string;
  moveTo: string;
  moveToName: string;
}

// SprintEdit mirrors issuerepo.SprintEdit: what a sprint_edit row carries.
export interface SprintEdit {
  boardId: number;
  name: string;
  goal: string;
  startDate: string;
  endDate: string;
  clearGoal: boolean;
}

// DraftSprint mirrors issuerepo.DraftSprint: what a sprint_create row carries.
export interface DraftSprint {
  boardId: number;
  boardName: string;
  name: string;
  goal: string;
  startDate: string;
  endDate: string;
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
  // One of TAM's six logical types, or the project's own name for a type
  // TAM has none for. The New issue dialog offers the types the project
  // really has, and some of them are neither of TAM's concepts nor a
  // renaming of one (issue #65 item 2). The Jira backend accepts a name
  // verbatim only when the project's type list carries it.
  type: DraftType;
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
  // Where a drag has put the draft on the board, or where the New issue
  // dialog's Sprint picker or the importer's Sprint column said it belongs.
  // A draft has no Jira state to journal a move against, so a board drag
  // rewrites these on the draft itself; without them the Pending changes
  // dialog could not say a draft had been moved at all, while the same move
  // on a real issue gets a row. The create sends neither: the Sprint field
  // is missing from most Data Center create screens, so Rekey journals the
  // sprint as a move under the key Jira hands back once the create lands.
  // Optional for the reason Issue.pending is: the backend always sends
  // them, and fixtures written before the board writes do not.
  statusId?: string;
  sprintId?: string;
  sprintName?: string;
  extra: Record<string, string>;
  // The extra field ids the dialog offered for this type, read off the
  // create screen. The create sends no extra outside them. Absent on a draft
  // written before the set existed and on the importer's drafts.
  screenFields?: string[];
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

// CreateFieldSet is one type's create fields plus whether the instance could
// say what its create screen carries. screenKnown is false when the answer
// came from Jira's classic create-meta call, which on some Data Center
// versions lists fields the screen does not carry: only required fields are
// offered and sent then, and the dialog says why.
export interface CreateFieldSet {
  fields: FieldSpec[];
  screenKnown: boolean;
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
// discards, and retryable is false for the ones that will fail identically
// forever. The three are optional for the reason Issue.pending is: the
// backend always sends them, but fixtures written before the board pass
// existed do not spell them out. The statuses a refused transition could
// have reached instead are in the error sentence itself, so nothing here
// reads them a second time.
export interface CommitFailure {
  key: string;
  error: string;
  entityType?: string;
  rowId?: number;
  retryable?: boolean;
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

// CommitHeld mirrors committer.Held: a row the Commit did not send because
// something it names was not created in Jira. It stays pending; reason is
// the sentence that says what it waits for.
export interface CommitHeld {
  key: string;
  entityType: string;
  rowId: number;
  waitsFor: string;
  reason: string;
}

export interface CommitResult {
  committed: string[];
  // leftOut names the draft's extra fields the create did not send. TAM
  // leaves out a field nothing could confirm is on the issue type's create
  // screen, rather than letting Jira refuse the whole create over it, and
  // the banner is where that stops being silent. Optional for the reason
  // Issue.pending is: fixtures written before it do not carry it.
  created: { tempKey: string; key: string; leftOut?: string[] }[];
  // createdSprints and held are optional for the same reason CommitFailure's
  // fields are: fixtures written before phased Commit do not spell them out.
  createdSprints?: { draftId: number; id: number; name: string }[];
  // sprintsChanged names each pushed sprint edit, start, completion or
  // delete, "Sprint 12 edited"; optional for the same reason.
  sprintsChanged?: string[];
  linked: { key: string; toKey: string; type: string }[];
  // moved is optional for the same reason CommitFailure's fields are.
  moved?: CommitMove[];
  conflicts: Conflict[];
  failures: CommitFailure[];
  held?: CommitHeld[];
  remaining: number;
}

// TransitionCheck is what CanTransition answers with. It is best effort: an
// error means the check could not be made, never that the move is illegal.
export interface TransitionCheck {
  reason?: string;
  reachable: string[];
  allowed: boolean;
}

export interface ImportPreview {
  headers: string[];
  rowCount: number;
  sample: string[];
}

export interface ImportMapping {
  key: string;
  type: string;
  summary: string;
  description: string;
  priority: string;
  labels: string;
  assignee: string;
  storyPoints: string;
  parentKey: string;
  sprint: string;
}

export interface ImportRowError {
  row: number;
  message: string;
}

export interface ImportResult {
  rows: number;
  created: string[];
  updated: string[];
  errors: ImportRowError[];
  // sprintCellsIgnored counts the keyed rows whose mapped Sprint cell held a
  // value: a sprint is a board write, not a field, so the cell is read by
  // nothing and the row's other fields land without it.
  sprintCellsIgnored: number;
}

// IMPORT_FIELDS are the fields a column can feed, in dialog order. Key comes
// first because it decides what the whole row does: mapped and filled in, the
// row updates the issue it names instead of creating a new one.
export const IMPORT_FIELDS: { id: keyof ImportMapping; label: string }[] = [
  { id: "key", label: "Issue key (updates)" },
  { id: "type", label: "Type" },
  { id: "summary", label: "Summary" },
  { id: "description", label: "Description" },
  { id: "priority", label: "Priority" },
  { id: "labels", label: "Labels" },
  { id: "assignee", label: "Assignee" },
  { id: "storyPoints", label: "Story points" },
  { id: "parentKey", label: "Parent key" },
  { id: "sprint", label: "Sprint" },
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
// RefreshDetails is the shell's Refresh: it drops the profile's cached issue
// details so the next read of whatever is on screen goes back to Jira. It
// takes the app's per-profile lock, so it is called through SyncContext and
// never from a component directly.
export const RefreshDetails: (profileId: string) => Promise<void> = App.RefreshDetails;

// The board bindings. GetBoard takes its arguments plainly: all four are
// scalars, so nothing has to go through a generated class's createFrom. The
// view is cast for the same reason ListIssues is: the generated cards type
// their issue type as a plain string.
export const ListBoards: (profileId: string) => Promise<Board[]> = App.ListBoards;
export const ListBoardSprints: (profileId: string, boardId: number) => Promise<Sprint[]> =
  App.ListBoardSprints;
// ListOpenSprints is every active or future sprint across every board the
// profile has synced, for a caller with no board of its own: the Backlog and
// the Epics tree belong to a project, not to a board, and this is what lets
// their Sprint field offer a choice instead of only printing a fact.
export const ListOpenSprints: (profileId: string) => Promise<SprintChoice[]> =
  App.ListOpenSprints;
export const GetBoard = (
  profileId: string,
  boardId: number,
  sprintId: string,
  swimlane: string,
): Promise<BoardView> =>
  App.GetBoard(profileId, boardId, sprintId, swimlane) as Promise<BoardView>;
export const SyncBoards: (profileId: string) => Promise<BoardSummary> = App.SyncBoards;

// The two sprint ceremonies and the read the start dialog opens with. Both
// ceremonies are journaled and sent on Commit, but take the app's
// per-profile lock, so both go through SyncContext's runQuietLock rather
// than being called from a component directly. Commit works a completion's
// unfinished set out from Jira.
export const StartSprint: (
  profileId: string,
  boardId: number,
  sprintId: number,
  name: string,
  goal: string,
  start: string,
  end: string,
) => Promise<void> = App.StartSprint;
export const CompleteSprint: (
  profileId: string,
  boardId: number,
  sprintId: number,
  moveTo: string,
) => Promise<void> = App.CompleteSprint;
export const SuggestSprintDates = (profileId: string, boardId: number): Promise<SprintSuggestion> =>
  App.SuggestSprintDates(profileId, boardId) as Promise<SprintSuggestion>;
// CreateSprint, EditSprint and DeleteSprint make, change and destroy a
// sprint. All three are journaled and sent on Commit, and take the same
// per-profile lock the two ceremonies do, the same way. CreateSprint's four
// fields are the dialog's whole draft;
// EditSprint's clearGoal is the one argument the draft alone cannot carry,
// since an empty goal box left alone and one asking to clear a goal that was
// there are different requests.
export const CreateSprint = (
  profileId: string,
  boardId: number,
  name: string,
  goal: string,
  start: string,
  end: string,
): Promise<SprintCreated> =>
  App.CreateSprint(profileId, boardId, name, goal, start, end) as Promise<SprintCreated>;
export const EditSprint = (
  profileId: string,
  boardId: number,
  sprintId: number,
  name: string,
  goal: string,
  start: string,
  end: string,
  clearGoal: boolean,
): Promise<void> => App.EditSprint(profileId, boardId, sprintId, name, goal, start, end, clearGoal);
export const DeleteSprint: (profileId: string, boardId: number, sprintId: number) => Promise<void> =
  App.DeleteSprint;
// ListBoardSprintDetails is the Sprints view's whole read: one board's
// sprints in the order that view wants them, each with its cards and its
// progress numbers, and the board's own unassigned work as a last node. It
// is cast for the same reason GetBoard is, since the generated cards type
// their issue type as a plain string.
export const ListBoardSprintDetails = (profileId: string, boardId: number): Promise<SprintDetail[]> =>
  App.ListBoardSprintDetails(profileId, boardId) as Promise<SprintDetail[]>;
// GetSprintReport is one sprint's reconstructed series and its board's
// velocity table together, in the one call that takes the per-profile lock
// once. sprintId of 0 asks for the board's most recent closed sprint, and
// refresh rebuilds from Jira instead of from the stored series, for the
// whole table rather than only the sprint on screen. It takes Go's
// per-profile lock, so a caller reaches it through SyncContext the way the
// sprint writes do rather than calling it directly.
//
// The sprint id is checked here rather than in a component so every future
// caller inherits the check. Wails marshals arguments with JSON.stringify,
// which turns both undefined and NaN into null, and Go decodes that as 0:
// an uninitialised picker would otherwise be served the newest closed
// sprint's report under whatever heading the view happened to be showing.
export const GetSprintReport = async (
  profileId: string,
  boardId: number,
  sprintId: number,
  refresh: boolean,
): Promise<SprintReport> => {
  if (!Number.isInteger(sprintId) || sprintId < 0) {
    throw new Error(`a sprint report needs a sprint id that is zero or more, not ${String(sprintId)}`);
  }
  return App.GetSprintReport(profileId, boardId, sprintId, refresh) as Promise<SprintReport>;
};
// CancelSprintReport stops whatever report this profile has running, so a
// fetch of six sprints of changelog does not hold the lock against the next
// sync for a screen nobody is looking at. It does nothing when none is
// running, which is why the view can call it on every unmount.
export const CancelSprintReport: (profileId: string) => Promise<void> = App.CancelSprintReport;

// The report as something other than a screen. A ReportDocument is headings,
// sentences, tables and the caveats on them, built by lib/reportDocument out
// of lib/reportText and lib/reportTables, so the Confluence page, the
// spreadsheet and the deck all read the report's one vocabulary and Go words
// none of it. notes is the caveats, and no renderer may drop them.
export interface ReportTable {
  columns: string[];
  rows: string[][];
}

export interface ReportSection {
  heading: string;
  lines: string[];
  table: ReportTable;
  notes: string[];
}

export interface ReportDocument {
  title: string;
  sections: ReportSection[];
}

// PublishedPage is the Confluence page a publish wrote, so the view can say
// which one it was.
export interface PublishedPage {
  title: string;
  pageId: string;
}

// PublishSprintReport writes the report to its own page under the sprint's
// Confluence page, through the transport the rituals sync uses. It is a
// write, so it happens when the user asks for it and never on mount, and it
// takes Go's per-profile lock under "report". It reaches no Jira.
export const PublishSprintReport = (
  profileId: string,
  boardId: number,
  sprintId: number,
  doc: ReportDocument,
): Promise<PublishedPage> =>
  App.PublishSprintReport(profileId, boardId, sprintId, reportout.Document.createFrom(doc)) as Promise<PublishedPage>;

// The two file exports. Each writes beside tam.db, the convention
// ExportDiagnostics set, and answers with the path it wrote.
export const ExportSprintReportXLSX = (doc: ReportDocument): Promise<string> =>
  App.ExportSprintReportXLSX(reportout.Document.createFrom(doc));
export const ExportSprintReportPPTX = (doc: ReportDocument): Promise<string> =>
  App.ExportSprintReportPPTX(reportout.Document.createFrom(doc));

// How many journal rows belong to cards staying in this sprint. The Complete
// button asks before it opens its dialog: a card dragged to Done an hour ago
// is Done on the board and not in Jira, and completing the sprint would move
// it to the backlog as unfinished.
export const PendingInSprint: (profileId: string, sprintId: number) => Promise<number> =
  App.PendingInSprint;
// The board's bulk move: one journal row per card, in one transaction, and
// nothing reaches Jira until Commit. It answers with how many of the given
// cards were moved, which is every one the cache holds.
export const JournalSprintMoves: (profileId: string, keys: string[], sprintId: string) => Promise<number> =
  App.JournalSprintMoves;

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
// CreateDraftBoard and AddIssuesToBoard are local writes too: no lock, no
// Jira call, and they return as soon as the journal rows are written.
// CreateDraftBoard answers with the board's negative placeholder id, which
// the picker uses at once to add issues to it before it is real.
export const CreateDraftBoard: (
  profileId: string,
  name: string,
  boardType: string,
  filterName: string,
  jql: string,
) => Promise<number> = App.CreateDraftBoard;
export const AddIssuesToBoard: (profileId: string, keys: string[], boardId: number, scope: string) => Promise<void> =
  App.AddIssuesToBoard;
export const SetProfileSetting: (
  profileId: string,
  key: string,
  value: string,
) => Promise<void> = App.SetProfileSetting;

export const EditIssue: (profileId: string, key: string, field: string, value: string) => Promise<void> =
  App.EditIssue;
export const CreateIssue = (profileId: string, draft: IssueDraft): Promise<string> =>
  App.CreateIssue(profileId, backend.IssueDraft.createFrom(draft));
export const GetCreateFields: (profileId: string, typeName: string) => Promise<CreateFieldSet> =
  App.GetCreateFields;
// The fields Jira says this issue's edit screen carries, by the same names
// EDITABLE_FIELDS uses. An empty array means nothing is known, which is what
// a profile that has never reached Jira gets; the panel falls back to
// EDITABLE_FIELDS then rather than refusing every field.
export const GetEditableFields: (profileId: string, key: string) => Promise<EditableField[]> =
  App.GetEditableFields as (profileId: string, key: string) => Promise<EditableField[]>;
export const ListUnpushableEdits: (profileId: string) => Promise<UnpushableEdit[]> =
  App.ListUnpushableEdits;
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
export const SaveImportTemplate: (profileId: string) => Promise<string> = App.SaveImportTemplate;

export const SearchUsers: (profileId: string, query: string) => Promise<JiraUser[]> =
  App.SearchUsers;
export const ListPriorities: (profileId: string) => Promise<string[]> = App.ListPriorities;
// The Jira name of this profile's sub-task type, "" when the project has
// none. TAM's own word for the level is "Sub-task"; the instance may call it
// anything, and this is how the forms say which.
export const GetSubtaskTypeName: (profileId: string) => Promise<string> =
  App.GetSubtaskTypeName;
// The issue types this profile's project offers, as the last sync recorded
// them. It reads the local store and asks Jira nothing, so the New issue
// dialog opens on it without a round trip. Empty is a profile that has
// never synced, not a project with no types.
export const ListProjectTypes: (profileId: string) => Promise<ProjectType[]> =
  App.ListProjectTypes;

export const GetLinkTypes: (profileId: string) => Promise<LinkType[]> = App.GetLinkTypes;
export const GetConfluenceConfig: (profileId: string) => Promise<ConfluenceConfig> = App.GetConfluenceConfig as any;
export const SetConfluenceConfig = (profileId: string, config: ConfluenceConfig, token: string): Promise<void> =>
  App.SetConfluenceConfig(profileId, profile.ConfluenceConfig.createFrom(config), token);
// A ritual page as tam.db keeps it. body is the local page in Confluence
// storage format; baseBody the page as of version, the last one synced;
// conflictBody a newer remote a Sync found while local edits were pending.
export type RitualStatus = "local" | "synced" | "unsynced" | "conflict" | "gone";
export interface RitualDocument {
  profileId: string;
  boardId: number;
  sprintId: number;
  ritualType: string;
  title: string;
  body: string;
  baseBody: string;
  pageId: string;
  version: number;
  conflictBody: string;
  conflictVersion: number;
  status: RitualStatus;
  updatedAt: string;
  syncedAt: string;
}
export interface RitualPageFailure { sprintName: string; title: string; reason: string }
// RitualRootMissing is a Sync that stopped because the configured root page
// answered 404. canCreate is the permission probe's answer, unknown read as yes.
export interface RitualRootMissing { pageId: string; spaceKey: string; canCreate: boolean; suggestedTitle: string }
export interface RitualSyncResult {
  // Set, and nothing else is, when the root page is gone. Optional only so
  // fixtures written before it stay valid; Go always sends it, null when the
  // root was read.
  rootMissing?: RitualRootMissing | null;
  created: number;
  pulled: number;
  pushed: number;
  conflicts: number;
  gone: number;
  failed: RitualPageFailure[];
  syncedAt: string;
}
export interface RitualMacroPreview { supported: boolean; jql: string; issues: Issue[] }

export const EnsureSprintRituals: (profileId: string, boardId: number, sprintId: number) => Promise<RitualDocument[]> = App.EnsureSprintRituals as any;
export const ListRitualDocuments: (profileId: string, boardId: number, sprintId: number) => Promise<RitualDocument[]> = App.ListRitualDocuments as any;
// version and pageId are what the editor was opened on; the save is refused
// once a Sync has moved the row past either.
export const SaveRitualBody: (profileId: string, boardId: number, sprintId: number, ritualType: string, body: string, version: number, pageId: string) => Promise<RitualDocument> = App.SaveRitualBody as any;
export const ResolveRitualConflict: (profileId: string, boardId: number, sprintId: number, ritualType: string, choice: "mine" | "theirs") => Promise<void> = App.ResolveRitualConflict;
export const ForgetRitualPage: (profileId: string, boardId: number, sprintId: number, ritualType: string) => Promise<void> = App.ForgetRitualPage;
export const DeleteRitualDocument: (profileId: string, boardId: number, sprintId: number, ritualType: string) => Promise<void> = App.DeleteRitualDocument;
export const RitualMacroIssues: (profileId: string, jql: string) => Promise<RitualMacroPreview> = App.RitualMacroIssues as any;
export const StandupEntry: (day: string) => Promise<string> = App.StandupEntry;
export const LastRitualSync: (profileId: string, boardId: number) => Promise<string> = App.LastRitualSync;
export const SyncRituals: (profileId: string, boardId: number) => Promise<RitualSyncResult> = App.SyncRituals as any;

export type RitualRootOutcome = "created" | "adopted" | "forbidden" | "titleTaken";
export interface RitualRoot { outcome: RitualRootOutcome; pageId: string; title: string; spaceKey: string; topLevel: boolean }
// sync is the pass Go ran on the new root, null when no root was set; syncError
// is that pass's refusal, with the new root saved either way.
export interface RitualRootResult { root: RitualRoot; sync: RitualSyncResult | null; syncError: string }
// CreateRitualRoot takes Go's "rituals" lock. Call it only through
// SyncContext.runRitualRoot, never directly.
export const CreateRitualRoot: (profileId: string, boardId: number, title: string, adopt: boolean) => Promise<RitualRootResult> = App.CreateRitualRoot as any;

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
