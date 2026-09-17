package issuerepo_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

func start12(name string) issuerepo.SprintStart {
	return issuerepo.SprintStart{BoardID: 1, Name: name, Goal: "Ship", StartDate: "2026-09-14T09:00:00.000+0000", EndDate: "2026-09-28T09:00:00.000+0000"}
}

func TestStartingAFutureSprintJournalsOneRowAndLeavesItFuture(t *testing.T) {
	repo, db := seedSprint12(t, "future")
	ctx := context.Background()
	if err := repo.JournalSprintStart(ctx, "p1", 12, start12("Sprint 12")); err != nil {
		t.Fatal(err)
	}
	if err := repo.JournalSprintStart(ctx, "p1", 12, start12("  Sprint 12b ")); err != nil {
		t.Fatal(err)
	}
	row, ok := rowOf(t, repo, "12", issuerepo.EntitySprintStart)
	if !ok {
		t.Fatal("no sprint_start row")
	}
	var after issuerepo.SprintStart
	if err := json.Unmarshal([]byte(row.AfterVal), &after); err != nil || after != start12("Sprint 12b") {
		t.Errorf("after_val = %s, %v; the second start replaces the first", row.AfterVal, err)
	}
	if rows, _ := repo.ListPendingChanges(ctx, "p1"); len(rows) != 1 {
		t.Errorf("one row per sprint, got %d", len(rows))
	}
	var state, name string
	if err := db.QueryRow(`SELECT state, name FROM sprint WHERE profile_id = 'p1' AND id = 12 AND board_id = 1`).Scan(&state, &name); err != nil || state != "future" || name != "Sprint 12" {
		t.Errorf("the cached sprint = %q %q, %v; want it untouched until Commit", state, name, err)
	}
	if err := repo.DiscardPendingChange(ctx, "p1", row.ID); err != nil {
		t.Fatal(err)
	}
	if rows, _ := repo.ListPendingChanges(ctx, "p1"); len(rows) != 0 {
		t.Errorf("discard drops the start: %+v", rows)
	}
}

func TestSprintStartRefusalsAreLocalAndJournalNothing(t *testing.T) {
	ctx := context.Background()
	for _, state := range []string{"active", "closed"} {
		repo, _ := seedSprint12(t, state)
		if err := repo.JournalSprintStart(ctx, "p1", 12, start12("Sprint 12")); err == nil || !strings.Contains(err.Error(), "never been started") {
			t.Errorf("starting a %s sprint = %v", state, err)
		}
		if rows, _ := repo.ListPendingChanges(ctx, "p1"); len(rows) != 0 {
			t.Errorf("%s: nothing journalled: %+v", state, rows)
		}
	}
	repo, _ := seedSprint12(t, "future")
	if err := repo.JournalSprintStart(ctx, "p1", 12, start12(" ")); err == nil {
		t.Error("a start with no name was journalled")
	}
	if err := repo.JournalSprintStart(ctx, "p1", 77, start12("Sprint 77")); err == nil {
		t.Error("a sprint the cache does not hold was queued to start")
	}
	if err := repo.JournalSprintStart(ctx, "p1", -5, start12("Draft")); !errors.Is(err, issuerepo.ErrDraftSprintGone) {
		t.Errorf("a draft nobody drafted = %v", err)
	}
	if err := repo.JournalSprintDelete(ctx, "p1", 1, 12); err != nil {
		t.Fatal(err)
	}
	if err := repo.JournalSprintStart(ctx, "p1", 12, start12("Sprint 12")); err == nil || !strings.Contains(err.Error(), "waiting to be deleted") {
		t.Errorf("a start while the delete waits = %v", err)
	}
	if _, ok := rowOf(t, repo, "12", issuerepo.EntitySprintStart); ok {
		t.Error("a refused start was journalled")
	}
}

func TestADraftSprintsStartFollowsItToTheRealIdAndGoesWithADiscard(t *testing.T) {
	repo, _ := newRepoWithDB(t)
	ctx := context.Background()
	draft, err := repo.CreateDraftSprint(ctx, "p1", issuerepo.DraftSprint{BoardID: 3, BoardName: "B", Name: "Sprint 20"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := repo.CreateDraftSprint(ctx, "p1", issuerepo.DraftSprint{BoardID: 3, BoardName: "B", Name: "Sprint 21"})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []int{draft.ID, other.ID} {
		s := start12("Sprint 20")
		s.BoardID = 99 // the draft's own board wins
		if err := repo.JournalSprintStart(ctx, "p1", id, s); err != nil {
			t.Fatal(err)
		}
	}

	if err := repo.RekeySprint(ctx, "p1", draft.ID, backend.Sprint{ID: 120, BoardID: 3, Name: "Sprint 20"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := rowOf(t, repo, "-1", issuerepo.EntitySprintStart); ok {
		t.Error("the start stayed under the draft id")
	}
	row, ok := rowOf(t, repo, "120", issuerepo.EntitySprintStart)
	var after issuerepo.SprintStart
	if !ok || json.Unmarshal([]byte(row.AfterVal), &after) != nil || after.BoardID != 3 || after.Name != "Sprint 20" {
		t.Errorf("the start under the real id = %+v", row)
	}

	if err := repo.DiscardDraftSprint(ctx, "p1", other.ID); err != nil {
		t.Fatal(err)
	}
	rows, _ := repo.ListPendingChanges(ctx, "p1")
	if len(rows) != 1 || rows[0].EntityKey != "120" {
		t.Errorf("discarding the other draft takes its start with it: %+v", rows)
	}
}

func TestCompletingASprintJournalsTheIntentNamedFromTheCache(t *testing.T) {
	repo, db := seedSprint12(t, "active")
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO sprint (profile_id, id, board_id, name, state) VALUES ('p1', 13, 1, 'Sprint 13', 'future')`); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		moveTo, name string
	}{{"13", "Sprint 13"}, {"", "the backlog"}, {"88", "sprint 88"}} {
		if err := repo.JournalSprintComplete(ctx, "p1", 12, issuerepo.SprintComplete{BoardID: 1, Name: "ignored", MoveTo: c.moveTo, PreviewCount: 4}); err != nil {
			t.Fatal(err)
		}
		row, _ := rowOf(t, repo, "12", issuerepo.EntitySprintComplete)
		var after issuerepo.SprintComplete
		want := issuerepo.SprintComplete{BoardID: 1, Name: "Sprint 12", MoveTo: c.moveTo, MoveToName: c.name, PreviewCount: 4}
		if err := json.Unmarshal([]byte(row.AfterVal), &after); err != nil || after != want {
			t.Errorf("after_val = %s, %v; want %+v", row.AfterVal, err, want)
		}
	}
	var state string
	if err := db.QueryRow(`SELECT state FROM sprint WHERE profile_id = 'p1' AND id = 12 AND board_id = 1`).Scan(&state); err != nil || state != "active" {
		t.Errorf("the cached sprint = %q, %v; want it active until Commit", state, err)
	}
}

func TestSprintCompleteRefusalsAndTheDeleteTheyBlock(t *testing.T) {
	ctx := context.Background()
	repo, _ := seedSprint12(t, "future")
	if err := repo.JournalSprintComplete(ctx, "p1", -1, issuerepo.SprintComplete{BoardID: 1}); err == nil {
		t.Error("a draft sprint was queued to complete")
	}
	if err := repo.JournalSprintDelete(ctx, "p1", 1, 12); err != nil {
		t.Fatal(err)
	}
	if err := repo.JournalSprintComplete(ctx, "p1", 12, issuerepo.SprintComplete{BoardID: 1}); err == nil || !strings.Contains(err.Error(), "waiting to be deleted") {
		t.Errorf("a completion while the delete waits = %v", err)
	}
	if _, ok := rowOf(t, repo, "12", issuerepo.EntitySprintComplete); ok {
		t.Error("a refused completion was journalled")
	}

	for entity, journalIt := range map[string]func(*issuerepo.Repository) error{
		"started": func(r *issuerepo.Repository) error { return r.JournalSprintStart(ctx, "p1", 12, start12("Sprint 12")) },
		"completed": func(r *issuerepo.Repository) error {
			return r.JournalSprintComplete(ctx, "p1", 12, issuerepo.SprintComplete{BoardID: 1})
		},
	} {
		repo, _ := seedSprint12(t, "future")
		if err := journalIt(repo); err != nil {
			t.Fatal(err)
		}
		if err := repo.JournalSprintDelete(ctx, "p1", 1, 12); err == nil || !strings.Contains(err.Error(), "waiting to be "+entity) {
			t.Errorf("a delete while the sprint waits to be %s = %v", entity, err)
		}
		if _, ok := rowOf(t, repo, "12", issuerepo.EntitySprintDelete); ok {
			t.Errorf("a delete was journalled while the sprint waits to be %s", entity)
		}
	}
}
