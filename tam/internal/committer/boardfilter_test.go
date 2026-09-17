package committer_test

import (
	"context"
	"errors"
	"testing"

	"agile-suite/tam/internal/issuerepo"
)

// The boards phase's post-Commit filter check, split from boardcreate_test.go.

// Test 7: the post-Commit filter check reports a missing key, removes the
// row anyway, a failing check does not fail the Commit, and it runs once
// per board, not once per issue.
func TestTheFilterCheckReportsMissingKeysRemovesRowsAnywayAndASingleBoardsFailureDoesNotFailTheCommit(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.repo.AddToBoard(ctx, "p1", []string{"PLAT-1", "PLAT-2"}, demoBoard, issuerepo.ScopeBacklog); err != nil {
		t.Fatal(err)
	}
	const otherBoard = 2
	if err := h.repo.AddToBoard(ctx, "p1", []string{"PLAT-3"}, otherBoard, issuerepo.ScopeBacklog); err != nil {
		t.Fatal(err)
	}
	h.jira.filterMissing[demoBoard] = []string{"PLAT-1"}
	h.jira.filterCheckErr[otherBoard] = errors.New("GET failed: 503 service unavailable")

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 1 || res.Failures[0].Key != "PLAT-1" || res.Failures[0].EntityType != issuerepo.EntityIssueBoard {
		t.Fatalf("one result line, for the key outside the filter: %+v", res.Failures)
	}
	want := "PLAT-1 is outside board 1's filter, so it will not show on that board."
	if res.Failures[0].Error != want {
		t.Errorf("message = %q, want %q", res.Failures[0].Error, want)
	}
	if res.Failures[0].Retryable {
		t.Error("not retryable: retrying will not change what the filter matches")
	}
	// PLAT-3's board's check failed outright; that must not surface as a
	// failure of its own, and must not touch PLAT-1/PLAT-2's board.
	for _, f := range res.Failures {
		if f.Key == "PLAT-3" {
			t.Errorf("a failed filter check is not a Commit failure: %+v", f)
		}
	}
	if len(res.Moved) != 3 {
		t.Errorf("all three adds landed regardless of what the filter check found: %+v", res.Moved)
	}
	if res.Remaining != 0 {
		t.Errorf("every journal row is removed either way, the write landed: %d", res.Remaining)
	}
	if len(h.jira.filterChecks) != 2 {
		t.Fatalf("one check per board, not per issue: %v", h.jira.filterChecks)
	}
	byBoard := map[string]bool{}
	for _, c := range h.jira.filterChecks {
		byBoard[c] = true
	}
	if !byBoard["1 PLAT-1,PLAT-2"] || !byBoard["2 PLAT-3"] {
		t.Errorf("checks: %v", h.jira.filterChecks)
	}
}
