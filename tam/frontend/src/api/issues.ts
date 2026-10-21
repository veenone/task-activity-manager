// Issues: their shapes, the Backlog's query and columns, and the reads.

import * as App from "../../wailsjs/go/main/App";
import { issuerepo } from "../../wailsjs/go/models";
import type { BoardSummary } from "./boards";

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
  // Jira's own bucket for the status (schema 19): "new", "indeterminate" or
  // "done", the same three keys on every instance where the status name is
  // whatever that instance calls it. Absent or empty when nothing has read
  // one for this row, and statusClass guesses from the name instead.
  statusCategory?: string;
  assignee: string;
  // The Jira username sync, edits, and drafts cache alongside assignee
  // (schema 14). Optional so fixtures written before assigned-to-me still
  // type-check; a row cached before schema 14 sends "" until the next sync.
  assigneeName?: string;
  // The assignee's email (schema 26). Blank where Jira hides email, and on a
  // row cached before schema 26 until the next sync.
  assigneeEmail?: string;
  reporter: string;
  priority: string;
  labels: string[];
  sprintId: string;
  sprintName: string;
  parentKey: string;
  storyPoints?: number | null;
  // Jira's time tracking on the issue, in seconds (schema 23). Absent or
  // null is "this issue carries no such time", which the grid leaves blank:
  // zero would say the team estimated it at no work. The aggregate is what
  // the issue and its children burned together, which Jira counts apart
  // from the issue's own.
  originalEstimateSeconds?: number | null;
  remainingEstimateSeconds?: number | null;
  timeSpentSeconds?: number | null;
  // The family's three: the issue and its sub-tasks together, which is
  // where every figure sits for an issue estimated through its children.
  // lib/issueTime decides which set a surface shows.
  aggregateEstimateSeconds?: number | null;
  aggregateRemainingSeconds?: number | null;
  aggregateTimeSpentSeconds?: number | null;
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

// Worklog is one entry against an issue. started carries its own UTC offset,
// which is what Jira dates the entry by, so it is read as a stamp and never
// normalised. timeSpent is the phrase somebody typed and seconds is what Jira
// made of it, which is the only one a total adds up from. pending and
// pendingId mark an entry the journal still holds, for the same reason Link
// carries them: the section draws Jira's entries and TAM's own in one list
// and only the second kind can be taken back.
export interface Worklog {
  id: string;
  author: string;
  authorName: string;
  started: string;
  timeSpent: string;
  seconds: number;
  comment: string;
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
  // The filter bar's own two lists. Empty is no filter, several entries
  // are an OR inside the field, and the fields are an AND. Both hold the
  // names the sync stored, which is what the grid shows and what
  // ListFilterOptions offers.
  //
  // assignees holds display names, so it is a different filter from
  // assigneeName below. An empty entry means unassigned, which is why it
  // is a list: "" on its own would read as no filter at all, and
  // ASSIGNEE_NONE is what the select uses for that choice. Both are
  // optional for the same reason assigneeName is: only a view with a
  // filter bar drives them, and the store reads a missing list as no
  // filter.
  statuses?: string[];
  assignees?: string[];
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
  | "storyPoints"
  | "timeSpent";

// GRID_COLUMNS is the Backlog's eight columns in display order: the header
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
  { id: "timeSpent", label: "Time", firstClickDesc: true },
];

export interface IssuePage {
  issues: Issue[];
  total: number;
}

export interface TreeQuery {
  text: string;
  sprintId: string;
  showDone: boolean;
  // The same three lists IssueQuery carries, read the same way. They
  // narrow the children, and an epic the filter leaves no child is dropped.
  types?: string[];
  statuses?: string[];
  assignees?: string[];
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

// FilterOptions is what the filter bar offers for the two fields whose
// values are the project's own: the statuses and the assignee names this
// profile's cached issues carry. Issue types come from ListProjectTypes.
// Neither list holds an empty entry, so the bar adds its All and
// Unassigned options itself.
export interface FilterOptions {
  statuses: string[];
  assignees: string[];
}

// ASSIGNEE_NONE is the assignee select's value for Unassigned. The query
// sends assignees as a list, where [] is no filter and [""] is nobody, so
// the select needs a value for nobody that is not the empty string its All
// option already uses. It is mapped back to "" before the query goes out.
export const ASSIGNEE_NONE = "__unassigned__";

export interface SyncState {
  lastSynced: string;
  lastFull: string;
  lastError: string;
  issueCount: number;
  // What the project held when the last sync counted it, 0 when none has.
  // issueCount below it is a cache the profile's scope JQL, or a type the
  // backend holds back, has narrowed, and the status bar says so rather
  // than letting a narrow cache read as a small project.
  projectTotal: number;
}

export interface SyncSummary {
  fetched: number;
  upserted: number;
  // No skipped count: since #100 a sync stores every row it fetched, so the
  // number would always be zero and would read as proof nothing was lost.
  // How many issues the project holds, counted without the scope and the
  // cut-off that narrow the fetch.
  projectTotal: number;
  full: boolean;
  elapsed: string;
  // boards is the boards pass's own summary, absent when the pass did not
  // run at all (syncer.Summary marks it omitempty). It is the only way a
  // dropped board, or a Jira with no Agile API, reaches the user from an
  // automatic sync, which is the sync most users ever run.
  boards?: BoardSummary;
}

export interface LinkedTest {
  key: string;
  summary: string;
  linkType: string;
}

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
export const ListFilterOptions: (profileId: string) => Promise<FilterOptions> =
  App.ListFilterOptions;
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

// ExportBacklog writes the rows a query matches to a workbook, wherever
// the save dialog says, and answers with the path. An empty path is a
// cancelled dialog: nothing was written and nothing went wrong. It is the
// filter that is exported, not the page, so the caller sends its query
// with the paging cleared.
export const ExportBacklog = (profileId: string, q: IssueQuery): Promise<string> =>
  App.ExportBacklog(profileId, issuerepo.IssueQuery.createFrom(q)) as Promise<string>;
