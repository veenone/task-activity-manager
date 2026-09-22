// Package backend is the seam between Task Activity Manager and the system
// that holds its issues. IssueBackend carries the read path and the write
// path plan 1b added: version checks, edits, and issue creation. The Jira
// implementation lives in backend/jira, the offline one in backend/demo.
// BoardBackend is the board capability the Boards view and the board writes
// need, on its own interface so a backend that cannot answer it does not
// have to.
package backend

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Logical issue types. Every issue TAM manages is one of these five; the
// Jira names they map to are the backend's business.
const (
	TypeTask        = "task"
	TypeEpic        = "epic"
	TypeStory       = "story"
	TypeBug         = "bug"
	TypeRequirement = "requirement"
	// TypeSubtask is Jira's sub-task level: an issue that hangs off another
	// issue through the parent field rather than off an epic through the Epic
	// Link. Its Jira name is per-instance ("Sub-task" by default, "Technical
	// task" on some), so it is discovered from the project rather than
	// hardcoded.
	TypeSubtask = "subtask"
)

// AllTypes is the six logical types in display order.
var AllTypes = []string{TypeTask, TypeEpic, TypeStory, TypeBug, TypeRequirement, TypeSubtask}

// Issue is one row of the Backlog: the columns the grid shows plus what sync
// needs to keep it current. StoryPoints is nil when the issue has none.
type Issue struct {
	Key          string   `json:"key"`
	ID           string   `json:"id"`
	Project      string   `json:"project"`
	Type         string   `json:"type"`
	Summary      string   `json:"summary"`
	Status       string   `json:"status"`
	StatusID     string   `json:"statusId"`
	Assignee     string   `json:"assignee"`
	AssigneeName string   `json:"assigneeName"`
	Reporter     string   `json:"reporter"`
	Priority     string   `json:"priority"`
	Labels       []string `json:"labels"`
	SprintID     string   `json:"sprintId"`
	SprintName   string   `json:"sprintName"`
	ParentKey    string   `json:"parentKey"`
	StoryPoints  *float64 `json:"storyPoints"`
	Rank         string   `json:"rank"`
	Created      string   `json:"created"`
	Updated      string   `json:"updated"`

	// Description is the issue's description, read with the rest of the row
	// by the sync and kept on it, so the detail panel draws it from the
	// local store instead of a round trip per selection.
	//
	// A pointer because nil and "" are different facts and the panel words
	// them differently. nil is "nothing has read this issue's description":
	// a row cached before schema 17, or a response that did not carry the
	// field. A pointer to "" is "Jira says this issue has no description",
	// which is what its null decodes to. Collapsing the two would have the
	// panel tell a reader an issue has no description when the truth is
	// that TAM has never looked.
	Description *string `json:"description"`

	// Pending and Draft are computed by the repository's reads, never
	// stored: Pending says the journal holds a change for this key, Draft
	// says the key is a local placeholder Commit has not yet created.
	Pending bool `json:"pending"`
	Draft   bool `json:"draft"`
}

// IssueDraft is a new issue as the form captured it. Type is the logical
// type (task, story, bug). Extra carries the create-meta required fields
// the form rendered as text, keyed by Jira field id; the backend sends each
// as a string, or as {"name": value} when the field takes an option.
type IssueDraft struct {
	Type        string   `json:"type"`
	Summary     string   `json:"summary"`
	Description string   `json:"description"`
	Priority    string   `json:"priority"`
	Labels      []string `json:"labels"`
	Assignee    string   `json:"assignee"`
	StoryPoints *float64 `json:"storyPoints"`

	// ParentKey is the epic (or parent) the draft belongs under. The Jira
	// backend sends it through the Epic Link field when it exists.
	ParentKey string `json:"parentKey"`

	// StatusID, SprintID, and SprintName are where a draft was last dropped
	// on a board, or where the form that made it said it belongs. A draft
	// has no Jira state, so a drag moves it in place rather than journalling
	// a transition against an issue Jira has never seen.
	//
	// The create sends none of the three. A new issue lands in its
	// workflow's first status whatever the board showed, and the Sprint
	// field is missing from most Data Center create screens, so a create
	// carrying it is refused. The sprint is not dropped, though: Rekey
	// journals it as a sprint move under the key Jira hands back, and the
	// board pass of the same Commit pushes it through the Agile endpoint.
	StatusID   string `json:"statusId"`
	SprintID   string `json:"sprintId"`
	SprintName string `json:"sprintName"`

	// ScreenFields are the extra field ids the create dialog offered for
	// this draft's type, read off the create screen when it was drafted.
	// The create sends no extra outside this set, which is what keeps a
	// field createmeta listed but the screen does not carry out of the
	// payload without a network call at Commit. Nil is a draft made before
	// the set existed or by the importer, whose extras are empty anyway.
	ScreenFields []string `json:"screenFields"`

	Extra map[string]string `json:"extra"`
}

// PendingMove is every board intent the journal holds for one issue, folded
// into one value so the board read can apply them in memory instead of
// asking per card. The three Has flags are what say a field is set: an
// empty SprintID is the backlog, which is a destination and not an absence,
// and reading it as "no sprint move" is how a card moved off a board would
// quietly stay on it.
type PendingMove struct {
	Key      string `json:"key"`
	StatusID string `json:"statusId"`
	SprintID string `json:"sprintId"`
	// StatusName and SprintName are the names journaled beside the two ids,
	// empty when the cache never held one. The board read overrides them on
	// the card together with the ids: IsDone reads the status name, so a
	// card drawn in a Done column while its name still said In Progress
	// would leave the view's own points total disagreeing with the column
	// it drew.
	StatusName string `json:"statusName"`
	SprintName string `json:"sprintName"`
	// RankNeighbour is the key the card was dropped against and RankBefore
	// which side of it. The rank itself is never cached: a made-up LexoRank
	// would be a second source of truth the next sync overwrites.
	RankNeighbour string `json:"rankNeighbour"`
	RankBefore    bool   `json:"rankBefore"`

	// BoardID and BoardScope are what AddToBoard queued for this issue: the
	// destination board and where on it, BoardScopeBacklog or a sprint id.
	// AddToBoard refuses an empty scope, so a non-empty BoardScope is what
	// says an add is pending.
	//
	// ponytail: an issue queued onto two boards at once keeps only the last
	// one this fold saw, the way every other field here holds one value per
	// issue; a per-board list would be needed to draw both at once.
	BoardID    int    `json:"boardId"`
	BoardScope string `json:"boardScope"`

	HasTransition bool `json:"hasTransition"`
	HasSprint     bool `json:"hasSprint"`
	HasRank       bool `json:"hasRank"`
}

// SplitLabels turns the comma list the form and the journal use back into
// Jira's label slice, trimming blanks.
func SplitLabels(s string) []string {
	out := []string{}
	for _, l := range strings.Split(s, ",") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// NonNil returns s, or an empty slice when s is nil. A nil slice encodes as
// JSON null, and null is what makes a label list or a column's status ids
// read as "missing" rather than "none"; the store layers all go through
// here so none of them has to carry a copy of the same three lines.
func NonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// FormatPoints renders story points the way the journal and the forms
// show them: a plain number, or empty for none.
func FormatPoints(p *float64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatFloat(*p, 'f', -1, 64)
}

// ParsePoints reads the text form back; blank means none.
func ParsePoints(s string) (*float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, errors.New("story points must be a number")
	}
	return &v, nil
}

// FieldSpec is one create-meta field the New issue form must ask for
// because Jira requires it and the form does not already carry it. Type is
// string, option, number, date, or array; option and array fields list
// their AllowedValues.
type FieldSpec struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Type          string        `json:"type"`
	Required      bool          `json:"required"`
	AllowedValues []FieldOption `json:"allowedValues"`
}

// CreateFieldSet is one type's create fields and whether the instance could
// say which fields its create screen carries. ScreenKnown is false when the
// answer came from Jira's classic create-meta call, which on some Data
// Center versions lists fields the screen does not carry: only the fields
// Jira marks required are offered and sent then, and the dialog says so.
type CreateFieldSet struct {
	Fields      []FieldSpec `json:"fields"`
	ScreenKnown bool        `json:"screenKnown"`
}

// FieldOption is one allowed value of an option field.
type FieldOption struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

// LinkType is one of Jira's issue link types with its two phrasings, for
// example Blocks: "blocks" outward, "is blocked by" inward.
type LinkType struct {
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
}

// LinkDraft is a link the user asked for: the type, which side the source
// issue is on, and the target with the summary and type the lookup showed.
type LinkDraft struct {
	Type      string `json:"type"`
	Direction string `json:"direction"` // "outward" or "inward"
	ToKey     string `json:"toKey"`
	ToSummary string `json:"toSummary"`
	ToType    string `json:"toType"`
}

// Link is one issue link seen from the issue that owns it.
type Link struct {
	Direction string `json:"direction"` // "outward" or "inward"
	Type      string `json:"type"`      // the Jira link type name, e.g. "Tested By"
	Key       string `json:"key"`       // the other issue
	Summary   string `json:"summary"`
	IssueType string `json:"issueType"` // the other issue's Jira type name
	// Pending marks a link the journal holds and Commit has not pushed;
	// PendingID is its journal row, for Discard.
	Pending   bool  `json:"pending"`
	PendingID int64 `json:"pendingId"`
}

// Comment is one comment on an issue, as the detail panel reads it.
//
// Author (the username) and AuthorName (the display name) are both empty
// for a comment Jira answered with no author at all, which is what an
// anonymous comment and one whose author has since been deleted look like;
// the panel words that for itself rather than inventing a name here.
// Restriction is the role or group a restricted comment is limited to, and
// empty for an ordinary one: a comment only some people can see must never
// be drawn as one everybody can.
//
// Created and Updated are RFC 3339, normalised from whatever Jira sent (its
// offsets carry no colon); a value that cannot be read is kept exactly as it
// arrived rather than dropped.
type Comment struct {
	ID          string `json:"id"`
	Author      string `json:"author"`
	AuthorName  string `json:"authorName"`
	Created     string `json:"created"`
	Updated     string `json:"updated"`
	Body        string `json:"body"`
	Restriction string `json:"restriction"`
}

// IssueDetail is what the detail panel shows beyond the grid columns.
// Fields holds the decoded custom fields keyed by field id, so a later
// phase's edit form can build on the same shape.
type IssueDetail struct {
	Key         string         `json:"key"`
	Description string         `json:"description"`
	Links       []Link         `json:"links"`
	Fields      map[string]any `json:"fields"`

	// Comments are the issue's comments, oldest first, so the newest are the
	// last few the panel shows. CommentTotal is how many the issue has,
	// which is not len(Comments) once either limit below has bitten, and
	// CommentsTruncated says so: the read keeps the newest 500 and stops, and
	// a comment page that failed leaves what was already read behind rather
	// than failing the whole detail. Every surface printing a count has to
	// say which of the two it is.
	//
	// CommentsTruncated with an empty Comments means the comment read
	// failed, and never that the issue has none: an issue nobody has
	// commented on comes back with a total of 0 and truncated false. So a
	// panel must not print "No comments." for the first case, which would
	// report a transport failure as a fact about the issue.
	//
	// Comment.Restriction is only visibility.value, not its type, so a
	// restricted comment can be named by its role or group but never
	// described as one or the other. Saying "role: Developers" over a group
	// of that name would be a confident wrong answer about who can see it;
	// the value alone is enough to mark the comment as restricted.
	Comments          []Comment `json:"comments"`
	CommentTotal      int       `json:"commentTotal"`
	CommentsTruncated bool      `json:"commentsTruncated"`

	// FetchedAt is when the store cached this detail, RFC 3339. The
	// repository's read fills it in; a detail straight off the wire has
	// none, and the panel prints it so a stale offline read says so.
	FetchedAt string `json:"fetchedAt"`
}

// IssueType is one issue type a project offers, under the project's own
// name for it.
type IssueType struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Subtask is Jira's own flag: a type at the sub-task level, which
	// cannot exist without a parent. The New issue dialog leaves these out,
	// because a sub-task is drafted from the issue it belongs to.
	Subtask bool `json:"subtask"`
	// Logical is TAM's own type for this one, "" when TAM has none. The
	// empty string is the honest answer and is carried as such: a type
	// nothing here recognises must not be drawn, stored or created as a
	// task (issue #65 item 2).
	Logical string `json:"logical"`
}

// User is the authenticated user, from the connection test.
type User struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

// Board types TAM draws. Jira has more (simple boards, for one); the
// backend filters the rest out rather than caching a board the Boards view
// could not lay out.
const (
	BoardTypeScrum  = "scrum"
	BoardTypeKanban = "kanban"
)

// Board is one Jira Agile board of a project.
type Board struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	// ProjectKey is the project the board belongs to, "" when the instance
	// did not say. Jira's board list answers with every board whose filter
	// mentions the project being synced, so this is what separates a
	// project's own boards from another team's that happen to include it.
	ProjectKey string `json:"projectKey"`
}

// BoardColumn is one column of a board's configuration. StatusIDs are the
// Jira status ids the column collects; a column may have none, which is
// how a Backlog column Jira never fills is described.
type BoardColumn struct {
	Name      string   `json:"name"`
	StatusIDs []string `json:"statusIds"`
}

// Sprint is one sprint of a board. State is Jira's own lowercase value
// (active, future, closed), and BoardID is the board the sprint was read
// from, not the board it was created on. Goal is Jira's own sprint goal,
// carried straight through from core/jira.RawSprint. CompleteDate is the
// moment the sprint actually closed, empty for one that has not, and is
// routinely days from EndDate, the planned one.
type Sprint struct {
	ID           int    `json:"id"`
	BoardID      int    `json:"boardId"`
	Name         string `json:"name"`
	State        string `json:"state"`
	StartDate    string `json:"startDate"`
	EndDate      string `json:"endDate"`
	Goal         string `json:"goal"`
	CompleteDate string `json:"completeDate"`
}

// SprintDraft is a sprint's fields for starting, creating, or editing one: a
// name (prefilled from the sprint's own when one exists), an optional goal,
// and start and end dates already in the Agile API's own datetime format.
// Turning a bare date, the shape an HTML date input produces, into that
// format is sprints.Service's job, not the backend's.
type SprintDraft struct {
	Name      string `json:"name"`
	Goal      string `json:"goal"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

// BoardBackend is the board capability, kept off IssueBackend so only the
// backends that speak Jira's Agile API have to answer for it. The boards
// sync pass and the commit pass's rank and sprint group both ask for it
// with a type assertion and skip themselves when a backend does not have
// it. Everything above RankIssue reads; RankIssue and everything below it
// write.
type BoardBackend interface {
	// Boards lists the project's boards from Jira's Agile API, scrum and
	// kanban only. It never writes.
	Boards(ctx context.Context, projectKey string) ([]Board, error)
	// BoardColumns reads one board's column configuration from Jira's
	// Agile API. It never writes.
	BoardColumns(ctx context.Context, boardID int) ([]BoardColumn, error)
	// BoardSprints lists one board's sprints from Jira's Agile API, empty
	// for a board that has none. It never writes.
	BoardSprints(ctx context.Context, boardID int) ([]Sprint, error)
	// BoardIssueKeys lists the keys the board holds, for one sprint when
	// sprintID is set and for the whole board when it is empty, narrowed to
	// projectKey. A board's filter is not bounded by a project, so without
	// that narrowing a board can answer with tens of thousands of keys of
	// which only the synced project's are usable. It reads Jira's Agile API
	// and never writes.
	BoardIssueKeys(ctx context.Context, boardID int, sprintID, projectKey string) ([]string, error)
	// RankIssue ranks key immediately before or after neighbourKey. Jira's
	// rank is one order across the whole board, so the neighbour may sit in
	// another column; the commit pass re-derives it from the board's own
	// order at push time.
	RankIssue(ctx context.Context, key, neighbourKey string, before bool) error
	// MoveIssuesToSprint moves keys onto the sprint, or onto the backlog
	// when sprintID is empty, which is a destination and not an absence.
	// The endpoint takes a batch, so the commit pass groups a planning
	// session's moves by target instead of paying a round trip per card.
	MoveIssuesToSprint(ctx context.Context, sprintID string, keys []string) error
	// StartSprint starts sprintID with the draft's name, goal, and dates. It
	// reaches Jira immediately: the sprint lifecycle is the one write in TAM
	// that does not go through the journal, because a sprint's start is a
	// timestamped fact a whole team reads and cannot be told to Jira an
	// hour late.
	StartSprint(ctx context.Context, sprintID int, s SprintDraft) error
	// CompleteSprint closes sprintID, sending state closed and nothing else
	// so it never rewrites the sprint's dates. Moving the issues that did
	// not finish is the caller's job, through MoveIssuesToSprint (called
	// PushIssuesToSprint when it is used this way, to keep it apart from
	// the journal binding of the same underlying call); CompleteSprint only
	// closes.
	CompleteSprint(ctx context.Context, sprintID int) error
	// CreateSprint creates a sprint on boardID with the draft's name, goal,
	// and dates, and returns it as this package's own Sprint, BoardID set
	// from the argument rather than from whatever the wire answered with.
	// Like StartSprint, it reaches Jira immediately: creating a sprint is
	// one of the writes TAM does not journal, for the reason written on
	// core/jira's UpdateSprint.
	CreateSprint(ctx context.Context, boardID int, d SprintDraft) (Sprint, error)
	// EditSprint edits sprintID with the draft's name, dates, and goal,
	// touching only the fields the draft carries. clearGoal is the
	// exception: when true it sends an empty goal on purpose, since a goal
	// otherwise can never be taken away, only overwritten by a new one. It
	// reaches Jira immediately, the same as CreateSprint.
	EditSprint(ctx context.Context, sprintID int, d SprintDraft, clearGoal bool) error
	// DeleteSprint deletes sprintID. Jira returns the sprint's issues to
	// the backlog rather than deleting them; nothing here removes them from
	// TAM's own cache, which is the caller's job. It reaches Jira
	// immediately, the same as CreateSprint.
	DeleteSprint(ctx context.Context, sprintID int) error
}

// BoardDraft is a new board's fields as the create dialog captured them:
// the name and type it should have, and the filter that will back it, built
// from a name and the JQL that scopes it.
type BoardDraft struct {
	Name       string
	Type       string // "scrum" or "kanban"
	FilterName string
	JQL        string
}

// BoardScopeBacklog is the scope AddToBoard takes for a card queued onto a
// board's backlog rather than one of its sprints.
const BoardScopeBacklog = "backlog"

// DraftPrefix starts the temporary key of an issue created locally and not
// yet committed. Commit swaps it for Jira's key.
const DraftPrefix = "TAM-NEW-"

// BoardCreator is the board-creating half of BoardBackend, kept off it on
// purpose: BoardBackend's own doc says it never writes, and a board create
// is exactly that, a write. It is a separate optional interface the
// committer type-asserts for, the same way it already type-asserts
// boardWriter at committer/boards.go:125, so a backend that cannot create
// boards simply does not answer for it rather than forcing every
// BoardBackend to grow a method it may not support.
type BoardCreator interface {
	// CreateBoard creates a board from the draft and returns its id. The
	// Jira implementation makes it in two Jira calls, a filter then the
	// board on it, and rolls the filter back when the board create fails,
	// so a failed attempt never leaves an orphaned filter behind. projectKey
	// is the project the board and its filter belong to.
	CreateBoard(ctx context.Context, projectKey string, d BoardDraft) (int, error)
	// AddToBoardBacklog adds keys to boardID's backlog.
	AddToBoardBacklog(ctx context.Context, boardID int, keys []string) error
}

// BoardFilterChecker is the courtesy read the committer makes once per board
// after it pushes issue_board adds: of the keys just pushed, which ones
// Jira's own board filter actually kept. Kept optional, the same as
// BoardCreator and for the same reason, so a backend that cannot answer it
// simply skips the check rather than the push it followed failing over it.
type BoardFilterChecker interface {
	// BoardFilterCheck reads which of keys are on boardID now. It is a read
	// made after a write that already landed, so its own failure is never
	// the commit's to report.
	BoardFilterCheck(ctx context.Context, boardID int, keys []string) ([]string, error)
}

// ErrNoTransition is what Transition returns when no workflow transition of
// the issue reaches the target status. It is the one board failure that
// will fail identically on every retry, so the commit pass reports it as
// not retryable and names where the card can actually go.
var ErrNoTransition = errors.New("no workflow transition reaches that status")

// NoTransition is the error ErrNoTransition travels in. It names the issue
// and the target and carries the names of the statuses that are reachable,
// so a failure can say where the card can go instead of only where it
// cannot. errors.Is finds ErrNoTransition through it.
//
// TargetStatus is the target's display name, empty where nothing knew it.
// The backend is handed a status id and has no name for a status its
// workflow cannot reach, so the journal row, which carries "id|Name", is
// what fills this in. Without it the sentence named the target by number
// while naming the reachable ones by name, and it is shown verbatim in the
// commit banner.
type NoTransition struct {
	Reason         string
	Key            string
	TargetStatusID string
	TargetStatus   string
	Reachable      []string
}

func (e *NoTransition) Error() string {
	if e.Reason != "" {
		return e.Reason
	}
	target := e.TargetStatus
	if target == "" {
		target = "status " + e.TargetStatusID
	}
	if len(e.Reachable) == 0 {
		return fmt.Sprintf("%s cannot move to %s: its workflow offers no transition at all from where it is now", e.Key, target)
	}
	return fmt.Sprintf("%s cannot move to %s: no transition reaches it. From here it can move to %s", e.Key, target, strings.Join(e.Reachable, ", "))
}

// Unwrap is what makes errors.Is(err, ErrNoTransition) true.
func (e *NoTransition) Unwrap() error { return ErrNoTransition }

// ErrTransitionFields is what Transition returns when the transition asks
// for a field beyond a resolution. Guessing a value for someone else's
// custom field is worse than saying the move has to be made in Jira, and
// like ErrNoTransition it fails the same way every time.
var ErrTransitionFields = errors.New("the transition needs fields TAM cannot fill in")

// TransitionCheck is what CanTransition answers with: whether the target
// status is reachable from where the issue is right now, and the names of
// the statuses that are. It is best effort, so a failed check means "we
// could not ask", never "the move is illegal".
type TransitionCheck struct {
	Reason    string   `json:"reason,omitempty"`
	Reachable []string `json:"reachable"`
	Allowed   bool     `json:"allowed"`
}

// IssueBackend is what the read path needs from the issue system.
type IssueBackend interface {
	TestConnection(ctx context.Context) (User, error)
	IsDemo() bool
	// SearchIssuesPage returns one page of issues in projectKey whose logical
	// type is in types, narrowed by scopeJQL when non-empty and by
	// updated >= since (RFC3339) when non-empty, plus the total match count.
	SearchIssuesPage(ctx context.Context, projectKey, scopeJQL, since string, types []string, startAt, maxResults int) ([]Issue, int, error)
	// GetIssueDetail fetches what the grid does not carry: description,
	// links, and the custom fields.
	GetIssueDetail(ctx context.Context, key string) (IssueDetail, error)
	IssueTypes(ctx context.Context, projectKey string) ([]IssueType, error)
	// SubtaskTypeName is the Jira name of this project's sub-task type, ""
	// when the project has none. The forms need it to say what they are
	// creating, since the instance chooses the word.
	SubtaskTypeName(ctx context.Context, projectKey string) (string, error)
	// GetIssue reads one issue's row fields, for the version check before a
	// commit and the refresh after it.
	GetIssue(ctx context.Context, key string) (Issue, error)
	// UpdateIssue pushes edited fields, keyed by the logical field name
	// (summary, description, priority, labels, storyPoints, assignee) with
	// the text form the journal holds.
	UpdateIssue(ctx context.Context, key string, fields map[string]string) error
	// EditableFields is the part of that same logical set the issue's edit
	// screen carries, sorted. The instance decides it per project and issue
	// type, so the answer holds for every issue sharing those two and the
	// caller caches it against them.
	EditableFields(ctx context.Context, key string) ([]string, error)
	// Transition moves the issue into targetStatusID by firing the workflow
	// transition that reaches it, resolved at push time from what Jira
	// offers for that issue at that moment. It returns ErrNoTransition when
	// none does and ErrTransitionFields when the transition asks for more
	// than a resolution.
	Transition(ctx context.Context, key string, targetStatusIDs []string) error
	// CanTransition reports whether targetStatusID is reachable from where
	// the issue sits now, and what it can reach instead. It is the check a
	// drop makes while the app is online; it never writes.
	CanTransition(ctx context.Context, key string, targetStatusIDs []string) (TransitionCheck, error)
	// CreateIssue creates the draft and returns the key Jira assigned, plus
	// the extra fields it left out of the payload, named for a person: a
	// field nothing could confirm belongs on the create screen is not sent,
	// and Commit says so rather than dropping a typed value in silence.
	CreateIssue(ctx context.Context, projectKey string, d IssueDraft) (string, []string, error)
	// CreateFields lists the create-screen fields of a logical type that
	// the New issue form does not already carry, required and optional,
	// with FieldSpec.Required saying which. The form's own fields (summary,
	// description, priority, labels, assignee, story points, Epic Link,
	// Epic Name, parent, sprint) never come back, whatever createmeta says.
	// ScreenKnown says whether the answer can be read as the create screen.
	CreateFields(ctx context.Context, projectKey, logicalType string) (CreateFieldSet, error)
	// LinkTypes lists the issue link types the instance defines.
	LinkTypes(ctx context.Context) ([]LinkType, error)
	// CreateLink links fromKey to the draft's target with the draft's type
	// and direction.
	CreateLink(ctx context.Context, fromKey string, d LinkDraft) error
	// SearchUsers lists the people who can be assigned an issue in
	// projectKey whose name or display name matches query. A blank query
	// asks for the first page of anyone assignable, which is what seeds the
	// local cache.
	SearchUsers(ctx context.Context, projectKey, query string) ([]User, error)
	// Priorities lists the instance's priority names, highest first, so the
	// forms can offer them instead of asking the user to remember them.
	Priorities(ctx context.Context) ([]string, error)
}
