// Package sprints pushes the sprint writes Commit sends: an edit, a delete,
// a start and a completion of a sprint Jira holds. None of them reaches Jira
// when the user asks. Each is a journal row issuerepo writes, and Commit's
// sprint changes phase pushes it through ForCommit, which reads what Jira or
// the cache holds before it writes. Creating a sprint is a draft under a
// negative id that Commit creates first; DraftSprint is the check a draft
// gets.
//
// This package writes no journal rows. It reads their count, to refuse a
// completion or a delete while changes are queued against the sprint, and it
// leaves an audit row behind each pushed edit and delete. Moving an issue
// into or out of a sprint stays an ordinary journal write and lives in
// issuerepo, where every other one does.
package sprints

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

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

// lifecycle is the part of backend.BoardBackend a push uses: the writes and
// the one read that records what they did. Like the committer's own board
// writer, it asks for those and not for the board configuration, so a test's
// backend does not have to answer for a board's columns to close a sprint.
// MoveIssuesToSprint is the completion's own move of the unfinished cards,
// made at Commit from Jira's read, and not a journaled sprint move.
type lifecycle interface {
	BoardSprints(ctx context.Context, boardID int) ([]backend.Sprint, error)
	MoveIssuesToSprint(ctx context.Context, sprintID string, keys []string) error
	StartSprint(ctx context.Context, sprintID int, d backend.SprintDraft) error
	CompleteSprint(ctx context.Context, sprintID int) error
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
// delete has to blank, and the audit row each of the two management writes
// leaves behind. issuerepo.Repository satisfies it.
//
// It is its own seam rather than two more methods on Store because the two
// caches are two repositories with two transactions, which is exactly what
// makes the delete's cache surgery an ordered pair rather than one write.
type Issues interface {
	ClearSprint(ctx context.Context, profileID, sprintID string) error
	AuditSprint(ctx context.Context, profileID string, sprintID int, action, field, before, after string) error
}

// Completion is what a completion did, and a push that failed partway is one
// of the things it can have done. A failed push has already taken some cards
// out of the sprint, so "twelve of forty moved and the sprint is still open"
// is the only honest report, and a count alone cannot say it: Failed names
// the cards still in the sprint, which is the list the user needs to decide
// what to do next.
//
// That report travels in Message rather than in a Go error, so Commit can
// tell a completion that moved cards and stopped, which it reports as a
// failure worth retrying and keeps the row for, from a refusal before
// anything moved. Message names the keys that did move.
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
	// everything did, or that the cache already held the sprint as closed.
	// It is never a failure of the completion: the sprint is closed, and this
	// is the cache bookkeeping after it.
	Note string `json:"note"`
	// Message is why the completion did not finish, empty when it did. A
	// completion carrying one has left the sprint open: either the push
	// stopped partway, and Failed names the cards still in the sprint, or
	// every card left and the close itself was refused, and Failed is empty
	// because the open sprint holds none of them.
	Message string `json:"message"`
}

// Service holds what a push needs: one profile's backend and the board
// cache. It is built per Commit, the way the commit engine is, because the
// backend it wraps belongs to a profile. It exports no write; ForCommit is
// how Commit reaches them.
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
	// blanked off the cards that carried it, and where the audit row of an
	// edit or a delete is written. It is wired beside Pending and for the
	// same reason, since app.go is the one place holding both repositories.
	//
	// Edit treats an unset seam as a footnote: its call is bookkeeping after
	// Jira has already moved, so a missing audit row is logged and nothing
	// more. Delete does not get that leniency, because its call to Issues is
	// not the same kind of afterthought: it is what keeps Jira's board tables
	// and the issue cache from naming a sprint forever that Jira no longer
	// has, the crash window boardrepo's own comment says a full sync repairs,
	// made permanent instead of momentary, and it is what writes the one
	// audit row that will be the only trace of the sprint left anywhere once
	// Jira has destroyed it. Refusing before a delete touches Jira costs
	// nothing; letting it through and losing both afterwards cannot be
	// undone. So Delete checks this field for nil itself, before it calls
	// Jira at all, rather than discovering the gap the way Edit does, after
	// there is nothing left to refuse.
	//
	// That check only catches the field being left unset. app.go's a.repo is
	// a concrete *issuerepo.Repository, and assigning a nil one here would
	// produce a typed nil interface, which s.Issues == nil does not see; a
	// build that wired a nil repository would panic inside Issues' own
	// calls rather than being caught here. Nothing in this package guards
	// against that: app.go's startup sets a.repo before anything can reach a
	// bound sprint method, and that ordering, not this field, is what keeps
	// a nil repository from ever being wired in the first place.
	Issues Issues
}

// New builds the service over a profile's backend, the board cache, and the
// project its issues live in.
func New(b Backend, store Store, projectKey string) *Service {
	return &Service{b: b, store: store, project: projectKey, PageSize: searchPage, PushBatch: pushBatch}
}

// Start pushes a journaled start with the draft the dialog filled in, and
// records the new state by refreshing that board's sprint list.
//
// The dates are read and rewritten in Jira's own datetime format again, so a
// row that is not a date is refused where the message can name it, and so
// is an end before a start.
//
// Jira's own error is returned unchanged. A sprint that cannot be started,
// because another is already active or because the account cannot manage
// sprints, is answered in a sentence the user needs to read word for word.
//
// The sprint has started once Jira says so, whatever happens to the cache
// afterwards, so a sprint list that could not be re-read comes back as the
// note beside a success rather than as a failure. Without it the picker
// still calls the running sprint future and the toolbar still offers Start.
func (c Committed) Start(ctx context.Context, profileID string, boardID, sprintID int, d backend.SprintDraft) (string, error) {
	s := c.s
	if sprintID < 0 {
		return "", ErrDraftSprint
	}
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

// Complete pushes a journaled completion: it moves the sprint's unfinished
// cards and then closes it, in that order, since the reverse would leave a
// closed sprint whose issues went nowhere. moveTo is the destination
// sprint's id, or empty for the backlog, which is a destination and not an
// absence.
//
// The unfinished set is worked out here, from Jira, and not taken from the
// dialog's preview: Jira's state at the moment of the write is what decides.
// The read is the issue search rather than the board's key list, because a
// key alone cannot say whether the card finished: an issue is complete when
// its status id is in the last board column's status ids, the same mapping
// the board itself draws with.
//
// A push that reached Jira and then failed is reported in the Completion and
// not as an error, so what did happen travels with the sentence about it.
// Calling it again is safe: a retry after a failed move or a failed close
// moves whatever is still unfinished and closes the sprint, and a retry
// after a close whose journal row could not be cleared finds the sprint
// closed in the refreshed cache and moves nothing.
//
// ponytail: that last case leans on the refresh after the close; if the
// refresh failed too, the retry asks Jira to close a closed sprint and
// reports Jira's refusal, which the user discards.
func (c Committed) Complete(ctx context.Context, profileID string, boardID, sprintID int, moveTo string) (Completion, error) {
	s := c.s
	if sprintID < 0 {
		return Completion{Failed: []string{}}, ErrDraftSprint
	}
	sid := strconv.Itoa(sprintID)
	done := Completion{Failed: []string{}}
	b, err := s.board()
	if err != nil {
		return done, err
	}
	moveTo, err = DestinationID(moveTo, sprintID)
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
	if closed, err := s.requireCompletable(ctx, profileID, boardID, sid); err != nil {
		return done, err
	} else if closed {
		done.Note = fmt.Sprintf("sprint %d was already closed, so nothing was moved", sprintID)
		return done, nil
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
			done.Message = fmt.Sprintf("%d of %d unfinished issues moved to %s%s, so the sprint was left open: %s",
				done.Moved, len(incomplete), done.MovedTo, keyList(moved), errtext.Line(err))
			s.refreshMembership(ctx, profileID, boardID, sid, moveTo, moved)
			return done, nil
		}
		moved = append(moved, incomplete[start:end]...)
	}
	done.Moved = len(moved)

	if err := b.CompleteSprint(ctx, sprintID); err != nil {
		// The worst state this feature reaches: every unfinished card has
		// left a sprint that is still open. The message names where they
		// went and which they were, or the sentence the user reads is Jira's
		// bare refusal with no word about their cards.
		done.Message = fmt.Sprintf("%d of %d unfinished issues moved to %s%s, but the sprint could not be closed and is open with none of them in it: %s",
			done.Moved, len(incomplete), done.MovedTo, keyList(moved), errtext.Line(err))
		s.refreshMembership(ctx, profileID, boardID, sid, moveTo, moved)
		return done, nil
	}
	s.refreshMembership(ctx, profileID, boardID, sid, moveTo, moved)
	done.Note = note(s.refreshSprints(ctx, b, profileID, boardID))
	return done, nil
}

// keyList is " (PLAT-1, PLAT-2)" for the keys a stopped completion moved,
// and nothing when it moved none.
func keyList(keys []string) string {
	if len(keys) == 0 {
		return ""
	}
	return " (" + strings.Join(keys, ", ") + ")"
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
func (s *Service) sprintIssues(ctx context.Context, sprintID string, complete func(string) bool) ([]string, error) {
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
			if !complete(iss.StatusID) {
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
