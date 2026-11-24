package committer_test

import (
	"context"
	"strings"
	"testing"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
)

// A create that got no HTTP answer may or may not have made the issue. The
// next Commit looks for it before posting again (#608).

func TestALostCreateThatLandedIsAdoptedNotPostedTwice(t *testing.T) {
	eng, repo, f := setup(t)
	ctx := context.Background()
	temp, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "Lost task"})
	if err != nil {
		t.Fatal(err)
	}
	f.createLost = "after"
	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 1 || res.Failures[0].Key != temp || !res.Failures[0].Retryable || res.Remaining != 1 {
		t.Fatalf("the lost create fails retryable and keeps its row: %+v", res)
	}
	// A local edit between the two Commits keeps the record of the attempt.
	if err := repo.EditField(ctx, "p1", temp, "labels", "later"); err != nil {
		t.Fatal(err)
	}

	f.createLost = ""
	res, err = eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.creates) != 1 {
		t.Fatalf("the second Commit posted again: %d creates", len(f.creates))
	}
	if len(res.Created) != 1 || res.Created[0].TempKey != temp || res.Created[0].Key != "PLAT-501" || len(res.Failures) != 0 || res.Remaining != 0 {
		t.Fatalf("the draft is adopted as PLAT-501: %+v", res)
	}
	if len(f.searches) != 1 || !strings.Contains(f.searches[0], "reporter = currentUser()") || !strings.Contains(f.searches[0], "created >= ") {
		t.Errorf("one search for what the user reported since the attempt: %q", f.searches)
	}
	if iss, err := repo.GetIssue(ctx, "p1", "PLAT-501"); err != nil || iss.Summary != "Lost task" {
		t.Errorf("the adopted row is in the store: %+v %v", iss, err)
	}
}

func TestALostCreateThatDidNotLandIsCreatedOnce(t *testing.T) {
	eng, repo, f := setup(t)
	ctx := context.Background()
	temp, _ := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "Never made"})
	f.createLost = "before"
	if _, err := eng.Commit(ctx, "p1", "PLAT"); err != nil {
		t.Fatal(err)
	}
	f.createLost = ""
	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.creates) != 1 || len(f.searches) != 1 {
		t.Fatalf("one search, then one create: %d creates, %d searches", len(f.creates), len(f.searches))
	}
	if len(res.Created) != 1 || res.Created[0].TempKey != temp || res.Remaining != 0 {
		t.Errorf("created: %+v", res)
	}
}

func TestTwoMatchesForALostCreateHoldTheDraft(t *testing.T) {
	eng, repo, f := setup(t)
	ctx := context.Background()
	f.rows["PLAT-77"] = backend.Issue{Key: "PLAT-77", Project: "PLAT", Type: backend.TypeTask, Summary: "Twice", Reporter: "f"}
	temp, _ := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "Twice"})
	f.createLost = "after"
	if _, err := eng.Commit(ctx, "p1", "PLAT"); err != nil {
		t.Fatal(err)
	}
	f.createLost = ""
	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.creates) != 1 || len(res.Created) != 0 {
		t.Fatalf("nothing posted or adopted: %d creates, %+v", len(f.creates), res.Created)
	}
	if len(res.Failures) != 1 || res.Failures[0].Key != temp || !strings.Contains(res.Failures[0].Error, "PLAT-501") || !strings.Contains(res.Failures[0].Error, "PLAT-77") || res.Remaining != 1 {
		t.Errorf("held with a failure naming both: %+v", res)
	}
}

func TestACreateJiraRefusedIsNotLost(t *testing.T) {
	eng, repo, f := setup(t)
	ctx := context.Background()
	_, _ = repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "Refused"})
	f.createErr = &corejira.WriteError{Method: "POST", Path: "/rest/api/2/issue", Status: "400 Bad Request", Message: "Severity is required"}
	if _, err := eng.Commit(ctx, "p1", "PLAT"); err != nil {
		t.Fatal(err)
	}
	f.createErr = nil
	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.searches) != 0 || len(f.creates) != 1 || len(res.Created) != 1 {
		t.Errorf("a refused create is retried without a search: %d searches, %d creates, %+v", len(f.searches), len(f.creates), res)
	}
}
