package syncer_test

import (
	"context"
	"fmt"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
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

func TestABoardWithMoreHistoryThanOnePassCanReadBackfillsNewestFirst(t *testing.T) {
	repo, boards := newBoardRepos(t)
	// Thirteen closed sprints against a budget of twelve, named so that a
	// higher id is a later sprint. Jira hands them back oldest first, which
	// is the order that would leave the newest unread if the pass walked
	// the list as it came.
	var sprints []backend.Sprint
	keys := map[string][]string{"": {"PLAT-1"}}
	for id := 1; id <= 13; id++ {
		sprints = append(sprints, backend.Sprint{
			ID: id, BoardID: 1, Name: fmt.Sprintf("Sprint %d", id), State: "closed",
			StartDate: fmt.Sprintf("2026-%02d-01T09:00:00Z", id),
		})
		keys[fmt.Sprint(id)] = []string{fmt.Sprintf("PLAT-%d", id)}
	}
	fb := closedBoardFake(sprints, keys)
	e := engineFor(fb, repo, boards)
	ctx := context.Background()
	if _, err := e.SyncBoards(ctx, "p1", "PLAT", nil); err != nil {
		t.Fatalf("first pass: %v", err)
	}

	// The budget spends itself from the top of the Sprints view downward,
	// so the sprint that finished most recently is the one a reader sees
	// numbers for first.
	if n := asksFor(fb, "13"); n != 1 {
		t.Errorf("the newest closed sprint was asked for %d times on the first pass, want 1", n)
	}
	if n := asksFor(fb, "1"); n != 0 {
		t.Errorf("the oldest closed sprint was asked for %d times on the first pass, want 0", n)
	}
	oldest, err := boards.SprintIssues(ctx, "p1", 1, "1")
	if err != nil || len(oldest) != 0 {
		t.Fatalf("oldest sprint membership = %v, %v, want none yet", oldest, err)
	}

	if _, err := e.SyncBoards(ctx, "p1", "PLAT", nil); err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if n := asksFor(fb, "1"); n != 1 {
		t.Errorf("the oldest closed sprint was asked for %d times in total, want 1 on the second pass", n)
	}
	oldest, err = boards.SprintIssues(ctx, "p1", 1, "1")
	if err != nil || len(oldest) != 1 || oldest[0] != "PLAT-1" {
		t.Fatalf("oldest sprint membership = %v, %v, want PLAT-1 after the second pass", oldest, err)
	}
	// And the twelve the first pass read are not read again to pay for it.
	if n := asksFor(fb, "13"); n != 1 {
		t.Errorf("the newest closed sprint was asked for %d times in total, want 1", n)
	}
}

func TestAClosedSprintThatHoldsNothingIsAskedAboutAgain(t *testing.T) {
	repo, boards := newBoardRepos(t)
	// The stated ceiling: board_issue cannot tell a closed sprint nobody
	// has read from one that genuinely holds nothing of this project, so
	// the second is re-read once per pass. One request, and the row stays
	// honest about it rather than claiming an empty sprint is a read one.
	fb := closedBoardFake(
		[]backend.Sprint{{ID: 11, BoardID: 1, Name: "Sprint 11", State: "closed", StartDate: "2026-08-04T09:00:00Z"}},
		map[string][]string{"": {"PLAT-1"}},
	)
	syncTwice(t, engineFor(fb, repo, boards))

	if n := asksFor(fb, "11"); n != 2 {
		t.Errorf("an empty closed sprint was asked for %d times across two passes, want one per pass", n)
	}
	got, err := boards.SprintIssues(context.Background(), "p1", 1, "11")
	if err != nil || len(got) != 0 {
		t.Fatalf("sprint 11 membership = %v, %v, want none", got, err)
	}
}
