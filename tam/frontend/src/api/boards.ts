// Boards and sprints: the board view, sprint ceremonies, and the board writes.

import * as App from "../../wailsjs/go/main/App";
import type { Issue } from "./issues";
import type { ReportPerson } from "./reports";

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

export interface ColumnView {
  name: string;
  // statusIds are the statuses the column collects, in the board's own
  // order. A drop journals the first of them, and a column with none
  // collects nothing and cannot be a drop target.
  statusIds: string[];
  total: number;
  points: number;
  // counted is the cards the column's WIP limit is measured against: total,
  // or total without the subtasks on a board whose constraint excludes them.
  // min and max are the limits themselves, null for a column Jira sets none
  // on, which is the ordinary case and never a breach; zero is a real limit
  // and not the same fact. constraint is what the board counts a limit in,
  // Jira's own "issueCount" or "issueCountExclSubs".
  //
  // All four are optional because a board cached before TAM read the limits
  // carries none of them, and so does a report stored before then.
  // lib/columnLimit is where that absence is turned into "no limit".
  counted?: number;
  min?: number | null;
  max?: number | null;
  constraint?: string;
  // localMax is the maximum the team set for this column in TAM, null for a
  // column they set none on. It is beside Jira's pair rather than folded into
  // it because the two are different facts: Jira's wins wherever the board
  // sets one, and every surface that prints a limit says which of the two it
  // is showing. lib/columnLimit decides the precedence and the wording.
  localMax?: number | null;
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

// TransitionCheck is what CanTransition answers with. It is best effort: an
// error means the check could not be made, never that the move is illegal.
export interface TransitionCheck {
  reason?: string;
  reachable: string[];
  allowed: boolean;
}

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
// BoardAllocation is the board's open work per assignee (#288), read from the
// cache with no Jira call. The rows are the People section's shape.
export const BoardAllocation = (profileId: string, boardId: number): Promise<ReportPerson[]> =>
  App.BoardAllocation(profileId, boardId) as unknown as Promise<ReportPerson[]>;

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

// SetColumnLimit stores the WIP limit the team set for one board column, or
// clears it when limit is empty. limit is the text the user typed rather than a
// number: Go is the one place that decides what a limit may be, so a word, a
// negative number and one past its ceiling come back as an error the dialog
// reads out. Nothing here reaches Jira; TAM never writes board configuration.
export const SetColumnLimit: (
  profileId: string,
  boardId: number,
  column: string,
  limit: string,
) => Promise<void> = App.SetColumnLimit;

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
