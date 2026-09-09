package sprints_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/sprints"
)

// moveCall is one push the completion made: where it sent the cards and
// which cards they were.
type moveCall struct {
	target string
	keys   []string
}

// fakeBackend answers the issue search and the three Agile writes, and
// records the order it was asked in. Nothing else: the service reaches for
// the narrow lifecycle seam, so a fake does not have to answer for a board
// configuration to close a sprint.
type fakeBackend struct {
	issues    []backend.Issue
	scopes    []string
	searchErr error

	moves   []moveCall
	moveErr map[int]error

	starts   []backend.SprintDraft
	startErr error

	completed   []int
	completeErr error

	sprints     []backend.Sprint
	sprintReads int

	// order is what happened, in the order it happened, so "moved first and
	// closed second" is asserted rather than assumed.
	order []string
}

func (f *fakeBackend) SearchIssuesPage(_ context.Context, _, scopeJQL, _ string, _ []string, startAt, maxResults int) ([]backend.Issue, int, error) {
	if f.searchErr != nil {
		return nil, 0, f.searchErr
	}
	f.scopes = append(f.scopes, scopeJQL)
	total := len(f.issues)
	if startAt >= total {
		return []backend.Issue{}, total, nil
	}
	end := startAt + maxResults
	if end > total {
		end = total
	}
	return f.issues[startAt:end], total, nil
}

func (f *fakeBackend) MoveIssuesToSprint(_ context.Context, sprintID string, keys []string) error {
	n := len(f.moves)
	f.moves = append(f.moves, moveCall{target: sprintID, keys: append([]string{}, keys...)})
	f.order = append(f.order, "move")
	if err, ok := f.moveErr[n]; ok {
		return err
	}
	return nil
}

func (f *fakeBackend) StartSprint(_ context.Context, sprintID int, d backend.SprintDraft) error {
	f.order = append(f.order, "start")
	if f.startErr != nil {
		return f.startErr
	}
	f.starts = append(f.starts, d)
	_ = sprintID
	return nil
}

func (f *fakeBackend) CompleteSprint(_ context.Context, sprintID int) error {
	f.order = append(f.order, "complete")
	if f.completeErr != nil {
		return f.completeErr
	}
	f.completed = append(f.completed, sprintID)
	return nil
}

func (f *fakeBackend) BoardSprints(context.Context, int) ([]backend.Sprint, error) {
	f.sprintReads++
	return f.sprints, nil
}

// fakeStore is the board cache: the columns "complete" is defined against,
// the destination's name, and the two writes the ceremonies make.
type fakeStore struct {
	columns    []backend.BoardColumn
	names      map[string]string
	sprints    []backend.Sprint
	written    int
	membership map[string][]string
}

func newStore() *fakeStore {
	return &fakeStore{
		columns: []backend.BoardColumn{
			{Name: "To Do", StatusIDs: []string{"1"}},
			{Name: "In Progress", StatusIDs: []string{"3"}},
			{Name: "Done", StatusIDs: []string{"5", "6"}},
		},
		names:      map[string]string{"13": "Sprint 13"},
		membership: map[string][]string{},
	}
}

func (s *fakeStore) Columns(context.Context, string, int) ([]backend.BoardColumn, error) {
	return s.columns, nil
}

func (s *fakeStore) SprintName(_ context.Context, _, sprintID string) (string, error) {
	return s.names[sprintID], nil
}

func (s *fakeStore) ReplaceSprints(_ context.Context, _ string, _ int, list []backend.Sprint) error {
	s.sprints = list
	s.written++
	return nil
}

func (s *fakeStore) ReplaceSprintIssues(_ context.Context, _ string, _ int, sprintID string, keys []string) error {
	s.membership[sprintID] = append([]string{}, keys...)
	return nil
}

// issue is one card of the sprint being completed: its key, the status id
// that says whether it finished, and the sprint it reports.
func issue(key, statusID string) backend.Issue {
	return backend.Issue{Key: key, Summary: key, StatusID: statusID, SprintID: "12"}
}

// sprintOf builds the cards of sprint 12 from a list of status ids.
func sprintOf(statusIDs ...string) []backend.Issue {
	out := make([]backend.Issue, 0, len(statusIDs))
	for i, id := range statusIDs {
		out = append(out, issue(fmt.Sprintf("PLAT-%d", i+1), id))
	}
	return out
}

func newService(b *fakeBackend, store *fakeStore) *sprints.Service {
	return sprints.New(b, store, "PLAT")
}

// TestStartPassesTheDraftThroughWithJiraSDates is the whole of a start: the
// name and goal reach Jira untouched, the two bare dates a date input
// produced arrive in the Agile API's own datetime format, and the board's
// sprint list is re-read so the picker shows the sprint as active.
func TestStartPassesTheDraftThroughWithJiraSDates(t *testing.T) {
	b := &fakeBackend{sprints: []backend.Sprint{{ID: 13, BoardID: 1, Name: "Sprint 13", State: "active"}}}
	store := newStore()
	draft := backend.SprintDraft{Name: "Sprint 13", Goal: "Ship the board", StartDate: "2026-09-09", EndDate: "2026-09-23"}

	if err := newService(b, store).Start(context.Background(), "p1", 1, 13, draft); err != nil {
		t.Fatalf("start: %v", err)
	}
	if len(b.starts) != 1 {
		t.Fatalf("starts = %+v, want the one call", b.starts)
	}
	got := b.starts[0]
	if got.Name != "Sprint 13" || got.Goal != "Ship the board" {
		t.Errorf("draft = %+v, want the name and goal unchanged", got)
	}
	if !strings.HasPrefix(got.StartDate, "2026-09-09T09:00:00.000") || !strings.HasPrefix(got.EndDate, "2026-09-23T09:00:00.000") {
		t.Errorf("dates = %q and %q, want Jira's own datetime format", got.StartDate, got.EndDate)
	}
	if store.written != 1 || len(store.sprints) != 1 || store.sprints[0].State != "active" {
		t.Errorf("cached sprints = %+v after %d writes, want the started copy", store.sprints, store.written)
	}
}

// TestStartReturnsJiraSRefusalWordForWord is the rule the plan states for
// every lifecycle failure: TAM does not guess at permissions or at which
// sprint is already running, it attempts the action and shows what came
// back.
func TestStartReturnsJiraSRefusalWordForWord(t *testing.T) {
	const refusal = "Sprint 12 is already active on this board"
	b := &fakeBackend{startErr: errors.New(refusal)}
	store := newStore()

	err := newService(b, store).Start(context.Background(), "p1", 1, 13,
		backend.SprintDraft{Name: "Sprint 13", StartDate: "2026-09-09", EndDate: "2026-09-23"})
	if err == nil {
		t.Fatal("start = nil error, want Jira's refusal")
	}
	if err.Error() != refusal {
		t.Errorf("err = %q, want Jira's sentence unchanged", err.Error())
	}
	if store.written != 0 {
		t.Error("the sprint list was rewritten after a start that never happened")
	}
}

// TestStartRefusesDatesItCannotUse keeps two mistakes off the wire: a value
// that is not a date, and an end before a start. The second check lives in
// the service and not in the dialog because the bound method is reachable
// without the dialog.
func TestStartRefusesDatesItCannotUse(t *testing.T) {
	for _, tc := range []struct {
		name       string
		start, end string
		want       string
	}{
		{"a start that is not a date", "next tuesday", "2026-09-23", "not a date"},
		{"no end at all", "2026-09-09", "", "missing"},
		{"an end before the start", "2026-09-23", "2026-09-09", "ends before it starts"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := &fakeBackend{}
			err := newService(b, newStore()).Start(context.Background(), "p1", 1, 13,
				backend.SprintDraft{Name: "Sprint 13", StartDate: tc.start, EndDate: tc.end})
			if err == nil {
				t.Fatal("start = nil error, want a refusal")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to say %q", err, tc.want)
			}
			if len(b.starts) != 0 || len(b.order) != 0 {
				t.Error("Jira was called with dates the service should have refused")
			}
		})
	}
}

// TestCompleteMovesTheUnfinishedCardsThenCloses is the ceremony itself: the
// three cards no column of the last kind collects go to the backlog, the two
// that finished stay, and the sprint closes afterwards. The order is the
// point: closing first would leave a closed sprint whose issues went
// nowhere, which nobody can undo from TAM.
func TestCompleteMovesTheUnfinishedCardsThenCloses(t *testing.T) {
	b := &fakeBackend{issues: sprintOf("1", "5", "3", "6", "1")}
	store := newStore()

	done, err := newService(b, store).Complete(context.Background(), "p1", 1, 12, "")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if done.Moved != 3 || done.MovedTo != "the backlog" || len(done.Failed) != 0 {
		t.Errorf("completion = %+v, want three cards moved to the backlog and nothing failed", done)
	}
	if len(b.moves) != 1 {
		t.Fatalf("moves = %+v, want one push", b.moves)
	}
	if b.moves[0].target != "" || strings.Join(b.moves[0].keys, ",") != "PLAT-1,PLAT-3,PLAT-5" {
		t.Errorf("push = %+v, want the three unfinished cards onto the backlog", b.moves[0])
	}
	if strings.Join(b.order, ",") != "move,complete" {
		t.Errorf("order = %v, want the move before the close", b.order)
	}
	if len(b.completed) != 1 || b.completed[0] != 12 {
		t.Errorf("completed = %v, want sprint 12 closed", b.completed)
	}
	if len(b.scopes) == 0 || b.scopes[0] != "sprint = 12" {
		t.Errorf("search scope = %v, want the sprint read from Jira rather than the cache", b.scopes)
	}
	// The two that finished are all the sprint still holds.
	if got := strings.Join(store.membership["12"], ","); got != "PLAT-2,PLAT-4" {
		t.Errorf("cached membership = %q, want only the cards that stayed", got)
	}
}

// TestCompleteMovesToTheNamedSprint is the other destination the dialog
// offers, and it is reported by name because that is what the user chose.
func TestCompleteMovesToTheNamedSprint(t *testing.T) {
	b := &fakeBackend{issues: sprintOf("1", "5")}
	store := newStore()

	done, err := newService(b, store).Complete(context.Background(), "p1", 1, 12, "13")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if done.Moved != 1 || done.MovedTo != "Sprint 13" {
		t.Errorf("completion = %+v, want the one card moved to Sprint 13 by name", done)
	}
	if len(b.moves) != 1 || b.moves[0].target != "13" {
		t.Errorf("push = %+v, want it aimed at sprint 13", b.moves)
	}
}

// TestCompleteWithNothingUnfinishedClosesWithoutAMove is the sprint every
// team wants: no push at all, because there is nothing to push, and no
// membership rewrite either, because nothing left the sprint.
func TestCompleteWithNothingUnfinishedClosesWithoutAMove(t *testing.T) {
	b := &fakeBackend{issues: sprintOf("5", "6", "5")}
	store := newStore()

	done, err := newService(b, store).Complete(context.Background(), "p1", 1, 12, "")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if done.Moved != 0 || len(done.Failed) != 0 {
		t.Errorf("completion = %+v, want nothing moved and nothing failed", done)
	}
	if len(b.moves) != 0 {
		t.Errorf("moves = %+v, want Jira asked to move nothing", b.moves)
	}
	if strings.Join(b.order, ",") != "complete" {
		t.Errorf("order = %v, want the close on its own", b.order)
	}
	if _, rewritten := store.membership["12"]; rewritten {
		t.Error("the sprint's membership was rewritten though no card left it")
	}
}

// TestCompleteWhoseMoveFailsLeavesTheSprintOpen is the destructive case. The
// push fails, so the sprint is never closed, the completion says so, and the
// error carries the count with it: the dialog has to be able to tell the
// user what did happen before it tells them what did not.
func TestCompleteWhoseMoveFailsLeavesTheSprintOpen(t *testing.T) {
	b := &fakeBackend{
		issues:  sprintOf("1", "3", "1"),
		moveErr: map[int]error{0: errors.New("403 Forbidden")},
	}
	store := newStore()

	done, err := newService(b, store).Complete(context.Background(), "p1", 1, 12, "")
	if err == nil {
		t.Fatal("complete = nil error, want the failed push reported")
	}
	if !strings.Contains(err.Error(), "403 Forbidden") || !strings.Contains(err.Error(), "left open") {
		t.Errorf("err = %v, want Jira's reason and the fact the sprint is still open", err)
	}
	if done.Moved != 0 || len(done.Failed) != 3 {
		t.Errorf("completion = %+v, want nothing moved and all three named", done)
	}
	if len(b.completed) != 0 {
		t.Errorf("sprint %v was closed after a move that failed", b.completed)
	}
	if _, rewritten := store.membership["12"]; rewritten {
		t.Error("the membership was rewritten though no card left the sprint")
	}
}

// TestASecondChunkThatFailsReportsWhatMovedAndCorrectsTheCache is the
// half-finished completion, at chunks of three. The first chunk lands, the
// second is refused, and everything from it on is still in the sprint: the
// count says three of seven, the sprint stays open, and the cache is
// corrected before the user is told, or they are left with cards that
// vanished from an open sprint with nothing recording where they went.
func TestASecondChunkThatFailsReportsWhatMovedAndCorrectsTheCache(t *testing.T) {
	b := &fakeBackend{
		issues:  sprintOf("1", "1", "1", "1", "1", "1", "5"),
		moveErr: map[int]error{1: errors.New("500 Internal Server Error")},
	}
	store := newStore()
	s := newService(b, store)
	s.PushBatch = 3

	done, err := s.Complete(context.Background(), "p1", 1, 12, "")
	if err == nil {
		t.Fatal("complete = nil error, want the failed chunk reported")
	}
	if !strings.Contains(err.Error(), "3 of 6") {
		t.Errorf("err = %v, want it to say how many of the unfinished cards moved", err)
	}
	if done.Moved != 3 || len(done.Failed) != 3 {
		t.Errorf("completion = %+v, want three moved and the other three named", done)
	}
	if strings.Join(done.Failed, ",") != "PLAT-4,PLAT-5,PLAT-6" {
		t.Errorf("failed = %v, want the failing chunk and everything after it", done.Failed)
	}
	if len(b.completed) != 0 {
		t.Errorf("sprint %v was closed after a chunk that failed", b.completed)
	}
	if len(b.moves) != 2 {
		t.Errorf("moves = %+v, want the pass to stop at the chunk that failed", b.moves)
	}
	// The three that landed have gone; the three that did not, and the card
	// that had finished, are what the sprint still holds.
	if got := strings.Join(store.membership["12"], ","); got != "PLAT-4,PLAT-5,PLAT-6,PLAT-7" {
		t.Errorf("cached membership = %q, want the cards that are still in the sprint", got)
	}
}

// TestCompleteChunksAtTwenty pins the width, since it is what one refusal
// can take down: Jira answers a partial refusal with a 207 that names issues
// by numeric id, which cannot be mapped back to keys, so a whole batch fails
// together.
func TestCompleteChunksAtTwenty(t *testing.T) {
	statuses := make([]string, 45)
	for i := range statuses {
		statuses[i] = "1"
	}
	b := &fakeBackend{issues: sprintOf(statuses...)}

	done, err := newService(b, newStore()).Complete(context.Background(), "p1", 1, 12, "")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if done.Moved != 45 {
		t.Errorf("moved = %d, want all forty five", done.Moved)
	}
	if len(b.moves) != 3 || len(b.moves[0].keys) != 20 || len(b.moves[2].keys) != 5 {
		t.Errorf("pushes = %d chunks (%d, %d, %d), want twenty at a time",
			len(b.moves), len(b.moves[0].keys), len(b.moves[1].keys), len(b.moves[2].keys))
	}
}

// TestCompleteReadsEveryPageOfTheSprint keeps a sprint bigger than one page
// from being half completed: the cards on page two are just as unfinished as
// the ones on page one.
func TestCompleteReadsEveryPageOfTheSprint(t *testing.T) {
	statuses := make([]string, 7)
	for i := range statuses {
		statuses[i] = "1"
	}
	b := &fakeBackend{issues: sprintOf(statuses...)}
	s := newService(b, newStore())
	s.PageSize = 3

	done, err := s.Complete(context.Background(), "p1", 1, 12, "")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if done.Moved != 7 {
		t.Errorf("moved = %d, want every page's cards", done.Moved)
	}
	if len(b.scopes) != 3 {
		t.Errorf("search calls = %d, want the sprint paged to its end", len(b.scopes))
	}
}

// TestCompleteRefusesWhileTheJournalHoldsCardsStayingInTheSprint is the
// user's own uncommitted work: a card dragged to Done an hour ago is Done on
// the board and not in Jira, and completing the sprint would move it to the
// backlog as unfinished. The toolbar asks the same question before it opens
// the dialog, but the board's writes are unguarded, so a drag can land
// between that answer and this one.
func TestCompleteRefusesWhileTheJournalHoldsCardsStayingInTheSprint(t *testing.T) {
	b := &fakeBackend{issues: sprintOf("1", "5")}
	s := newService(b, newStore())
	s.Pending = func(context.Context, string, int) (int, error) { return 2, nil }

	_, err := s.Complete(context.Background(), "p1", 1, 12, "")
	if err == nil {
		t.Fatal("complete = nil error, want it refused while changes are pending")
	}
	if !strings.Contains(err.Error(), "commit") {
		t.Errorf("err = %v, want it to name Commit as the thing to do", err)
	}
	if len(b.order) != 0 {
		t.Errorf("Jira was called (%v) though the completion should have been refused", b.order)
	}
}

// TestCompleteRefusesADestinationItCannotUse covers the two answers that
// would be wrong before anything moved: an id that is not a number, which
// ends up in a URL path, and the sprint completing into itself.
func TestCompleteRefusesADestinationItCannotUse(t *testing.T) {
	for _, moveTo := range []string{"fourteen", "12"} {
		b := &fakeBackend{issues: sprintOf("1")}
		if _, err := newService(b, newStore()).Complete(context.Background(), "p1", 1, 12, moveTo); err == nil {
			t.Errorf("complete into %q = nil error, want a refusal", moveTo)
		}
		if len(b.order) != 0 {
			t.Errorf("Jira was called for the destination %q", moveTo)
		}
	}
}

// TestCompleteRefusesABoardItCannotJudge is the definition of unfinished,
// enforced: without the board's columns TAM cannot tell which cards
// finished, and guessing decides where somebody's work goes.
func TestCompleteRefusesABoardItCannotJudge(t *testing.T) {
	for _, cols := range [][]backend.BoardColumn{
		nil,
		{{Name: "To Do", StatusIDs: []string{"1"}}, {Name: "Backlog", StatusIDs: []string{}}},
	} {
		b := &fakeBackend{issues: sprintOf("1")}
		store := newStore()
		store.columns = cols
		if _, err := newService(b, store).Complete(context.Background(), "p1", 1, 12, ""); err == nil {
			t.Errorf("complete with columns %+v = nil error, want a refusal", cols)
		}
		if len(b.order) != 0 {
			t.Error("Jira was called for a board whose columns cannot say what finished")
		}
	}
}

// TestCompleteLeavesACardThatIsNoLongerInTheSprintAlone is the narrowing the
// service does on top of the query. It can only ever move fewer cards than
// the search returned, never more, which is the right direction for a move
// nobody can undo from TAM.
func TestCompleteLeavesACardThatIsNoLongerInTheSprintAlone(t *testing.T) {
	elsewhere := issue("PLAT-9", "1")
	elsewhere.SprintID = "13"
	b := &fakeBackend{issues: append(sprintOf("1"), elsewhere)}

	done, err := newService(b, newStore()).Complete(context.Background(), "p1", 1, 12, "")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if done.Moved != 1 || len(b.moves) != 1 || strings.Join(b.moves[0].keys, ",") != "PLAT-1" {
		t.Errorf("push = %+v, want only the card the sprint actually holds", b.moves)
	}
}

// TestABackendWithNoAgileApiRefusesBothCeremonies is the door a future
// read-only backend comes through. It says nothing about permissions: TAM
// does not guess at those before trying.
func TestABackendWithNoAgileApiRefusesBothCeremonies(t *testing.T) {
	s := sprints.New(searchOnly{}, newStore(), "PLAT")
	if err := s.Start(context.Background(), "p1", 1, 13, backend.SprintDraft{StartDate: "2026-09-09", EndDate: "2026-09-23"}); err == nil {
		t.Error("start = nil error, want a backend with no Agile API refused")
	}
	if _, err := s.Complete(context.Background(), "p1", 1, 12, ""); err == nil {
		t.Error("complete = nil error, want a backend with no Agile API refused")
	}
}

// searchOnly can search and nothing else, which is what a backend that
// cannot speak Jira's Agile API looks like from here.
type searchOnly struct{}

func (searchOnly) SearchIssuesPage(context.Context, string, string, string, []string, int, int) ([]backend.Issue, int, error) {
	return []backend.Issue{}, 0, nil
}

// TestTheSearchIsScopedToTheSprintAndTheProject keeps the completion's read
// off the whole project: one paged query, narrowed by sprint, which is what
// brings the status back with the key instead of one call per card.
func TestTheSearchIsScopedToTheSprintAndTheProject(t *testing.T) {
	b := &fakeBackend{issues: sprintOf("1")}
	if _, err := newService(b, newStore()).Complete(context.Background(), "p1", 1, 12, ""); err != nil {
		t.Fatalf("complete: %v", err)
	}
	for _, scope := range b.scopes {
		if scope != "sprint = "+strconv.Itoa(12) {
			t.Errorf("scope = %q, want the sprint's own query", scope)
		}
	}
}
