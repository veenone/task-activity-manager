package syncer_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/dbtx"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/syncer"
)

// These cover the one rule the boards pass gained: a closed sprint's
// membership is read once and then re-supplied from the cache for ever
// after. They live in their own file because boards_test.go is 364 lines
// and C2 counts a test file like any other.

// asksFor counts the calls the pass made for one sprint's keys, which is
// the whole point of read-once: a second call is the failure.
func asksFor(fb *fake, sprintID string) int {
	n := 0
	for _, r := range fb.keysRequested {
		if r.SprintID == sprintID {
			n++
		}
	}
	return n
}

// closedBoardFake is one scrum board, whatever sprints the caller wants,
// and a key per scope.
func closedBoardFake(sprints []backend.Sprint, keys map[string][]string) *fake {
	return &fake{
		boards:    []backend.Board{{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}},
		columns:   map[int][]backend.BoardColumn{1: {{Name: "To Do", StatusIDs: []string{"1"}}}},
		sprints:   map[int][]backend.Sprint{1: sprints},
		issueKeys: map[int]map[string][]string{1: keys},
	}
}

func syncTwice(t *testing.T, e *syncer.Engine) {
	t.Helper()
	for pass := 1; pass <= 2; pass++ {
		if _, err := e.SyncBoards(context.Background(), "p1", "PLAT", nil); err != nil {
			t.Fatalf("pass %d: %v", pass, err)
		}
	}
}

func engineFor(fb *fake, repo *issuerepo.Repository, boards *boardrepo.Repository) *syncer.Engine {
	e := syncer.New(fb, repo)
	e.Boards = boards
	return e
}

func TestAClosedSprintsMembershipIsReadOnceAndSurvivesTheNextPass(t *testing.T) {
	repo, boards := newBoardRepos(t)
	fb := closedBoardFake(
		[]backend.Sprint{{ID: 11, BoardID: 1, Name: "Sprint 11", State: "closed", StartDate: "2026-08-04T09:00:00Z"}},
		map[string][]string{"": {"PLAT-1"}, "11": {"PLAT-9", "PLAT-8"}},
	)
	syncTwice(t, engineFor(fb, repo, boards))

	// ReplaceBoard deletes every board_issue row of the board before it
	// writes the scopes the pass read, so a closed scope survives only
	// because readBoard re-supplied it from the cache. Asserting the rows
	// without the call count would pass against code that re-fetches every
	// time, and the call count without the rows would pass against code
	// that fetched nothing at all.
	got, err := boards.SprintIssues(context.Background(), "p1", 1, "11")
	if err != nil {
		t.Fatalf("sprint issues: %v", err)
	}
	if len(got) != 2 || got[0] != "PLAT-9" || got[1] != "PLAT-8" {
		t.Errorf("sprint 11 membership = %v, want PLAT-9 and PLAT-8 in board order", got)
	}
	if n := asksFor(fb, "11"); n != 1 {
		t.Errorf("sprint 11 was asked for %d times across two passes, want 1", n)
	}
}

func TestAnActiveSprintsKeysAreStillFetchedEveryPass(t *testing.T) {
	repo, boards := newBoardRepos(t)
	fb := closedBoardFake(
		[]backend.Sprint{{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active", StartDate: "2026-08-18T09:00:00Z"}},
		map[string][]string{"": {"PLAT-1"}, "12": {"PLAT-1"}},
	)
	syncTwice(t, engineFor(fb, repo, boards))

	// A running sprint's membership changes under the pass, so the cache is
	// never an answer for it. This is the half of the rule the change must
	// not have broken.
	if n := asksFor(fb, "12"); n != 2 {
		t.Errorf("sprint 12 was asked for %d times across two passes, want one per pass", n)
	}
}

func TestASprintThatClosesKeepsTheMembershipItsLastActivePassCached(t *testing.T) {
	repo, boards := newBoardRepos(t)
	fb := closedBoardFake(
		[]backend.Sprint{{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active", StartDate: "2026-08-18T09:00:00Z"}},
		map[string][]string{"": {"PLAT-1"}, "12": {"PLAT-1", "PLAT-2"}},
	)
	e := engineFor(fb, repo, boards)
	ctx := context.Background()
	if _, err := e.SyncBoards(ctx, "p1", "PLAT", nil); err != nil {
		t.Fatalf("first pass: %v", err)
	}
	// Jira closes it between passes, which is the common way a sprint's
	// membership comes to be cached at all.
	fb.sprints[1] = []backend.Sprint{{ID: 12, BoardID: 1, Name: "Sprint 12", State: "closed", StartDate: "2026-08-18T09:00:00Z"}}
	if _, err := e.SyncBoards(ctx, "p1", "PLAT", nil); err != nil {
		t.Fatalf("second pass: %v", err)
	}

	got, err := boards.SprintIssues(ctx, "p1", 1, "12")
	if err != nil {
		t.Fatalf("sprint issues: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("sprint 12 membership = %v, want the two cards its active pass cached", got)
	}
	if n := asksFor(fb, "12"); n != 1 {
		t.Errorf("sprint 12 was asked for %d times, want only the pass it was still active for", n)
	}
}

func TestTheBackfillReachesTheTailOfABoardWhoseHistoryIsMostlyAnotherProjects(t *testing.T) {
	repo, boards := newBoardRepos(t)
	// Thirteen closed sprints against a budget of twelve, and the twelve
	// newest hold nothing of this project, which is the ordinary shape of a
	// board whose filter spans several projects: ownBoards' own comment
	// describes one holding 8,485 cards while the project being synced had
	// 38. Only the oldest has anything here.
	//
	// This is the case that caught the first version of the backfill. It
	// counted a sprint as read when board_issue held a row for it, so a
	// sprint that answered empty looked unread for ever, spent a unit of
	// every pass, and the tail was never reached at all. Jira hands the
	// sprints back oldest first, which is also the order that would leave
	// the newest unread if the pass walked them as they came.
	var sprints []backend.Sprint
	keys := map[string][]string{"": {"PLAT-1"}}
	for id := 1; id <= 13; id++ {
		sprints = append(sprints, backend.Sprint{
			ID: id, BoardID: 1, Name: fmt.Sprintf("Sprint %d", id), State: "closed",
			StartDate: fmt.Sprintf("2026-%02d-01T09:00:00Z", id),
		})
	}
	keys["1"] = []string{"PLAT-1"}
	fb := closedBoardFake(sprints, keys)
	e := engineFor(fb, repo, boards)
	ctx := context.Background()

	if _, err := e.SyncBoards(ctx, "p1", "PLAT", nil); err != nil {
		t.Fatalf("first pass: %v", err)
	}
	// The budget spends itself from the top of the Sprints view downward,
	// so the sprint that finished most recently gets its numbers first.
	if n := asksFor(fb, "13"); n != 1 {
		t.Errorf("the newest closed sprint was asked for %d times on the first pass, want 1", n)
	}
	if n := asksFor(fb, "1"); n != 0 {
		t.Errorf("the oldest closed sprint was asked for %d times on the first pass, want 0: the budget is twelve", n)
	}

	if _, err := e.SyncBoards(ctx, "p1", "PLAT", nil); err != nil {
		t.Fatalf("second pass: %v", err)
	}
	// The twelve that answered empty are read, so they do not spend a unit
	// again and the second pass reaches the tail.
	if n := asksFor(fb, "1"); n != 1 {
		t.Errorf("the oldest closed sprint was asked for %d times in total, want 1 on the second pass", n)
	}
	oldest, err := boards.SprintIssues(ctx, "p1", 1, "1")
	if err != nil || len(oldest) != 1 || oldest[0] != "PLAT-1" {
		t.Fatalf("oldest sprint membership = %v, %v, want PLAT-1 after the second pass", oldest, err)
	}

	// And then it settles: three more passes ask for nothing at all, which
	// is what "read once and kept" has to mean for a sprint that answered
	// empty as much as for one that answered with cards.
	for pass := 3; pass <= 5; pass++ {
		if _, err := e.SyncBoards(ctx, "p1", "PLAT", nil); err != nil {
			t.Fatalf("pass %d: %v", pass, err)
		}
	}
	for id := 1; id <= 13; id++ {
		if n := asksFor(fb, fmt.Sprint(id)); n != 1 {
			t.Errorf("closed sprint %d was asked for %d times across five passes, want exactly 1", id, n)
		}
	}
}

func TestAnEmptyClosedSprintReportsItselfReadRatherThanUnread(t *testing.T) {
	repo, boards := newBoardRepos(t)
	// board_issue cannot tell a sprint nobody has read from one that holds
	// nothing, so the sprint row carries the bit instead. Without it the
	// row would say "Cards not read yet" for ever about a sprint that was
	// read and was genuinely empty.
	fb := closedBoardFake(
		[]backend.Sprint{{ID: 11, BoardID: 1, Name: "Sprint 11", State: "closed", StartDate: "2026-08-04T09:00:00Z"}},
		map[string][]string{"": {"PLAT-1"}},
	)
	syncTwice(t, engineFor(fb, repo, boards))

	details, err := boards.BoardSprintDetails(context.Background(), noIssues{}, "p1", 1)
	if err != nil {
		t.Fatalf("board sprint details: %v", err)
	}
	for _, d := range details {
		if d.Name != "Sprint 11" {
			continue
		}
		if !d.MembershipCached {
			t.Error("a closed sprint that was read and answered empty must report itself read")
		}
		if d.Total != 0 {
			t.Errorf("sprint 11 total = %d, want 0: it was read and it holds nothing", d.Total)
		}
		return
	}
	t.Fatal("no Sprint 11 in the details")
}

// noIssues is an IssueSource with nothing in the cache. The membership
// question these tests ask is about board_issue and the sprint row, not
// about which cards the issue cache happens to hold.
type noIssues struct{}

func (noIssues) IssuesByKeys(context.Context, dbtx.Querier, string, []string) ([]backend.Issue, error) {
	return nil, nil
}
func (noIssues) DraftIssues(context.Context, dbtx.Querier, string) ([]backend.Issue, error) {
	return nil, nil
}
func (noIssues) PendingMoves(context.Context, dbtx.Querier, string) ([]backend.PendingMove, error) {
	return nil, nil
}

func TestAFailedHistoricalReadDoesNotCostTheBoardItsSync(t *testing.T) {
	repo, boards := newBoardRepos(t)
	// The backfill is best-effort work about sprints nobody is waiting on.
	// A 403 or a timeout on a sprint from years ago must not drop the board
	// that carries the running sprint, which is what the view is for, and
	// it must not do so silently on every pass for ever.
	fb := closedBoardFake(
		[]backend.Sprint{
			{ID: 11, BoardID: 1, Name: "Sprint 11", State: "closed", StartDate: "2026-08-04T09:00:00Z"},
			{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active", StartDate: "2026-08-18T09:00:00Z"},
		},
		map[string][]string{"": {"PLAT-1", "PLAT-2"}, "12": {"PLAT-2"}},
	)
	fb.issueKeysErr = map[int]map[string]error{1: {"11": errors.New("jira: 403 Forbidden")}}
	e := engineFor(fb, repo, boards)
	ctx := context.Background()

	sum, err := e.SyncBoards(ctx, "p1", "PLAT", nil)
	if err != nil {
		t.Fatalf("sync boards: %v", err)
	}
	if sum.Boards != 1 || len(sum.Dropped) != 0 {
		t.Errorf("summary = %d boards, dropped %v; want the board landed and nothing dropped", sum.Boards, sum.Dropped)
	}
	// The mandatory work landed: the running sprint has its membership.
	running, err := boards.SprintIssues(ctx, "p1", 1, "12")
	if err != nil || len(running) != 1 || running[0] != "PLAT-2" {
		t.Fatalf("active sprint membership = %v, %v, want PLAT-2", running, err)
	}
	// And the sprint that failed is not marked read, so it is tried again
	// rather than passed off as a sprint that holds nothing.
	syncedNow, err := boards.SyncedSprints(ctx, "p1", 1)
	if err != nil {
		t.Fatalf("synced sprints: %v", err)
	}
	if syncedNow["11"] {
		t.Error("a sprint whose read failed must not be recorded as read")
	}

	if _, err := e.SyncBoards(ctx, "p1", "PLAT", nil); err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if n := asksFor(fb, "11"); n != 2 {
		t.Errorf("the failing sprint was asked for %d times across two passes, want one per pass until it answers", n)
	}
}
