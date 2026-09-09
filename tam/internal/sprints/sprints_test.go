package sprints_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
	demobackend "agile-suite/tam/internal/backend/demo"
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
	projects  []string
	searchErr error

	// ignoreScope answers every search with every issue, whatever the query
	// asked for, which is what a backend that does not honour the scope
	// looks like from here. The default narrows, the way Jira and the demo
	// backend both do, so a test meaning to exercise the service's own
	// second narrowing has to ask for a backend that does not.
	ignoreScope bool

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

func (f *fakeBackend) SearchIssuesPage(_ context.Context, projectKey, scopeJQL, _ string, _ []string, startAt, maxResults int) ([]backend.Issue, int, error) {
	if f.searchErr != nil {
		return nil, 0, f.searchErr
	}
	f.scopes = append(f.scopes, scopeJQL)
	f.projects = append(f.projects, projectKey)
	all := f.issues
	if !f.ignoreScope {
		all = inScope(f.issues, scopeJQL)
	}
	total := len(all)
	if startAt >= total {
		return []backend.Issue{}, total, nil
	}
	end := startAt + maxResults
	if end > total {
		end = total
	}
	return all[startAt:end], total, nil
}

// inScope is the narrowing the query itself does on a backend that honours
// it: "sprint = N" comes back with that sprint's cards and no others. The
// service leans on exactly this when it keeps an issue the backend reports
// no sprint for, so a fake that answered with the whole project would let a
// completion that moves the backlog pass every test here.
func inScope(issues []backend.Issue, scopeJQL string) []backend.Issue {
	id := strings.TrimPrefix(scopeJQL, "sprint = ")
	out := make([]backend.Issue, 0, len(issues))
	for _, iss := range issues {
		if iss.SprintID == id {
			out = append(out, iss)
		}
	}
	return out
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
	onBoard    map[string]bool
	cached     map[string][]string
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
		onBoard:    map[string]bool{"1/12": true, "1/13": true},
		cached:     map[string][]string{},
		membership: map[string][]string{},
	}
}

// inSprint seeds the board's own cached membership of sprint 12, in the rank
// order the view reads it back in. That is what a completion subtracts the
// cards it moved from, and it is deliberately not the order the search
// answers in.
func (s *fakeStore) inSprint(keys ...string) {
	s.holds("12", keys...)
}

// holds seeds one scope of the board's cached membership: a sprint by id, or
// the board's own list under the empty id, which is the scope a completion
// into the backlog writes back to.
func (s *fakeStore) holds(scopeID string, keys ...string) {
	s.cached[scopeID] = keys
}

func (s *fakeStore) Columns(context.Context, string, int) ([]backend.BoardColumn, error) {
	return s.columns, nil
}

func (s *fakeStore) SprintName(_ context.Context, _, sprintID string) (string, error) {
	return s.names[sprintID], nil
}

func (s *fakeStore) BoardHasSprint(_ context.Context, _ string, boardID int, sprintID string) (bool, error) {
	return s.onBoard[fmt.Sprintf("%d/%s", boardID, sprintID)], nil
}

func (s *fakeStore) SprintIssues(_ context.Context, _ string, _ int, sprintID string) ([]string, error) {
	return append([]string{}, s.cached[sprintID]...), nil
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
	store.inSprint("PLAT-1", "PLAT-2", "PLAT-3", "PLAT-4", "PLAT-5")

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
// push fails, so the sprint is never closed, and the completion says so in
// its own Message rather than in a Go error: Wails would drop the value the
// keys travel in. The dialog has to be able to tell the user what did happen
// before it tells them what did not.
func TestCompleteWhoseMoveFailsLeavesTheSprintOpen(t *testing.T) {
	b := &fakeBackend{
		issues:  sprintOf("1", "3", "1"),
		moveErr: map[int]error{0: errors.New("403 Forbidden")},
	}
	store := newStore()

	done, err := newService(b, store).Complete(context.Background(), "p1", 1, 12, "")
	if err != nil {
		t.Fatalf("complete = %v, want the failed push carried in the completion instead", err)
	}
	if !strings.Contains(done.Message, "403 Forbidden") || !strings.Contains(done.Message, "left open") {
		t.Errorf("message = %q, want Jira's reason and the fact the sprint is still open", done.Message)
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

// TestAMiddleChunkThatFailsReportsWhatMovedAndCorrectsTheCache is the
// half-finished completion, at chunks of three. The first chunk lands, the
// second is refused, and everything from it on is still in the sprint: the
// count says three of nine, the sprint stays open, and the cache is
// corrected before the user is told, or they are left with cards that
// vanished from an open sprint with nothing recording where they went.
//
// Nine unfinished cards at a width of three, so the refusal lands in the
// middle chunk and every assertion here carries its own weight. At seven
// cards, six of them unfinished, the failure was in the last chunk of two:
// "everything from the failure on" and "the failing chunk" were the same
// three keys, so a slice bound that stopped at the chunk read exactly like
// one that ran to the end, and "two pushes" could not tell a pass that
// stopped from a pass that ran out of chunks.
func TestAMiddleChunkThatFailsReportsWhatMovedAndCorrectsTheCache(t *testing.T) {
	b := &fakeBackend{
		issues:  sprintOf("1", "1", "1", "1", "1", "1", "1", "1", "1", "5"),
		moveErr: map[int]error{1: errors.New("500 Internal Server Error")},
	}
	store := newStore()
	store.inSprint("PLAT-1", "PLAT-2", "PLAT-3", "PLAT-4", "PLAT-5",
		"PLAT-6", "PLAT-7", "PLAT-8", "PLAT-9", "PLAT-10")
	s := newService(b, store)
	s.PushBatch = 3

	done, err := s.Complete(context.Background(), "p1", 1, 12, "")
	if err != nil {
		t.Fatalf("complete = %v, want the failed chunk carried in the completion instead", err)
	}
	if !strings.Contains(done.Message, "3 of 9") {
		t.Errorf("message = %q, want it to say how many of the unfinished cards moved", done.Message)
	}
	if done.Moved != 3 || len(done.Failed) != 6 {
		t.Errorf("completion = %+v, want three moved and the other six named", done)
	}
	// The chunk that failed and the chunk after it, which was never
	// attempted: both are still in the sprint, and the failing chunk alone
	// would be a report that loses three cards.
	if strings.Join(done.Failed, ",") != "PLAT-4,PLAT-5,PLAT-6,PLAT-7,PLAT-8,PLAT-9" {
		t.Errorf("failed = %v, want the failing chunk and everything after it", done.Failed)
	}
	if len(b.completed) != 0 {
		t.Errorf("sprint %v was closed after a chunk that failed", b.completed)
	}
	// Two of the three chunks, so this says the pass stopped rather than that
	// it ran to the end.
	if len(b.moves) != 2 {
		t.Errorf("moves = %+v, want the pass to stop at the chunk that failed", b.moves)
	}
	// The three that landed have gone; the six that did not, and the card
	// that had finished, are what the sprint still holds.
	if got := strings.Join(store.membership["12"], ","); got != "PLAT-4,PLAT-5,PLAT-6,PLAT-7,PLAT-8,PLAT-9,PLAT-10" {
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
	for _, moveTo := range []string{"fourteen", "12", "012", "+13", "-1", "0", "13.0"} {
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
// service does on top of the query, against a backend that answers the
// query with more than it asked for. It can only ever move fewer cards than
// the search returned, never more, which is the right direction for a move
// nobody can undo from TAM.
func TestCompleteLeavesACardThatIsNoLongerInTheSprintAlone(t *testing.T) {
	elsewhere := issue("PLAT-9", "1")
	elsewhere.SprintID = "13"
	b := &fakeBackend{issues: append(sprintOf("1"), elsewhere), ignoreScope: true}

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
	if len(b.scopes) == 0 {
		t.Fatal("the sprint was never read")
	}
	for _, scope := range b.scopes {
		if scope != "sprint = "+strconv.Itoa(12) {
			t.Errorf("scope = %q, want the sprint's own query", scope)
		}
	}
	for _, project := range b.projects {
		if project != "PLAT" {
			t.Errorf("project = %q, want the profile's own project", project)
		}
	}
}

// TestARefreshThatComesBackEmptyLeavesTheBoardsSprintsAlone is the silent
// corruption a single 400 used to cause. core/jira turns any 400 on the
// sprint endpoint's first page into ErrNoSprints, for the kanban board that
// genuinely has none, and the Jira backend turns that into an empty slice
// and no error. Handing that to ReplaceSprints, which deletes the board's
// sprint rows before it inserts, emptied the picker and dropped the sprint
// length the date suggestion is built from, with nothing reported anywhere
// because the call did not fail. A board a ceremony has just run on
// demonstrably has a sprint, so an empty answer is not written.
func TestARefreshThatComesBackEmptyLeavesTheBoardsSprintsAlone(t *testing.T) {
	history := []backend.Sprint{
		{ID: 11, BoardID: 1, Name: "Sprint 11", State: "closed"},
		{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active"},
	}
	for _, tc := range []struct {
		name string
		run  func(s *sprints.Service) error
	}{
		{"after a completion", func(s *sprints.Service) error {
			_, err := s.Complete(context.Background(), "p1", 1, 12, "")
			return err
		}},
		{"after a start", func(s *sprints.Service) error {
			return s.Start(context.Background(), "p1", 1, 13,
				backend.SprintDraft{Name: "Sprint 13", StartDate: "2026-09-09", EndDate: "2026-09-23"})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := &fakeBackend{issues: sprintOf("5"), sprints: []backend.Sprint{}}
			store := newStore()
			store.sprints = append([]backend.Sprint{}, history...)

			if err := tc.run(newService(b, store)); err != nil {
				t.Fatalf("ceremony: %v", err)
			}
			if b.sprintReads != 1 {
				t.Errorf("sprint reads = %d, want the one re-read", b.sprintReads)
			}
			if store.written != 0 {
				t.Error("the board's sprint list was rewritten from an empty answer, which empties the picker and the sprint length with it")
			}
			if len(store.sprints) != len(history) {
				t.Errorf("cached sprints = %+v, want the board's history left as it was", store.sprints)
			}
		})
	}
}

// TestTheMembershipRewriteKeepsTheBoardsOrderAndItsScope is the other silent
// corruption. The completion's search ends ORDER BY key ASC and is scoped by
// project and issue type, while the cached membership is the board's rank
// order drawn from the board's own filter. Writing the search's answer back
// alphabetized the sprint until the next boards sync and could insert a key
// the board never drew, so what stays is the cached scope minus the cards
// that moved.
func TestTheMembershipRewriteKeepsTheBoardsOrderAndItsScope(t *testing.T) {
	// PLAT-2 is the one unfinished card. PLAT-9 is in the sprint and in the
	// search's answer, but the board's filter does not draw it.
	b := &fakeBackend{issues: append(sprintOf("5", "1", "5"), issue("PLAT-9", "5"))}
	store := newStore()
	store.inSprint("PLAT-3", "PLAT-2", "PLAT-1")

	done, err := newService(b, store).Complete(context.Background(), "p1", 1, 12, "")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if done.Moved != 1 {
		t.Fatalf("completion = %+v, want the one unfinished card moved", done)
	}
	// Rank order, not key order, and without the card the board never drew.
	if got := strings.Join(store.membership["12"], ","); got != "PLAT-3,PLAT-1" {
		t.Errorf("cached membership = %q, want the board's own scope in the board's own order", got)
	}
}

// TestCompleteRefusesASprintThatIsNotOnTheBoard is the pair nothing used to
// check. "Finished" is judged against the board's last column while the
// cards come from the sprint, so a board and a sprint with nothing to do
// with each other silently decided where somebody's work went, and the close
// landed anyway.
func TestCompleteRefusesASprintThatIsNotOnTheBoard(t *testing.T) {
	b := &fakeBackend{issues: sprintOf("1")}
	store := newStore()

	_, err := newService(b, store).Complete(context.Background(), "p1", 7, 12, "")
	if err == nil {
		t.Fatal("complete = nil error, want a sprint that is not on the board refused")
	}
	if !strings.Contains(err.Error(), "12") || !strings.Contains(err.Error(), "7") {
		t.Errorf("err = %v, want it to name both the sprint and the board", err)
	}
	if len(b.order) != 0 {
		t.Errorf("Jira was called (%v) for a board the sprint is not on", b.order)
	}
}

// TestACloseThatFailsSaysWhereTheCardsWent is the worst state this feature
// reaches: every unfinished card has left a sprint that is still open. The
// move path already wraps its own failure that way, and Jira's bare refusal
// on its own says nothing about the cards.
func TestACloseThatFailsSaysWhereTheCardsWent(t *testing.T) {
	b := &fakeBackend{issues: sprintOf("1", "1", "5"), completeErr: errors.New("403 Forbidden")}
	store := newStore()
	store.inSprint("PLAT-1", "PLAT-2", "PLAT-3")

	done, err := newService(b, store).Complete(context.Background(), "p1", 1, 12, "13")
	if err == nil {
		t.Fatal("complete = nil error, want the refused close reported")
	}
	for _, want := range []string{"403 Forbidden", "2 of 2", "Sprint 13", "could not be closed"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v, want it to say %q", err, want)
		}
	}
	if done.Moved != 2 {
		t.Errorf("completion = %+v, want the two cards it did move", done)
	}
	if got := strings.Join(store.membership["12"], ","); got != "PLAT-3" {
		t.Errorf("cached membership = %q, want the cards that are still in the sprint", got)
	}
}

// demoCard is one card of the demo dataset as the backend reports it now.
type demoCard struct {
	sprintID string
	status   string
}

// demoCards is the whole dataset by key, which is what a completion against
// the demo backend is measured against: which sprint every card was in
// before, and which it is in after.
func demoCards(t *testing.T, b *demobackend.Backend) map[string]demoCard {
	t.Helper()
	page, _, err := b.SearchIssuesPage(context.Background(), "PLAT", "", "", backend.AllTypes, 0, 500)
	if err != nil {
		t.Fatalf("read the demo dataset: %v", err)
	}
	out := map[string]demoCard{}
	for _, iss := range page {
		out[iss.Key] = demoCard{sprintID: iss.SprintID, status: iss.Status}
	}
	return out
}

// TestCompleteOnTheDemoBackendMovesOnlyThatSprintsCards runs the ceremony
// against the backend the plan's own walk-through uses, rather than against
// a fake written beside the service.
//
// It is the test that would have caught the demo answering "sprint = N"
// with the whole project: every card in the dataset then walked through the
// service's second narrowing, since an issue the search returns is kept
// unless it names a different sprint, and a completion moved the whole
// backlog into the destination and reported success. Nothing outside sprint
// 12 may move, and what does move is exactly the sprint's own unfinished
// cards.
func TestCompleteOnTheDemoBackendMovesOnlyThatSprintsCards(t *testing.T) {
	b := demobackend.New("PLAT")
	before := demoCards(t, b)
	store := newStore()
	store.columns = []backend.BoardColumn{
		{Name: "To Do", StatusIDs: []string{demobackend.StatusID("To Do")}},
		{Name: "In Progress", StatusIDs: []string{demobackend.StatusID("In Progress")}},
		{Name: "Done", StatusIDs: []string{demobackend.StatusID("Done")}},
	}
	inTwelve := []string{}
	for key, card := range before {
		if card.sprintID == "12" {
			inTwelve = append(inTwelve, key)
		}
	}
	sort.Strings(inTwelve)
	store.cached["12"] = inTwelve

	done, err := sprints.New(b, store, "PLAT").Complete(context.Background(), "p1", 1, 12, "13")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	after := demoCards(t, b)
	left := 0
	for key, was := range before {
		if after[key].sprintID == was.sprintID {
			continue
		}
		left++
		if was.sprintID != "12" {
			t.Errorf("%s was in sprint %q and is now in %q; completing sprint 12 moved a card that was never in it",
				key, was.sprintID, after[key].sprintID)
		}
	}
	if done.Moved != left {
		t.Errorf("the completion reported %d cards moved and %d actually moved", done.Moved, left)
	}
	if done.Moved == 0 || done.Moved >= len(before) {
		t.Fatalf("completion = %+v over a dataset of %d cards, want sprint 12's unfinished ones", done, len(before))
	}
	// The definition, read off the dataset rather than assumed: a card of
	// the sprint moved if and only if it was outside the last column.
	for _, key := range inTwelve {
		moved := after[key].sprintID != before[key].sprintID
		if finished := before[key].status == "Done"; finished == moved {
			t.Errorf("%s is %q and moved = %v, want only the unfinished cards of the sprint moved", key, before[key].status, moved)
		}
	}
}

// TestTheDestinationLearnsWhatWasMovedIntoIt is the other half of the
// membership rewrite, and the half nothing used to write. The banner says
// twelve cards moved to Sprint 13 and the board switches its picker to
// Sprint 13, so a destination that still draws what it drew before makes the
// one report this ceremony gives look like a lie. Nothing else corrects it:
// the ceremony bypasses the journal, so there is no pending move for the
// view to fold in either.
func TestTheDestinationLearnsWhatWasMovedIntoIt(t *testing.T) {
	b := &fakeBackend{issues: sprintOf("1", "5", "1")}
	store := newStore()
	store.inSprint("PLAT-1", "PLAT-2", "PLAT-3")
	store.holds("13", "PLAT-40")

	done, err := newService(b, store).Complete(context.Background(), "p1", 1, 12, "13")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if done.Moved != 2 || done.MovedTo != "Sprint 13" {
		t.Fatalf("completion = %+v, want the two unfinished cards moved to Sprint 13", done)
	}
	if got := strings.Join(store.membership["12"], ","); got != "PLAT-2" {
		t.Errorf("sprint 12 = %q, want only the card that finished", got)
	}
	// The cards it already held first, in the order it held them, and the
	// arrivals after: the destination's own rank is Jira's to hand back at
	// the next boards sync.
	if got := strings.Join(store.membership["13"], ","); got != "PLAT-40,PLAT-1,PLAT-3" {
		t.Errorf("sprint 13 = %q, want the cards it held plus the ones it was sent", got)
	}
}

// TestACompletionIntoTheBacklogWritesTheBoardsOwnList is the backlog as a
// destination rather than as an absence. The scope it writes back to is the
// board's own list, whose keys are already on the board, so the write is
// there to keep the pair symmetric and must not duplicate a card the list
// already holds.
func TestACompletionIntoTheBacklogWritesTheBoardsOwnList(t *testing.T) {
	b := &fakeBackend{issues: sprintOf("1", "1")}
	store := newStore()
	store.inSprint("PLAT-1", "PLAT-2")
	store.holds("", "PLAT-1", "PLAT-2", "PLAT-7")

	if _, err := newService(b, store).Complete(context.Background(), "p1", 1, 12, ""); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if got := strings.Join(store.membership["12"], ","); got != "" {
		t.Errorf("sprint 12 = %q, want it empty now every card has left it", got)
	}
	if got := strings.Join(store.membership[""], ","); got != "PLAT-1,PLAT-2,PLAT-7" {
		t.Errorf("the board's own list = %q, want it unchanged, since the cards never left the board", got)
	}
}
