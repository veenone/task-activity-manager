package sprints

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/donerule"
	"agile-suite/tam/internal/sprintdate"
)

// The checks a ceremony makes before it touches Jira, gathered off
// sprints.go so the two ceremonies read as what they do rather than as what
// they refuse. Every one of them exists because the bound method is
// reachable without the dialog that would have asked the same question.

// destinationID reads the destination the dialog sent: a sprint id, or the
// empty string for the backlog, which is a destination and not an absence.
//
// The id ends up in a URL path, so only a plain positive number is a sprint
// id here. strconv.Atoi on its own takes "+13", "-1", "0" and "012", and
// "012" then walked past a string comparison with the sprint being completed
// and went to Jira as it was typed. Comparing the numbers is what catches
// that, and refusing everything but the canonical form is what keeps the
// value that reaches Jira the one that was checked.
func destinationID(moveTo string, sprintID int) (string, error) {
	moveTo = strings.TrimSpace(moveTo)
	if moveTo == "" {
		return "", nil
	}
	n, err := strconv.Atoi(moveTo)
	if err != nil || n <= 0 || strconv.Itoa(n) != moveTo {
		return "", fmt.Errorf("sprint id %q is not a sprint id", moveTo)
	}
	if n == sprintID {
		return "", errors.New("a sprint cannot be completed into itself")
	}
	return moveTo, nil
}

// requireCompletable is what the cache has to say about the sprint before a
// completion touches Jira: the board holds it, and it is not one that has
// never been started.
//
// The pair first. The completion judges "finished" against the board's last
// column while the cards come from the sprint, so a mismatched pair would
// decide where somebody's work goes on a definition borrowed from a board
// the sprint was never on, and close the sprint anyway.
//
// Then the state, because a completion moves the cards out before it finds
// out Jira will not close the sprint. Aimed at a sprint that never started,
// it empties that sprint in Jira and then fails, and TAM cannot undo either
// half. The toolbar only offers Complete on the active sprint, which is the
// same argument the other guards here already refuse to rest on: the bound
// method is reachable without the dialog.
//
// Only "future" is refused, and deliberately not "anything that is not
// active". A sprint someone started on the web an hour ago still reads as
// future in a cache nobody has refreshed since, and refusing that costs a
// Refresh, where emptying it costs the sprint. A state the cache does not
// recognise is left to Jira to answer for.
func (s *Service) requireCompletable(ctx context.Context, profileID string, boardID int, sprintID string) error {
	state, ok, err := s.store.BoardSprintState(ctx, profileID, boardID, sprintID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("sprint %s is not on board %d, so that board's last column cannot say which of the sprint's cards finished; complete the sprint from the board it belongs to", sprintID, boardID)
	}
	if state == "future" {
		return fmt.Errorf("sprint %s has not been started, and completing it would move its cards out and then fail to close it; start it first, or press Refresh if it was started somewhere else", sprintID)
	}
	return nil
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
	n, err := s.pendingInSprint(ctx, profileID, sprintID)
	if err != nil || n == 0 {
		return err
	}
	return fmt.Errorf("%d pending change(s) belong to cards in this sprint; commit them before completing it, or Jira will be asked which cards finished before it has been told", n)
}

// completeStatuses is what counts as finished for this completion: the rule
// donerule builds from the board's last column, which the sprint report
// reconstructs its burndown with too. A board whose columns are not cached,
// or whose last column collects nothing, cannot answer the question this
// action turns on, and guessing is not an option for a move nobody can undo
// from TAM. donerule says only that it cannot answer; naming which of the
// two happened is this function's job, because refreshing the boards fixes
// one of them and nothing a user can do here fixes the other.
func (s *Service) completeStatuses(ctx context.Context, profileID string, boardID int) (func(string) bool, error) {
	cols, err := s.store.Columns(ctx, profileID, boardID)
	if err != nil {
		return nil, err
	}
	last, ok := donerule.LastColumn(cols)
	if !ok {
		return nil, fmt.Errorf("board %d has no cached columns, so TAM cannot tell which issues finished; refresh the boards first", boardID)
	}
	done := donerule.Done(cols)
	if done == nil {
		return nil, fmt.Errorf("the last column of board %d (%q) collects no statuses, so TAM cannot tell which issues finished", boardID, last.Name)
	}
	return done, nil
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

// What the management writes refuse: two states read from Jira rather than
// from the cache, and one count of the journal.

// jiraSprint is what Jira says about one sprint of a board right now,
// deliberately read over the wire rather than out of the board cache.
//
// requireCompletable above reads the cache, and when that cache is stale it
// refuses too much: a sprint started on the web an hour ago still reads
// future locally, and a completion refused over that costs a Refresh. The
// two guards built on this read would permit too much instead. A delete
// would destroy a running sprint and an edit would rewrite a closed one's
// dates, which is what velocity and burndown are computed from, so both pay
// a round trip they cannot get back rather than trust a list that may be
// minutes old.
//
// Whether Jira refuses a closed edit on its own is still an open question
// (docs/superpowers/plans/assets/2026-09-10-sprint-wire-probe.md, probe 4);
// until it is answered, this read is the only thing standing there.
//
// An empty list is an error and not an answer. core/jira turns any 400 on
// the first page of the sprint endpoint into ErrNoSprints, for the kanban
// board that genuinely has none, and the backend turns that into an empty
// slice and no error. So from here one flaky 400 is indistinguishable from
// "this board has no sprints", and a board being asked about a sprint of its
// own demonstrably has one. Reading it as an absence is what would tell a
// user at 2am that their sprint was already deleted, and then remove TAM's
// copy of a sprint still running in Jira.
//
// The second return says whether the list held the sprint. False with no
// error is a real absence in a real list, which each caller answers for
// itself.
func (s *Service) jiraSprint(ctx context.Context, b lifecycle, boardID, sprintID int) (backend.Sprint, bool, error) {
	list, err := b.BoardSprints(ctx, boardID)
	if err != nil {
		return backend.Sprint{}, false, fmt.Errorf("board %d's sprints could not be read, so TAM cannot tell what state sprint %d is in and changed nothing: %w", boardID, sprintID, err)
	}
	if len(list) == 0 {
		return backend.Sprint{}, false, fmt.Errorf("board %d answered with no sprints at all, which cannot be true of the board sprint %d belongs to, so nothing was changed; try again, and press Refresh if it keeps happening", boardID, sprintID)
	}
	for _, sp := range list {
		if sp.ID == sprintID {
			return sp, true, nil
		}
	}
	return backend.Sprint{}, false, nil
}

// requireEditable is what Jira has to say about a sprint before its name,
// goal or dates are rewritten: the board lists it, and it is not closed.
//
// A closed sprint is refused because its dates are what velocity and
// burndown are computed from, and moving them changes charts nobody is
// looking at. A sprint the board's list does not hold is refused too, since
// a state nobody could read is not a state that was checked; the sentence
// says which of the two happened, because pressing Refresh is the answer to
// one of them and not the other.
func (s *Service) requireEditable(ctx context.Context, b lifecycle, boardID, sprintID int) (backend.Sprint, error) {
	sp, held, err := s.jiraSprint(ctx, b, boardID, sprintID)
	if err != nil {
		return backend.Sprint{}, err
	}
	if !held {
		return backend.Sprint{}, fmt.Errorf("board %d no longer lists sprint %d, so TAM cannot tell whether it is closed and did not change it; press Refresh", boardID, sprintID)
	}
	if sprintState(sp) == "closed" {
		return backend.Sprint{}, fmt.Errorf("sprint %d is closed, and its dates are what velocity and burndown are computed from, so TAM does not edit it", sprintID)
	}
	return sp, nil
}

// requireDeletable tells apart what a board's own answer can mean for the
// one action in TAM that cannot be undone.
//
// A sprint the board holds as future is deleted. A sprint it holds in any
// other state is refused by name, because deleting an active sprint strands
// work a team is doing right now and deleting a closed one destroys the
// record a chart is drawn from. A sprint the board's own list does not hold
// is treated as already gone, which is a success: ordinarily that means
// somebody deleted it elsewhere, and all that is left to do is remove TAM's
// own copy.
//
// "Ordinarily" is doing real work in that sentence, and this comment used to
// pretend it was not: the read above is board scoped, one board's
// BoardSprints, while what a hit does next is not. forget below removes the
// sprint from every board's cached rows and blanks it off every cached issue,
// because Jira hands one sprint to every board whose filter reaches it and a
// board scoped purge would leave another board's copy standing, an argument
// boardrepo's own comment makes for DeleteSprintEverywhere. So a sprint board
// 1's filter no longer reaches, while board 2 still lists it and Jira still
// holds it, also reads as already gone from here: this method cannot tell
// that case apart from a real deletion, and the delete purges board 2's cache
// too and tells the user the sprint was gone before TAM asked, which is false
// about Jira even though it was true of what board 1 could see.
//
// That does not destroy anything in Jira: DeleteSprint is skipped exactly as
// it is for a real already-gone sprint, since there is nothing this method
// believes needs deleting, and a boards sync repairs whichever board's cache
// was purged too early. That is why purging stays the answer here instead of
// growing a second, board scoped case for it: the only real cost is a
// sentence that oversells its own certainty, not anything TAM cannot recover
// from with a Refresh.
//
// A fourth case a real Jira read can produce, a list with nothing in it at
// all, never reaches here: jiraSprint refuses it outright, and its own
// comment says why that refusal is the whole point.
func (s *Service) requireDeletable(ctx context.Context, b lifecycle, boardID, sprintID int) (backend.Sprint, bool, error) {
	sp, held, err := s.jiraSprint(ctx, b, boardID, sprintID)
	if err != nil {
		return backend.Sprint{}, false, err
	}
	if !held {
		return backend.Sprint{}, true, nil
	}
	if state := sprintState(sp); state != "future" {
		return backend.Sprint{}, false, fmt.Errorf("sprint %d is %s, and only a sprint that has never been started can be deleted; complete it instead, or press Refresh if TAM still shows it as future", sprintID, statePhrase(state))
	}
	return sp, false, nil
}

// sprintState is the state Jira reported, in the lower case the cache stores
// and the rest of this package compares against.
func sprintState(sp backend.Sprint) string {
	return strings.ToLower(strings.TrimSpace(sp.State))
}

// statePhrase is how a refusal names that state, since a sprint Jira
// reported no state for still has to read as a sentence.
func statePhrase(state string) string {
	if state == "" {
		return "in a state Jira did not name"
	}
	return state
}

// refusePendingDelete stops a delete while the journal still holds changes
// for cards in the sprint. Those rows point at a sprint that is about to
// stop existing, and Commit is the one thing that resolves them either way.
//
// It is a check and not a lock, and it cannot be made into one: the board's
// own writes are journaled and deliberately take no lock, so a card can be
// dragged into this sprint between this answer and the delete a moment
// later. That is rare and it is recoverable: the stranded row is pushed at a
// sprint Jira no longer has, the next Commit reports exactly that, and the
// user discards it from the Pending changes dialog.
func (s *Service) refusePendingDelete(ctx context.Context, profileID string, sprintID int) error {
	n, err := s.pendingInSprint(ctx, profileID, sprintID)
	if err != nil || n == 0 {
		return err
	}
	return fmt.Errorf("%d pending change(s) belong to cards in this sprint; commit them before deleting it, or they will be pushed at a sprint that no longer exists", n)
}

// pendingInSprint is how many journal rows belong to cards staying in the
// sprint, and zero when nothing is wired to answer. Both refusals above ask
// it the same question and word the answer for their own action.
func (s *Service) pendingInSprint(ctx context.Context, profileID string, sprintID int) (int, error) {
	if s.Pending == nil {
		return 0, nil
	}
	return s.Pending(ctx, profileID, sprintID)
}
