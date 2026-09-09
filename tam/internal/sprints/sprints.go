// Package sprints owns the two sprint ceremonies: starting one and
// completing one. They are the only writes in TAM that do not go through the
// journal, because a sprint's start is a timestamped fact a whole team reads
// and what a completion does with the cards that did not finish depends on
// the sprint's contents at the moment it closes, not at whatever moment a
// Commit next runs. That exception has one home here, and one place to test.
//
// Nothing in this package touches the journal. The bulk move the board's
// selection makes is an ordinary journal write and lives in issuerepo, where
// every other one does.
package sprints

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/sprintdate"
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
var errNoLifecycle = errors.New("this connection cannot start or complete sprints")

// Backend is what a ceremony needs from the profile's backend before the
// Agile capability is asked for: the issue search, which is how a sprint's
// issues come back with their statuses in one call rather than one call per
// card. backend.IssueBackend satisfies it.
type Backend interface {
	SearchIssuesPage(ctx context.Context, projectKey, scopeJQL, since string, types []string, startAt, maxResults int) ([]backend.Issue, int, error)
}

// lifecycle is the part of backend.BoardBackend a ceremony uses: the three
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
}

// Store is what a ceremony needs from the board cache: the columns, which
// are what "complete" is defined against, the destination's name, and the
// two writes that keep the cache honest once Jira has moved.
type Store interface {
	Columns(ctx context.Context, profileID string, boardID int) ([]backend.BoardColumn, error)
	SprintName(ctx context.Context, profileID, sprintID string) (string, error)
	ReplaceSprints(ctx context.Context, profileID string, boardID int, sprints []backend.Sprint) error
	ReplaceSprintIssues(ctx context.Context, profileID string, boardID int, sprintID string, keys []string) error
}

// Completion is what a completion did, and it is returned together with the
// error rather than instead of it. A push that fails partway has already
// taken some cards out of the sprint, so "twelve of forty moved and the
// sprint is still open" is the only honest report, and a count alone cannot
// say it: Failed names the cards still in the sprint, which is the list the
// user needs to decide what to do next.
type Completion struct {
	// Moved is how many issues left the sprint.
	Moved int `json:"moved"`
	// MovedTo names where they went, the backlog or a sprint by name.
	MovedTo string `json:"movedTo"`
	// Failed are the incomplete issues that did not move, empty when they
	// all did.
	Failed []string `json:"failed"`
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
func (s *Service) Start(ctx context.Context, profileID string, boardID, sprintID int, d backend.SprintDraft) error {
	b, err := s.board()
	if err != nil {
		return err
	}
	if d.StartDate, d.EndDate, err = dates(d.StartDate, d.EndDate); err != nil {
		return err
	}
	if err := b.StartSprint(ctx, sprintID, d); err != nil {
		return err
	}
	s.refreshSprints(ctx, b, profileID, boardID)
	return nil
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
func (s *Service) Complete(ctx context.Context, profileID string, boardID, sprintID int, moveTo string) (Completion, error) {
	sid := strconv.Itoa(sprintID)
	moveTo = strings.TrimSpace(moveTo)
	done := Completion{Failed: []string{}}
	b, err := s.board()
	if err != nil {
		return done, err
	}
	if moveTo != "" {
		// The id ends up in a URL path, so the only honest answer to
		// "sprint fourteen" is that it is not a sprint id at all.
		if _, err := strconv.Atoi(moveTo); err != nil {
			return done, fmt.Errorf("sprint id %q is not a number", moveTo)
		}
	}
	if moveTo == sid {
		return done, errors.New("a sprint cannot be completed into itself")
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
	all, incomplete, err := s.sprintIssues(ctx, sid, complete)
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
			s.refreshMembership(ctx, profileID, boardID, sid, all, moved)
			return done, fmt.Errorf("%d of %d unfinished issues moved to %s, so the sprint was left open: %w",
				done.Moved, len(incomplete), done.MovedTo, err)
		}
		moved = append(moved, incomplete[start:end]...)
	}
	done.Moved = len(moved)

	if err := b.CompleteSprint(ctx, sprintID); err != nil {
		s.refreshMembership(ctx, profileID, boardID, sid, all, moved)
		return done, err
	}
	s.refreshMembership(ctx, profileID, boardID, sid, all, moved)
	s.refreshSprints(ctx, b, profileID, boardID)
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

// destination is what the completion reports as the place the cards went.
func (s *Service) destination(ctx context.Context, profileID, sprintID string) (string, error) {
	if sprintID == "" {
		return "the backlog", nil
	}
	name, err := s.store.SprintName(ctx, profileID, sprintID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(name) == "" {
		return "sprint " + sprintID, nil
	}
	return name, nil
}

// refusePending stops a completion while the journal still holds changes for
// cards staying in this sprint. Committing them first is what makes the
// board and Jira agree about which cards finished.
func (s *Service) refusePending(ctx context.Context, profileID string, sprintID int) error {
	if s.Pending == nil {
		return nil
	}
	n, err := s.Pending(ctx, profileID, sprintID)
	if err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("%d pending change(s) belong to cards in this sprint; commit them before completing it, or Jira will be asked which cards finished before it has been told", n)
	}
	return nil
}

// completeStatuses is the set of status ids that count as finished: the ones
// the board's last column collects. A board whose columns are not cached, or
// whose last column collects nothing, cannot answer the question this action
// turns on, and guessing is not an option for a move nobody can undo from
// TAM.
func (s *Service) completeStatuses(ctx context.Context, profileID string, boardID int) (map[string]bool, error) {
	cols, err := s.store.Columns(ctx, profileID, boardID)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("board %d has no cached columns, so TAM cannot tell which issues finished; refresh the boards first", boardID)
	}
	last := cols[len(cols)-1]
	if len(last.StatusIDs) == 0 {
		return nil, fmt.Errorf("the last column of board %d (%q) collects no statuses, so TAM cannot tell which issues finished", boardID, last.Name)
	}
	set := make(map[string]bool, len(last.StatusIDs))
	for _, id := range last.StatusIDs {
		set[id] = true
	}
	return set, nil
}

// sprintIssues reads the sprint from Jira in one paged query and splits it:
// every key it holds, and the keys of the ones that did not finish.
//
// The query is narrowed to the sprint on the wire, and the answer is
// narrowed again here. That is not belt and braces: it is what keeps an
// irreversible move honest against a backend whose search does not honour
// the scope, and it can only ever move fewer cards, never more. An issue the
// backend reports no sprint for is kept, since the query is what put it in
// this list.
func (s *Service) sprintIssues(ctx context.Context, sprintID string, complete map[string]bool) (all, incomplete []string, err error) {
	all, incomplete = []string{}, []string{}
	startAt, total := 0, -1
	for total < 0 || startAt < total {
		page, n, err := s.b.SearchIssuesPage(ctx, s.project, "sprint = "+sprintID, "", backend.AllTypes, startAt, s.pageWidth())
		if err != nil {
			return nil, nil, fmt.Errorf("read sprint %s: %w", sprintID, err)
		}
		total = n
		for _, iss := range page {
			if iss.SprintID != "" && iss.SprintID != sprintID {
				continue
			}
			all = append(all, iss.Key)
			if !complete[iss.StatusID] {
				incomplete = append(incomplete, iss.Key)
			}
		}
		startAt += len(page)
		if len(page) == 0 {
			break
		}
	}
	return all, incomplete, nil
}

// refreshMembership writes back what the sprint still holds once some of its
// cards have left it. It is best effort and logged: the cards have moved in
// Jira whatever the cache says, and failing the ceremony over a local write
// would report a move that happened as one that did not.
func (s *Service) refreshMembership(ctx context.Context, profileID string, boardID int, sprintID string, all, moved []string) {
	if len(moved) == 0 {
		return
	}
	gone := make(map[string]bool, len(moved))
	for _, key := range moved {
		gone[key] = true
	}
	remaining := make([]string, 0, len(all))
	for _, key := range all {
		if !gone[key] {
			remaining = append(remaining, key)
		}
	}
	if err := s.store.ReplaceSprintIssues(ctx, profileID, boardID, sprintID, remaining); err != nil {
		log.Printf("tam: sprint %s membership could not be refreshed after %d cards left it: %v", sprintID, len(moved), err)
	}
}

// refreshSprints re-reads that board's sprints and nothing else, so the
// picker shows the state the ceremony just produced. It is not a boards
// sync: that walks every board's columns, sprints and membership, takes
// minutes on a real project, and would be refused outright by the lock the
// ceremony itself is holding.
func (s *Service) refreshSprints(ctx context.Context, b lifecycle, profileID string, boardID int) {
	list, err := b.BoardSprints(ctx, boardID)
	if err != nil {
		log.Printf("tam: board %d sprints could not be re-read after the ceremony: %v", boardID, err)
		return
	}
	if err := s.store.ReplaceSprints(ctx, profileID, boardID, list); err != nil {
		log.Printf("tam: board %d sprints could not be cached after the ceremony: %v", boardID, err)
	}
}

// dates turns the two the dialog collected into the pair Jira is sent,
// refusing rather than guessing. An empty date is refused because a sprint
// with one end is not a sprint, and an end before a start is refused here
// rather than only in the dialog.
func dates(start, end string) (string, string, error) {
	from, err := sprintdate.Parse(start)
	if err != nil {
		return "", "", fmt.Errorf("start date: %w", err)
	}
	to, err := sprintdate.Parse(end)
	if err != nil {
		return "", "", fmt.Errorf("end date: %w", err)
	}
	if to.Before(from) {
		return "", "", errors.New("the sprint ends before it starts")
	}
	return sprintdate.Format(from), sprintdate.Format(to), nil
}
