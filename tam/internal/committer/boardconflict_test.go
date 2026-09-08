package committer_test

import (
	"context"
	"errors"
	"testing"

	"agile-suite/tam/internal/issuerepo"
)

// The two resolutions of a board conflict. A board row is held back on its
// before_val against the remote status or sprint id, which is not the
// comparison an edit is held on, so both resolutions have to be shown to
// mean the same thing here that they mean everywhere else.

// TestOverrideOnABoardConflictPushesTheMoveOnTheNextCommit is the one that
// would have shipped broken: rebasing only the base version left the row's
// before_val pointing at where the card used to be, so the identical
// conflict came back on every Commit from then on, with no way out.
func TestOverrideOnABoardConflictPushesTheMoveOnTheNextCommit(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.repo.MoveToColumn(ctx, "p1", "PLAT-1", "5"); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.MoveToSprint(ctx, "p1", "PLAT-3", "13", "Sprint 13"); err != nil {
		t.Fatal(err)
	}
	// Somebody else moved both cards while the laptop was offline: one to a
	// third status, one to another sprint.
	moved := h.jira.rows["PLAT-1"]
	moved.StatusID, moved.Status, moved.Updated = "3", "In Progress", "2026-09-06T00:00:00Z"
	h.jira.rows["PLAT-1"] = moved
	moved = h.jira.rows["PLAT-3"]
	moved.SprintID, moved.SprintName, moved.Updated = "11", "Sprint 11", "2026-09-06T00:00:00Z"
	h.jira.rows["PLAT-3"] = moved

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Conflicts) != 2 || len(h.jira.pushed) != 0 {
		t.Fatalf("both cards are held back: %+v (pushed %v)", res, h.jira.pushed)
	}
	for _, c := range res.Conflicts {
		if err := h.eng.ResolveOverride(ctx, "p1", c.Key, c.RemoteVersion); err != nil {
			t.Fatalf("override %s: %v", c.Key, err)
		}
	}

	res, err = h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Conflicts) != 0 || len(res.Failures) != 0 || res.Remaining != 0 {
		t.Fatalf("override means the move goes anyway: %+v", res)
	}
	if len(res.Moved) != 2 {
		t.Fatalf("both moves landed: %+v", res.Moved)
	}
	byType := map[string]string{}
	for _, m := range res.Moved {
		byType[m.EntityType] = m.Target
	}
	if byType[issuerepo.EntityTransition] != "Done" || byType[issuerepo.EntitySprintMove] != "Sprint 13" {
		t.Errorf("the user's own targets were pushed: %+v", res.Moved)
	}
}

// TestKeepRemoteOnABoardConflictDropsTheMove is the other resolution: the
// journal row goes and the cached card takes the place Jira has it in.
func TestKeepRemoteOnABoardConflictDropsTheMove(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.repo.MoveToColumn(ctx, "p1", "PLAT-1", "5"); err != nil {
		t.Fatal(err)
	}
	moved := h.jira.rows["PLAT-1"]
	moved.StatusID, moved.Status, moved.Updated = "3", "In Progress", "2026-09-06T00:00:00Z"
	h.jira.rows["PLAT-1"] = moved

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Conflicts) != 1 {
		t.Fatalf("the card is held back: %+v", res)
	}
	if err := h.eng.ResolveKeepRemote(ctx, "p1", "PLAT-1"); err != nil {
		t.Fatal(err)
	}

	res, err = h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Conflicts) != 0 || len(res.Moved) != 0 || len(h.jira.pushed) != 0 || res.Remaining != 0 {
		t.Fatalf("nothing is left to push: %+v (pushed %v)", res, h.jira.pushed)
	}
	iss, err := h.repo.GetIssue(ctx, "p1", "PLAT-1")
	if err != nil {
		t.Fatal(err)
	}
	if iss.Pending || iss.StatusID != "3" {
		t.Errorf("the card sits where Jira has it: %+v", iss)
	}
}

// TestOverrideAuditsOnlyTheBoardRowThatWasHeld covers the trail rather than
// the outcome. An issue is held back whole, so an Override raised by an
// edit alone reaches a board row that never disagreed with Jira. Rebasing
// that row writes the value it already carried and audits an override that
// overrode nothing, which reads in the Activity tab as a decision the user
// never made.
func TestOverrideAuditsOnlyTheBoardRowThatWasHeld(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.repo.EditField(ctx, "p1", "PLAT-1", "summary", "mine"); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.MoveToColumn(ctx, "p1", "PLAT-1", "5"); err != nil {
		t.Fatal(err)
	}
	// The card has not moved in Jira, so the transition is a push and not a
	// conflict; only the edit is held, on the updated stamp. The push then
	// fails, which is what leaves the board row for the Override to find.
	moved := h.jira.rows["PLAT-1"]
	moved.Summary, moved.Updated = "theirs", "2026-09-06T00:00:00Z"
	h.jira.rows["PLAT-1"] = moved
	h.jira.transitionErr["PLAT-1"] = errors.New("the gateway timed out")

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Conflicts) != 1 || len(res.Failures) != 1 {
		t.Fatalf("the edit is held and the transition failed: %+v", res)
	}
	for _, f := range res.Conflicts[0].Fields {
		if f.Field != "summary" {
			t.Fatalf("only the edit disagreed with Jira: %+v", res.Conflicts[0].Fields)
		}
	}
	if err := h.eng.ResolveOverride(ctx, "p1", "PLAT-1", res.Conflicts[0].RemoteVersion); err != nil {
		t.Fatal(err)
	}

	entries, err := h.repo.ListActivity(ctx, "p1", "PLAT-1", 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range entries {
		if a.Action == "override" && a.EntityType == issuerepo.EntityTransition {
			t.Errorf("a board row that was never held must not be audited as overridden: %+v", a)
		}
	}
}
