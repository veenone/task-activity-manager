// Package sprints owns the five writes that reach Jira the moment they are
// made: starting a sprint, completing one, and creating, editing and
// deleting one. They are the only writes in TAM that do not go through the
// journal, and the reason is a cost rather than a principle.
//
// A sprint's id has to be real before anything can point at it. TAM carries
// the machinery to defer an issue's id, the TAM-NEW- draft key and the
// commit pass that swaps it for Jira's, and carries nothing that would defer
// a sprint's: journaling a create would mean inventing a local sprint id,
// teaching the commit pass to rekey every issue_sprint row aimed at it, and
// putting an id that is not real into the board cache, the Backlog's sprint
// filter and the importer's Sprint column. Calling that a principle is how a
// sixth exception gets added without an argument, so it is written down as
// what it is.
//
// The other four follow the first. A sprint is a container a whole team
// plans into, so one that exists on a single laptop is one nobody else can
// move an issue into, and there is nothing to reconcile later either: a card
// move can be rebased onto a status that shifted underneath it, while a
// sprint somebody else has already deleted cannot be renamed.
//
// The exception has one home here, one place to test, and a fence:
// exceptions_test.go names the Service's own exported methods, so a sixth
// immediate write arrives with a failing test rather than quietly.
//
// This package writes no journal rows. It reads their count, to refuse a
// completion or a delete while changes are queued against the sprint, and it
// leaves an audit row behind each of the three management writes. Moving an
// issue into or out of a sprint stays an ordinary journal write and lives in
// issuerepo, where every other one does.
package sprints

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/errtext"
)

// pushBatch is how many cards one push of a completion carries. Twenty
// rather than Jira's fifty, matching the committer's own batch: the bulk
// endpoints answer a partial refusal with a 207 that names issues by numeric
// id, which cannot be mapped back to keys, so the whole batch fails
// together. A smaller batch is how much of a completion one refusal can
// take down.
const pushBatch = 20

// searchPage is how many issues one page of the sprint's own read asks for,
// the same fifty the sync uses.
const searchPage = 50

// errNoLifecycle is what a backend that cannot speak Jira's Agile API
// answers a ceremony with. It says nothing about permissions: TAM does not
// guess at those before trying.
var errNoLifecycle = errors.New("this connection cannot manage sprints")

// Backend is what a ceremony needs from the profile's backend before the
// Agile capability is asked for: the issue search, which is how a sprint's
// issues come back with their statuses in one call rather than one call per
// card. backend.IssueBackend satisfies it.
type Backend interface {
	SearchIssuesPage(ctx context.Context, projectKey, scopeJQL, since string, types []string, startAt, maxResults int) ([]backend.Issue, int, error)
}

// lifecycle is the part of backend.BoardBackend a ceremony uses: the six
// writes and the one read that records what they did. Like the committer's
// own board writer, it asks for those and not for the board configuration,
// so a test's backend does not have to answer for a board's columns to close
// a sprint.
//
// MoveIssuesToSprint is the call the plan names PushIssuesToSprint when it
// is used this way. The name matters: the completion pushes to Jira now,
// bypassing the journal exactly as the rest of the ceremony does, and it is
// not the bulk binding of the same underlying call, which only ever writes
// journal rows.
type lifecycle interface {
	BoardSprints(ctx context.Context, boardID int) ([]backend.Sprint, error)
	MoveIssuesToSprint(ctx context.Context, sprintID string, keys []string) error
	StartSprint(ctx context.Context, sprintID int, d backend.SprintDraft) error
	CompleteSprint(ctx context.Context, sprintID int) error
	CreateSprint(ctx context.Context, boardID int, d backend.SprintDraft) (backend.Sprint, error)
	EditSprint(ctx context.Context, sprintID int, d backend.SprintDraft, clearGoal bool) error
	DeleteSprint(ctx context.Context, sprintID int) error
}

// Store is what these writes need from the board cache: the columns, which
// are what "complete" is defined against, the destination's name, and the
// writes that keep the cache honest once Jira has moved.
//
// DeleteSprintEverywhere is the last of those and the only one that carries
// no board id, because Jira hands one sprint to every board whose filter
// reaches it and a delete that cleaned one board would leave the other
// board's copy standing in OpenSprints. boardrepo's own comment holds the
// rest of that argument, and the order it has to be called in.
type Store interface {
	Columns(ctx context.Context, profileID string, boardID int) ([]backend.BoardColumn, error)
	BoardSprintState(ctx context.Context, profileID string, boardID int, sprintID string) (string, bool, error)
	SprintName(ctx context.Context, profileID, sprintID string) (string, error)
	SprintIssues(ctx context.Context, profileID string, boardID int, sprintID string) ([]string, error)
	ReplaceSprints(ctx context.Context, profileID string, boardID int, sprints []backend.Sprint) error
	ReplaceSprintIssues(ctx context.Context, profileID string, boardID int, sprintID string, keys []string) error
	DeleteSprintEverywhere(ctx context.Context, profileID string, sprintID int) error
}

// Issues is the issue cache's side of a sprint write, which is a different
// repository from the board cache Store is: the cached sprint columns a
// delete has to blank, and the audit row each of the three management writes
// leaves behind. issuerepo.Repository satisfies it.
//
// It is its own seam rather than two more methods on Store because the two
// caches are two repositories with two transactions, which is exactly what
// makes the delete's cache surgery an ordered pair rather than one write.
type Issues interface {
	ClearSprint(ctx context.Context, profileID, sprintID string) error
	AuditSprint(ctx context.Context, profileID string, sprintID int, action, before, after string) error
}

// Completion is what a completion did, and a push that failed partway is one
// of the things it can have done. A failed push has already taken some cards
// out of the sprint, so "twelve of forty moved and the sprint is still open"
// is the only honest report, and a count alone cannot say it: Failed names
// the cards still in the sprint, which is the list the user needs to decide
// what to do next.
//
// That report travels in Message rather than in a Go error, because Wails
// discards a bound method's return value whenever the method also returns a
// non-nil error: the dispatcher fills in either the result or the error and
// never both. An error would therefore deliver the sentence and drop the
// keys it is about, which is the one thing the user needs at that moment.
//
// Every count here is over the issue types TAM syncs (backend.AllTypes) and
// nothing else. A sprint holding a card of a type this project defines for
// itself is a sprint the completion never sees that card in: it is not
// counted, not moved to the chosen destination, and lands in the backlog by
// Jira's own close behaviour. So Moved and Failed describe what the
// completion considered, and neither is offered as the size of the sprint.
type Completion struct {
	// Moved is how many of the issues it considered left the sprint.
	Moved int `json:"moved"`
	// MovedTo names where they went, the backlog or a sprint by name.
	MovedTo string `json:"movedTo"`
	// Failed are the incomplete issues that did not move, empty when they
	// all did.
	Failed []string `json:"failed"`
	// Note is what did not land after the sprint was closed, empty when
	// everything did. It is never a failure of the completion: the sprint is
	// closed and the cards have moved, and this is the cache bookkeeping
	// after them, whose one visible consequence is a picker still offering
	// Start for a sprint that is already running.
	Note string `json:"note"`
	// Message is why the completion did not finish, empty when it did. A
	// completion carrying one has left the sprint open: either the push
	// stopped partway, and Failed names the cards still in the sprint, or
	// every card left and the close itself was refused, and Failed is empty
	// because the open sprint holds none of them.
	Message string `json:"message"`
}

// Service runs the ceremonies against one profile's backend and the board
// cache. It is built per call, the way the commit engine is, because the
// backend it wraps belongs to a profile.
type Service struct {
	b       Backend
	store   Store
	project string

	// PageSize and PushBatch are the two loop widths, exported so a test can
	// exercise a second chunk without inventing forty issues.
	PageSize  int
	PushBatch int

	// Pending, when set, answers how many journal rows target an issue that
	// is staying in the sprint. A completion refuses while any do: a card
	// dragged to Done an hour ago is Done on the user's screen and not in
	// Jira, and completing the sprint would move it to the backlog as
	// unfinished. The toolbar asks the same question before it opens the
	// dialog, but the board's writes are deliberately unguarded, so a drag
	// can land between that answer and this one.
	Pending func(ctx context.Context, profileID string, sprintID int) (int, error)

	// Issues, when set, is the issue cache: where a deleted sprint's name is
	// blanked off the cards that carried it, and where the audit row of a
	// create, an edit or a delete is written. It is wired beside Pending and
	// for the same reason, since app.go is the one place holding both
	// repositories.
	//
	// Both of its calls are bookkeeping after Jira has already moved, so an
	// unset seam is logged rather than refused: a build that forgot to wire
	// it leaves stale sprint text on cached issues until the next full sync,
	// which is the same damage a failed call would do.
	Issues Issues
}

// New builds the service over a profile's backend, the board cache, and the
// project its issues live in.
func New(b Backend, store Store, projectKey string) *Service {
	return &Service{b: b, store: store, project: projectKey, PageSize: searchPage, PushBatch: pushBatch}
}

// Start starts the sprint on Jira with the draft the dialog filled in, and
// records the new state by refreshing that board's sprint list.
//
// The dates arrive as a date input produced them, a bare day with no time
// and no zone, and are read and rewritten in Jira's own datetime format
// here rather than concatenated into one: a value that is not a date is
// refused where the message can name it. So is an end before a start, and
// that check lives here rather than in the dialog because the bound method
// is reachable without it.
//
// Jira's own error is returned unchanged. A sprint that cannot be started,
// because another is already active or because the account cannot manage
// sprints, is answered in a sentence the user needs to read word for word.
//
// The sprint has started once Jira says so, whatever happens to the cache
// afterwards, so a sprint list that could not be re-read comes back as the
// note beside a success rather than as a failure. Without it the picker
// still calls the running sprint future, the toolbar still offers Start for
// it, and Jira answers that second start with a 400 nobody can explain.
func (s *Service) Start(ctx context.Context, profileID string, boardID, sprintID int, d backend.SprintDraft) (string, error) {
	b, err := s.board()
	if err != nil {
		return "", err
	}
	if d.StartDate, d.EndDate, err = dates(d.StartDate, d.EndDate); err != nil {
		return "", err
	}
	if err := b.StartSprint(ctx, sprintID, d); err != nil {
		return "", err
	}
	return note(s.refreshSprints(ctx, b, profileID, boardID)), nil
}

// Complete moves the sprint's unfinished cards and then closes it, in that
// order: the reverse would leave a closed sprint whose issues went nowhere,
// which nobody can undo from TAM. moveTo is the destination sprint's id, or
// empty for the backlog, which is a destination and not an absence.
//
// The sprint's issues are re-read from Jira and not from the cache, which
// can be minutes stale and would otherwise decide the fate of cards the user
// cannot see. The read is the issue search rather than the board's key list,
// because a key alone cannot say whether the card finished: an issue is
// complete when its status id is in the last board column's status ids, the
// same mapping the board itself draws with.
//
// A ceremony that reached Jira and then failed is reported in the Completion
// and not as an error, so what did happen travels with the sentence about
// it: the keys a half-finished push left behind, and the counts a refused
// close has to be read against. An error is kept for the refusals that
// happen before anything moves, which have nothing to report but themselves.
func (s *Service) Complete(ctx context.Context, profileID string, boardID, sprintID int, moveTo string) (Completion, error) {
	sid := strconv.Itoa(sprintID)
	done := Completion{Failed: []string{}}
	b, err := s.board()
	if err != nil {
		return done, err
	}
	moveTo, err = destinationID(moveTo, sprintID)
	if err != nil {
		return done, err
	}
	if done.MovedTo, err = s.destination(ctx, profileID, moveTo); err != nil {
		return done, err
	}
	if err := s.refusePending(ctx, profileID, sprintID); err != nil {
		return done, err
	}
	complete, err := s.completeStatuses(ctx, profileID, boardID)
	if err != nil {
		return done, err
	}
	if err := s.requireCompletable(ctx, profileID, boardID, sid); err != nil {
		return done, err
	}
	incomplete, err := s.sprintIssues(ctx, sid, complete)
	if err != nil {
		return done, err
	}

	moved := make([]string, 0, len(incomplete))
	for start := 0; start < len(incomplete); start += s.pushWidth() {
		end := start + s.pushWidth()
		if end > len(incomplete) {
			end = len(incomplete)
		}
		if err := b.MoveIssuesToSprint(ctx, moveTo, incomplete[start:end]); err != nil {
			// Everything from this chunk on is still in the sprint. The
			// chunks before it are not, so the cache is corrected before the
			// user is told, or they are left with cards that vanished from an
			// open sprint with nothing recording where they went.
			done.Failed = append(done.Failed, incomplete[start:]...)
			done.Moved = len(moved)
			done.Message = fmt.Sprintf("%d of %d unfinished issues moved to %s, so the sprint was left open: %s",
				done.Moved, len(incomplete), done.MovedTo, errtext.Line(err))
			s.refreshMembership(ctx, profileID, boardID, sid, moveTo, moved)
			// No Go error: the keys in Failed are what the dialog has to
			// name, and Wails drops the value when an error goes with it.
			return done, nil
		}
		moved = append(moved, incomplete[start:end]...)
	}
	done.Moved = len(moved)

	if err := b.CompleteSprint(ctx, sprintID); err != nil {
		// The worst state this feature reaches: every unfinished card has
		// left a sprint that is still open. The move path says so and this
		// one has to say it too, or the sentence the user reads is Jira's
		// bare refusal with no word about where their cards went.
		//
		// It travels in Message rather than as a Go error for the same
		// reason the failed push does, and for one more. The dialog renders
		// a Completion carrying a Message as an outcome and a Go error as a
		// refusal, so an error printed this accurate sentence directly above
		// a list still headed "47 cards are not finished and will move out
		// of the sprint" and a footer still promising the move: both future
		// tense, both already false, on the one state nobody can undo.
		done.Message = fmt.Sprintf("%d of %d unfinished issues moved to %s, but the sprint could not be closed and is open with none of them in it: %s",
			done.Moved, len(incomplete), done.MovedTo, errtext.Line(err))
		s.refreshMembership(ctx, profileID, boardID, sid, moveTo, moved)
		return done, nil
	}
	s.refreshMembership(ctx, profileID, boardID, sid, moveTo, moved)
	done.Note = note(s.refreshSprints(ctx, b, profileID, boardID))
	return done, nil
}

// board is the Agile capability behind this profile's backend.
func (s *Service) board() (lifecycle, error) {
	bb, ok := s.b.(lifecycle)
	if !ok || s.b == nil {
		return nil, errNoLifecycle
	}
	return bb, nil
}

func (s *Service) pushWidth() int {
	if s.PushBatch > 0 {
		return s.PushBatch
	}
	return pushBatch
}

func (s *Service) pageWidth() int {
	if s.PageSize > 0 {
		return s.PageSize
	}
	return searchPage
}

// sprintIssues reads the sprint from Jira in one paged query and keeps the
// keys of the cards that did not finish.
//
// The query is narrowed to the sprint on the wire, and the answer is
// narrowed again here. That is not belt and braces: it is what keeps an
// irreversible move honest against a backend that answers with a card the
// sprint no longer holds, and it can only ever move fewer cards, never
// more.
//
// An issue the backend reports no sprint for is kept, since the query is
// what put it in this list. That leniency is the one part of this read that
// leans on the backend: a backend whose search ignores the scope hands the
// whole project to this loop, every card of it reports no sprint, and every
// card of it walks past the guard and out of the backlog. Both the demo
// backend and this package's fake narrow "sprint = N" for exactly that
// reason.
func (s *Service) sprintIssues(ctx context.Context, sprintID string, complete map[string]bool) ([]string, error) {
	incomplete := []string{}
	startAt, total := 0, -1
	for total < 0 || startAt < total {
		page, n, err := s.b.SearchIssuesPage(ctx, s.project, "sprint = "+sprintID, "", backend.AllTypes, startAt, s.pageWidth())
		if err != nil {
			return nil, fmt.Errorf("read sprint %s: %w", sprintID, err)
		}
		total = n
		for _, iss := range page {
			if iss.SprintID != "" && iss.SprintID != sprintID {
				continue
			}
			if !complete[iss.StatusID] {
				incomplete = append(incomplete, iss.Key)
			}
		}
		startAt += len(page)
		if len(page) == 0 {
			break
		}
	}
	return incomplete, nil
}
