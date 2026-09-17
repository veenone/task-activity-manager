package sprints_test

import (
	"context"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/sprints"
)

func TestDraftSprintConvertsTheDatesAndRefusesWhatCannotBeASprint(t *testing.T) {
	d, err := sprints.DraftSprint(backend.SprintDraft{Name: "  Sprint 15 ", Goal: "Ship promos", StartDate: "2026-09-16", EndDate: "2026-09-30"})
	if err != nil {
		t.Fatalf("DraftSprint: %v", err)
	}
	if d.Name != "Sprint 15" || !strings.HasPrefix(d.StartDate, "2026-09-16T") || !strings.HasPrefix(d.EndDate, "2026-09-30T") {
		t.Errorf("draft = %+v, want a trimmed name and the Agile API's own datetimes", d)
	}
	if _, err := sprints.DraftSprint(backend.SprintDraft{Name: " ", StartDate: "2026-09-16", EndDate: "2026-09-30"}); err == nil {
		t.Error("a sprint with no name is refused")
	}
	if _, err := sprints.DraftSprint(backend.SprintDraft{Name: "Sprint 15", StartDate: "2026-09-30", EndDate: "2026-09-16"}); err == nil {
		t.Error("a sprint that ends before it starts is refused")
	}
}

// A draft sprint's id is negative and means nothing to Jira. None of the
// writes to a real sprint may send it, and a completion may not push cards
// into one, whichever path reached the service.
func TestNoImmediateWriteReachesJiraForADraftSprint(t *testing.T) {
	b := &fakeBackend{sprints: []backend.Sprint{{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active"}}}
	store := newStore()
	s := manageService(b, store, newIssues(store))
	ctx := context.Background()

	if _, err := sprints.ForCommit(s).Start(ctx, "p1", 1, -1, draft("Sprint 15", "")); err == nil || !strings.Contains(err.Error(), "draft") {
		t.Errorf("start = %v", err)
	}
	if _, err := sprints.ForCommit(s).Edit(ctx, "p1", 1, -1, draft("Sprint 15", ""), false); err == nil || !strings.Contains(err.Error(), "draft") {
		t.Errorf("edit = %v", err)
	}
	if _, err := sprints.ForCommit(s).Delete(ctx, "p1", 1, -1); err == nil || !strings.Contains(err.Error(), "draft") {
		t.Errorf("delete = %v", err)
	}
	if _, err := sprints.ForCommit(s).Complete(ctx, "p1", 1, -1, ""); err == nil || !strings.Contains(err.Error(), "draft") {
		t.Errorf("complete = %v", err)
	}
	if _, err := sprints.ForCommit(s).Complete(ctx, "p1", 1, 12, "-1"); err == nil || !strings.Contains(err.Error(), "draft sprint") {
		t.Errorf("complete into a draft = %v", err)
	}
	if b.sprintReads != 0 || len(b.starts) != 0 || len(b.edited) != 0 || len(b.deleted) != 0 || len(b.completed) != 0 || len(b.moves) != 0 {
		t.Errorf("Jira was asked something: reads %d, starts %v, edits %v, deletes %v, completes %v, moves %v",
			b.sprintReads, b.starts, b.edited, b.deleted, b.completed, b.moves)
	}
}
