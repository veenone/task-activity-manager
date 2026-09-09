package committer_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// draftInSprint is a draft made with a sprint already chosen, which is what
// the New issue dialog and the importer's Sprint column both produce.
func draftInSprint(sprintID, sprintName string) backend.IssueDraft {
	return backend.IssueDraft{
		Type: backend.TypeTask, Summary: "Rotate the gateway keys",
		SprintID: sprintID, SprintName: sprintName,
	}
}

func TestADraftCreatedIntoASprintIsMovedThereAfterItsCreate(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	temp, err := h.repo.CreateDraft(ctx, "p1", "PLAT", draftInSprint("42", "Sprint 42"))
	if err != nil {
		t.Fatal(err)
	}

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Created) != 1 || res.Created[0].TempKey != temp || res.Created[0].Key != "PLAT-501" {
		t.Fatalf("the draft was created: %+v", res)
	}
	// The move names the key Jira handed back, which is the ordering: the
	// intent cannot exist under PLAT-501 until the create has answered, and
	// the board pass runs after the creates in the same Commit.
	if len(h.jira.pushed) != 1 || h.jira.pushed[0] != "sprint 42 PLAT-501" {
		t.Fatalf("one sprint move, under the real key: %v", h.jira.pushed)
	}
	if len(res.Moved) != 1 || res.Moved[0].Key != "PLAT-501" ||
		res.Moved[0].EntityType != issuerepo.EntitySprintMove || res.Moved[0].Target != "Sprint 42" {
		t.Errorf("the move is reported: %+v", res.Moved)
	}
	if len(res.Failures) != 0 || len(res.Conflicts) != 0 || res.Remaining != 0 {
		t.Fatalf("nothing was left behind: %+v", res)
	}
	if remote := h.jira.rows["PLAT-501"]; remote.SprintID != "42" {
		t.Errorf("Jira holds the issue in the sprint: %+v", remote)
	}
	iss, err := h.repo.GetIssue(ctx, "p1", "PLAT-501")
	if err != nil || iss.Pending || iss.SprintID != "42" || iss.SprintName != "Sprint 42" {
		t.Errorf("the local row settled in the sprint: %+v %v", iss, err)
	}
}

func TestADraftWithNoSprintPushesNoMove(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if _, err := h.repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "No sprint"}); err != nil {
		t.Fatal(err)
	}

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.pushed) != 0 || len(res.Moved) != 0 {
		t.Fatalf("a draft with no sprint journals nothing to push: %v %+v", h.jira.pushed, res.Moved)
	}
	if len(res.Created) != 1 || res.Remaining != 0 {
		t.Errorf("the create still landed on its own: %+v", res)
	}
}

func TestACreateThatFailedLeavesItsSprintForTheNextCommit(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	temp, err := h.repo.CreateDraft(ctx, "p1", "PLAT", draftInSprint("42", "Sprint 42"))
	if err != nil {
		t.Fatal(err)
	}
	h.jira.createErr = errors.New("Jira is unreachable")

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 1 || res.Failures[0].Key != temp || !res.Failures[0].Retryable {
		t.Fatalf("the create failed and is worth retrying: %+v", res.Failures)
	}
	if len(h.jira.pushed) != 0 {
		t.Fatalf("nothing is moved for an issue Jira does not hold: %v", h.jira.pushed)
	}
	pend, err := h.repo.PendingForKey(ctx, "p1", temp)
	if err != nil {
		t.Fatal(err)
	}
	if len(pend) != 1 || pend[0].EntityType != issuerepo.EntityIssueCreate {
		t.Fatalf("the create row is all that waits, the sprint is still inside it: %+v", pend)
	}
	if !strings.Contains(pend[0].AfterVal, `"sprintId":"42"`) {
		t.Errorf("the draft kept its sprint: %s", pend[0].AfterVal)
	}

	h.jira.createErr = nil
	res, err = h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Created) != 1 || len(h.jira.pushed) != 1 || h.jira.pushed[0] != "sprint 42 PLAT-501" {
		t.Fatalf("the next Commit creates it and moves it: %+v %v", res, h.jira.pushed)
	}
}
