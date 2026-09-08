package committer_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agile-suite/tam/internal/committer"
	"agile-suite/tam/internal/issuerepo"
)

// The rank group's own tests, beside ranks.go: what a board's final order
// anchors each card against, and what is left when there is nothing to
// anchor to.

func TestTwoCardsRankedAgainstEachOtherCommitInTheBoardsOrder(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.repo.RankIssue(ctx, "p1", "PLAT-3", "PLAT-2", true, demoBoard); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.RankIssue(ctx, "p1", "PLAT-1", "PLAT-3", true, demoBoard); err != nil {
		t.Fatal(err)
	}
	// The order the screen ended up showing, which is not the order the two
	// drops were journaled in.
	h.order.order[demoBoard] = []string{"PLAT-2", "PLAT-1", "PLAT-3"}

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"rank PLAT-1 after PLAT-2", "rank PLAT-3 after PLAT-1"}
	if strings.Join(h.jira.pushed, " | ") != strings.Join(want, " | ") {
		t.Errorf("ranks push top to bottom, each after the card above it: %v", h.jira.pushed)
	}
	if len(res.Moved) != 2 || len(res.Failures) != 0 || res.Remaining != 0 {
		t.Errorf("result: %+v", res)
	}
}

// TestACardRankedToTheTopOfTheBoardGoesBeforeTheCardBelowIt covers the
// commonest gesture on a board, dragging a card to the top of the leftmost
// column. There is no card above it to anchor against, and anchoring only
// downwards used to report that as dropped. "X before Y" puts X in exactly
// the place "X after W" would in Jira's one global rank, and only the card
// at the top of a board can take that branch.
func TestACardRankedToTheTopOfTheBoardGoesBeforeTheCardBelowIt(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.repo.RankIssue(ctx, "p1", "PLAT-1", "PLAT-3", true, demoBoard); err != nil {
		t.Fatal(err)
	}
	h.order.order[demoBoard] = []string{"PLAT-1", "PLAT-3"}

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.pushed) != 1 || h.jira.pushed[0] != "rank PLAT-1 before PLAT-3" {
		t.Fatalf("the top card is ranked before the one below it: %v", h.jira.pushed)
	}
	if len(res.Failures) != 0 || len(res.Moved) != 1 || res.Remaining != 0 {
		t.Fatalf("result: %+v", res)
	}
	if m := res.Moved[0]; m.Target != "PLAT-3" || m.Side != issuerepo.RankSideBefore {
		t.Errorf("the move names its anchor and its side: %+v", m)
	}
}

// TestTheOnlyCardOnABoardHasNothingToRankAgainst is what is left of the
// no-anchor drop once the top of the board is a push: a board of one card.
func TestTheOnlyCardOnABoardHasNothingToRankAgainst(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.repo.RankIssue(ctx, "p1", "PLAT-1", "PLAT-2", true, demoBoard); err != nil {
		t.Fatal(err)
	}
	h.order.order[demoBoard] = []string{"PLAT-1"}

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.pushed) != 0 {
		t.Errorf("nothing is pushed against a card that is not there: %v", h.jira.pushed)
	}
	if len(res.Failures) != 1 {
		t.Fatalf("one failure, the rank's: %+v", res.Failures)
	}
	f := res.Failures[0]
	if f.Key != "PLAT-1" || f.EntityType != issuerepo.EntityRank || f.RowID == 0 || f.Retryable {
		t.Errorf("the dropped rank names its row: %+v", f)
	}
	if !strings.Contains(f.Error, "only card on board 1") {
		t.Errorf("the reason says why: %s", f.Error)
	}
	if res.Remaining != 1 {
		t.Errorf("the row stays for an undo: %d", res.Remaining)
	}
}

// TestAFailedRankDoesNotAnchorTheNextCard is the other half of getting the
// anchor right: a card Jira never moved is not where the board says it is,
// so the next card down cannot be ranked against it.
func TestAFailedRankDoesNotAnchorTheNextCard(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.repo.RankIssue(ctx, "p1", "PLAT-1", "PLAT-3", true, demoBoard); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.RankIssue(ctx, "p1", "PLAT-2", "PLAT-1", false, demoBoard); err != nil {
		t.Fatal(err)
	}
	h.order.order[demoBoard] = []string{"PLAT-1", "PLAT-2", "PLAT-3"}
	h.jira.rankErr["PLAT-1"] = errors.New("PUT failed: 503 service unavailable")

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.pushed) != 0 {
		t.Errorf("nothing is ranked against a card Jira never moved: %v", h.jira.pushed)
	}
	if len(res.Failures) != 2 || len(res.Moved) != 0 || res.Remaining != 2 {
		t.Fatalf("both ranks stayed: %+v", res)
	}
	byKey := map[string]committer.Failure{}
	for _, f := range res.Failures {
		byKey[f.Key] = f
	}
	if f := byKey["PLAT-1"]; !f.Retryable {
		t.Errorf("a transport failure is worth retrying: %+v", f)
	}
	if f := byKey["PLAT-2"]; f.Retryable || !strings.Contains(f.Error, "no card above PLAT-2 on board 1 was ranked") {
		t.Errorf("the card below it says why it was left: %+v", f)
	}
}

func TestARankWhoseCardHasLeftTheBoardIsDropped(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.repo.RankIssue(ctx, "p1", "PLAT-1", "PLAT-2", true, demoBoard); err != nil {
		t.Fatal(err)
	}
	h.order.order[demoBoard] = []string{"PLAT-2", "PLAT-3"}

	res, _ := h.eng.Commit(ctx, "p1", "PLAT")
	if len(h.jira.pushed) != 0 || len(res.Failures) != 1 {
		t.Fatalf("pushed %v, failures %+v", h.jira.pushed, res.Failures)
	}
	if !strings.Contains(res.Failures[0].Error, "no longer on board 1") || res.Remaining != 1 {
		t.Errorf("failure: %+v (remaining %d)", res.Failures[0], res.Remaining)
	}
}

func TestARankWhoseBoardIsGoneFromTheStoreIsDropped(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.repo.RankIssue(ctx, "p1", "PLAT-1", "PLAT-2", true, 9); err != nil {
		t.Fatal(err)
	}
	res, _ := h.eng.Commit(ctx, "p1", "PLAT")
	if len(res.Failures) != 1 || res.Failures[0].Retryable || res.Remaining != 1 {
		t.Fatalf("result: %+v", res)
	}
	if !strings.Contains(res.Failures[0].Error, "board 9") {
		t.Errorf("the reason names the board: %s", res.Failures[0].Error)
	}
}
