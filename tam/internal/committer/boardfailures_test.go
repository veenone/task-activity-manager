package committer_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/committer"
)

// What a board row that did not land says for itself, and whether the
// dialog offers to Commit again for it.

// readOnlyBoards answers backend.IssueBackend and nothing more: embedding
// the interface gives it the reads and the transition, and leaves it
// without the two Agile writes, which is exactly the backend the commit
// pass's type assertion is there to catch.
type readOnlyBoards struct{ backend.IssueBackend }

// TestABackendThatCannotWriteToBoardsStillPushesItsTransitions is the
// branch the assertion guards. A transition goes through Jira's ordinary
// REST API and still lands; the rank and the sprint move say why they
// cannot, and neither is worth a retry.
func TestABackendThatCannotWriteToBoardsStillPushesItsTransitions(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	eng := committer.New(readOnlyBoards{h.jira}, h.repo, h.order)
	if err := h.repo.MoveToColumn(ctx, "p1", "PLAT-1", "5"); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.MoveToSprint(ctx, "p1", "PLAT-2", "13", "Sprint 13"); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.RankIssue(ctx, "p1", "PLAT-3", "PLAT-2", true, demoBoard); err != nil {
		t.Fatal(err)
	}
	h.order.order[demoBoard] = []string{"PLAT-2", "PLAT-3"}

	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.pushed) != 1 || h.jira.pushed[0] != "transition PLAT-1 5" {
		t.Fatalf("the transition still goes: %v", h.jira.pushed)
	}
	if len(res.Moved) != 1 || len(res.Failures) != 2 || res.Remaining != 2 {
		t.Fatalf("result: %+v", res)
	}
	for _, f := range res.Failures {
		if f.Retryable {
			t.Errorf("a backend that cannot write to boards will not start: %+v", f)
		}
		if !strings.Contains(f.Error, "cannot move cards between sprints or reorder them") {
			t.Errorf("the reason says what is missing: %s", f.Error)
		}
	}
}

// TestAnInstanceWithNoAgileAPIIsNotWorthARetry is the case that actually
// reaches a person: both shipped backends can write to boards, but a Data
// Center without Jira Software answers the rank and the sprint endpoints
// with a 404, which core/jira reports as ErrNoAgile. Nothing about sending
// it again changes that.
func TestAnInstanceWithNoAgileAPIIsNotWorthARetry(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.repo.MoveToSprint(ctx, "p1", "PLAT-1", "13", "Sprint 13"); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.RankIssue(ctx, "p1", "PLAT-2", "PLAT-3", true, demoBoard); err != nil {
		t.Fatal(err)
	}
	h.order.order[demoBoard] = []string{"PLAT-3", "PLAT-2"}
	h.jira.sprintErr = fmt.Errorf("move PLAT-1 to sprint 13: %w", corejira.ErrNoAgile)
	h.jira.rankErr["PLAT-2"] = fmt.Errorf("rank PLAT-2 after PLAT-3: %w", corejira.ErrNoAgile)

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 2 || res.Remaining != 2 {
		t.Fatalf("both rows stayed: %+v", res)
	}
	for _, f := range res.Failures {
		if f.Retryable {
			t.Errorf("an instance with no agile api answers the same way every time: %+v", f)
		}
	}
}

// TestAJournalDeleteThatFailedAfterAPushIsRetryable is the one board
// failure that a second Commit really does fix. The write landed in Jira
// and only the local delete failed, so the next Commit reads a remote that
// is already at the target, calls the row satisfied, and clears it.
func TestAJournalDeleteThatFailedAfterAPushIsRetryable(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.repo.MoveToColumn(ctx, "p1", "PLAT-1", "5"); err != nil {
		t.Fatal(err)
	}
	// A SQLite write error, staged where the pass makes its only local
	// write: the trigger refuses the delete and leaves every other
	// statement of the commit alone.
	if _, err := h.db.Exec(`CREATE TRIGGER no_clear BEFORE DELETE ON pending_change
		BEGIN SELECT RAISE(ABORT, 'disk I/O error'); END`); err != nil {
		t.Fatal(err)
	}

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.pushed) != 1 || len(res.Failures) != 1 || res.Remaining != 1 {
		t.Fatalf("the push landed and the row stayed: %+v (pushed %v)", res, h.jira.pushed)
	}
	f := res.Failures[0]
	if !f.Retryable || !strings.Contains(f.Error, "journal could not be cleared") {
		t.Errorf("a failed local delete is worth another Commit: %+v", f)
	}

	// And the retry settles it: Jira is already at the target, so the row
	// is satisfied rather than pushed a second time.
	if _, err := h.db.Exec(`DROP TRIGGER no_clear`); err != nil {
		t.Fatal(err)
	}
	res, err = h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.pushed) != 1 || len(res.Failures) != 0 || res.Remaining != 0 {
		t.Fatalf("the second Commit clears it without pushing again: %+v (pushed %v)", res, h.jira.pushed)
	}
	if len(res.Moved) != 1 || !res.Moved[0].Satisfied {
		t.Errorf("the row is satisfied, not pushed: %+v", res.Moved)
	}
}
