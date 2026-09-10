package issuerepo_test

import (
	"context"
	"testing"
)

// TestAnAuditedSprintDeleteIsReadableByTheSprintsOwnId pins the two things
// the sprint write path relies on: the row lands, and the id it is keyed by
// is what finds it again. That id is also why no screen shows the row, since
// the Activity tab only ever asks with an issue key, which is the reason the
// delete confirmation must not promise an activity log.
func TestAnAuditedSprintDeleteIsReadableByTheSprintsOwnId(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()

	if err := repo.AuditSprint(ctx, "p1", 12, "delete", "Sprint 12", ""); err != nil {
		t.Fatalf("audit sprint: %v", err)
	}

	rows, err := repo.ListActivity(ctx, "p1", "12", 10)
	if err != nil {
		t.Fatalf("list activity: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("read back %d rows, want the one the delete wrote", len(rows))
	}
	got := rows[0]
	if got.EntityType != "sprint" || got.Action != "delete" || got.BeforeVal != "Sprint 12" || got.AfterVal != "" {
		t.Errorf("row = %+v, want a sprint delete carrying the name it had", got)
	}
	if got.Actor == "" || got.OccurredAt == "" {
		t.Errorf("row = %+v, want the actor and the time stamped on it", got)
	}
}

// TestASprintsAuditRowIsNotReturnedForAnIssue keeps the entity key honest: a
// sprint id and an issue key are different keys, so an issue's Activity tab
// never shows a sprint's rows and never has to explain them.
func TestASprintsAuditRowIsNotReturnedForAnIssue(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()

	if err := repo.AuditSprint(ctx, "p1", 12, "create", "", "Sprint 12"); err != nil {
		t.Fatalf("audit sprint: %v", err)
	}

	rows, err := repo.ListActivity(ctx, "p1", "PLAT-12", 10)
	if err != nil {
		t.Fatalf("list activity: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("PLAT-12 read back %d rows, want none of the sprint's", len(rows))
	}
}
