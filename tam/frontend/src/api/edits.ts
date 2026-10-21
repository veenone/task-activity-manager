// Creating and editing issues: drafts, field specs, links, and work logs.

import * as App from "../../wailsjs/go/main/App";
import { backend } from "../../wailsjs/go/models";
import type { DraftType, Issue, ProjectType, Worklog } from "./issues";

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

// LookupIssue is cast the same way ListIssues is above: the generated
// binding types the issue type as a plain string, narrowed to IssueType here.
export const LookupIssue = (profileId: string, key: string): Promise<Issue> =>
  App.LookupIssue(profileId, key) as Promise<Issue>;
export const AddLink = (profileId: string, key: string, link: LinkDraft): Promise<void> =>
  App.AddLink(profileId, key, backend.LinkDraft.createFrom(link));

// ListWorklogs is Jira's entries for the issue followed by the ones the
// journal holds. It is called when the Work log section is expanded, not on
// every sync: most rows are never asked about, and the answer is Jira's own
// rather than anything the store caches.
export const ListWorklogs: (profileId: string, key: string) => Promise<Worklog[]> =
  App.ListWorklogs as unknown as (profileId: string, key: string) => Promise<Worklog[]>;
// LogWork journals an entry. Nothing reaches Jira until Commit.
export const LogWork: (profileId: string, key: string, timeSpent: string, comment: string) => Promise<void> =
  App.LogWork;
// CheckWorkDuration is the duration rule on its own, so the form can refuse
// what was typed where it was typed. It is the same rule LogWork applies, so
// the two cannot disagree about what Jira will take.
export const CheckWorkDuration: (timeSpent: string) => Promise<void> = App.CheckWorkDuration;
