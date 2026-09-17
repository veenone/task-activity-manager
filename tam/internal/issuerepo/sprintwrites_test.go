package issuerepo_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"agile-suite/tam/internal/issuerepo"
)

// seedSprint12 caches PLAT's issues and sprint 12 as board 1 and board 2 both
// hold it, in the given state.
func seedSprint12(t *testing.T, state string) (*issuerepo.Repository, *sql.DB) {
	t.Helper()
	repo, db := newRepoWithDB(t)
	if err := repo.UpsertPage(context.Background(), "p1", sample(), time.Now(), false); err != nil {
		t.Fatal(err)
	}
	for _, board := range []int{1, 2} {
		if _, err := db.Exec(`INSERT INTO sprint (profile_id, id, board_id, name, state, start_date, end_date, goal)
			VALUES ('p1', 12, ?, 'Sprint 12', ?, '2026-09-01T09:00:00.000+0000', '2026-09-14T09:00:00.000+0000', 'Old goal')`, board, state); err != nil {
			t.Fatal(err)
		}
	}
	return repo, db
}

func sprintRow(t *testing.T, db *sql.DB, board int) (name, goal, start, end string) {
	t.Helper()
	if err := db.QueryRow(`SELECT name, goal, start_date, end_date FROM sprint WHERE profile_id = 'p1' AND id = 12 AND board_id = ?`, board).
		Scan(&name, &goal, &start, &end); err != nil {
		t.Fatal(err)
	}
	return
}

func edit12(name, goal string, clearGoal bool) issuerepo.SprintEdit {
	return issuerepo.SprintEdit{
		BoardID: 1, Name: name, Goal: goal, ClearGoal: clearGoal,
		StartDate: "2026-09-02T09:00:00.000+0000", EndDate: "2026-09-16T09:00:00.000+0000",
	}
}

func TestEditingARealSprintJournalsOneRowAndChangesTheCacheAtOnce(t *testing.T) {
	repo, db := seedSprint12(t, "active")
	ctx := context.Background()
	if err := repo.JournalSprintEdit(ctx, "p1", 12, edit12("Sprint 12b", "", false)); err != nil {
		t.Fatal(err)
	}
	if err := repo.JournalSprintEdit(ctx, "p1", 12, edit12("Sprint 12c", "New goal", false)); err != nil {
		t.Fatal(err)
	}

	row, ok := rowOf(t, repo, "12", issuerepo.EntitySprintEdit)
	if !ok {
		t.Fatal("no sprint_edit row")
	}
	var after issuerepo.SprintEdit
	if err := json.Unmarshal([]byte(row.AfterVal), &after); err != nil || after.Name != "Sprint 12c" || after.Goal != "New goal" || after.BoardID != 1 {
		t.Errorf("after_val = %s, %v; the second edit replaces the first", row.AfterVal, err)
	}
	var before map[string]string
	if err := json.Unmarshal([]byte(row.BeforeVal), &before); err != nil || before["name"] != "Sprint 12" || before["goal"] != "Old goal" {
		t.Errorf("before_val = %s, %v; the first edit's before value is kept", row.BeforeVal, err)
	}
	rows, _ := repo.ListPendingChanges(ctx, "p1")
	if len(rows) != 1 {
		t.Errorf("one row per sprint, got %d", len(rows))
	}
	for _, board := range []int{1, 2} {
		name, goal, start, _ := sprintRow(t, db, board)
		if name != "Sprint 12c" || goal != "New goal" || start != "2026-09-02T09:00:00.000+0000" {
			t.Errorf("board %d's cached sprint = %q %q %q", board, name, goal, start)
		}
	}
	iss, err := repo.GetIssue(ctx, "p1", "PLAT-409")
	if err != nil || iss.SprintName != "Sprint 12c" || iss.SprintID != "12" {
		t.Errorf("the card follows the rename: %+v, %v", iss, err)
	}
}

func TestAnEmptyGoalLeavesTheCachedGoalAndClearGoalRemovesIt(t *testing.T) {
	repo, db := seedSprint12(t, "future")
	ctx := context.Background()
	if err := repo.JournalSprintEdit(ctx, "p1", 12, edit12("Sprint 12", "", false)); err != nil {
		t.Fatal(err)
	}
	if _, goal, _, _ := sprintRow(t, db, 1); goal != "Old goal" {
		t.Errorf("an empty goal leaves the goal alone, as Jira's partial update does: %q", goal)
	}
	if err := repo.JournalSprintEdit(ctx, "p1", 12, edit12("Sprint 12", "", true)); err != nil {
		t.Fatal(err)
	}
	if _, goal, _, _ := sprintRow(t, db, 1); goal != "" {
		t.Errorf("clearGoal removes it: %q", goal)
	}
	// The dialog reopens on the cleared goal and so sends clearGoal false; the
	// earlier clear still has to reach Jira.
	if err := repo.JournalSprintEdit(ctx, "p1", 12, edit12("Sprint 12 renamed", "", false)); err != nil {
		t.Fatal(err)
	}
	row, _ := rowOf(t, repo, "12", issuerepo.EntitySprintEdit)
	var after issuerepo.SprintEdit
	if err := json.Unmarshal([]byte(row.AfterVal), &after); err != nil || !after.ClearGoal {
		t.Errorf("a later edit keeps the earlier clear: %s, %v", row.AfterVal, err)
	}
}

func TestDiscardingASprintEditRestoresTheSprintAndItsCards(t *testing.T) {
	repo, db := seedSprint12(t, "future")
	ctx := context.Background()
	if err := repo.JournalSprintEdit(ctx, "p1", 12, edit12("Sprint 12b", "", true)); err != nil {
		t.Fatal(err)
	}
	row, _ := rowOf(t, repo, "12", issuerepo.EntitySprintEdit)
	if err := repo.DiscardPendingChange(ctx, "p1", row.ID); err != nil {
		t.Fatal(err)
	}
	for _, board := range []int{1, 2} {
		name, goal, start, end := sprintRow(t, db, board)
		if name != "Sprint 12" || goal != "Old goal" || start != "2026-09-01T09:00:00.000+0000" || end != "2026-09-14T09:00:00.000+0000" {
			t.Errorf("board %d's sprint = %q %q %q %q", board, name, goal, start, end)
		}
	}
	if iss, err := repo.GetIssue(ctx, "p1", "PLAT-409"); err != nil || iss.SprintName != "Sprint 12" {
		t.Errorf("the card's sprint name goes back: %+v, %v", iss, err)
	}
	if rows, _ := repo.ListPendingChanges(ctx, "p1"); len(rows) != 0 {
		t.Errorf("the row is gone: %+v", rows)
	}
}

func TestSprintEditRefusalsAreLocalAndJournalNothing(t *testing.T) {
	ctx := context.Background()
	closed, _ := seedSprint12(t, "closed")
	if err := closed.JournalSprintEdit(ctx, "p1", 12, edit12("x", "", false)); err == nil {
		t.Error("a closed sprint was edited")
	}
	if err := closed.JournalSprintEdit(ctx, "p1", 77, edit12("x", "", false)); err == nil {
		t.Error("a sprint the cache does not hold was edited")
	}
	if rows, _ := closed.ListPendingChanges(ctx, "p1"); len(rows) != 0 {
		t.Errorf("nothing journalled: %+v", rows)
	}

	deleting, _ := seedSprint12(t, "future")
	if err := deleting.JournalSprintDelete(ctx, "p1", 1, 12); err != nil {
		t.Fatal(err)
	}
	if err := deleting.JournalSprintEdit(ctx, "p1", 12, edit12("x", "", false)); err == nil {
		t.Error("a sprint with a pending delete was edited")
	}
	if _, ok := rowOf(t, deleting, "12", issuerepo.EntitySprintEdit); ok {
		t.Error("an edit row was journalled beside the delete")
	}
}

func TestDeletingARealSprintJournalsItAndLeavesTheCacheUntilCommit(t *testing.T) {
	repo, db := seedSprint12(t, "future")
	ctx := context.Background()
	if err := repo.JournalSprintDelete(ctx, "p1", 1, 12); err != nil {
		t.Fatal(err)
	}
	row, ok := rowOf(t, repo, "12", issuerepo.EntitySprintDelete)
	if !ok || row.BeforeVal != "Sprint 12" {
		t.Fatalf("delete row = %+v", row)
	}
	var after struct {
		BoardID int    `json:"boardId"`
		Name    string `json:"name"`
	}
	if err := json.Unmarshal([]byte(row.AfterVal), &after); err != nil || after.BoardID != 1 || after.Name != "Sprint 12" {
		t.Errorf("after_val = %s, %v", row.AfterVal, err)
	}
	if name, _, _, _ := sprintRow(t, db, 1); name != "Sprint 12" {
		t.Errorf("the sprint row stays until Commit: %q", name)
	}
	if iss, _ := repo.GetIssue(ctx, "p1", "PLAT-409"); iss.SprintID != "12" {
		t.Errorf("the cards stay in the sprint until Commit: %+v", iss)
	}
	if err := repo.DiscardPendingChange(ctx, "p1", row.ID); err != nil {
		t.Fatal(err)
	}
	if rows, _ := repo.ListPendingChanges(ctx, "p1"); len(rows) != 0 {
		t.Errorf("discard drops the delete: %+v", rows)
	}
	if name, _, _, _ := sprintRow(t, db, 1); name != "Sprint 12" {
		t.Errorf("the sprint is still there after the discard: %q", name)
	}
}

func TestADeleteSupersedesAPendingEdit(t *testing.T) {
	repo, db := seedSprint12(t, "future")
	ctx := context.Background()
	if err := repo.JournalSprintEdit(ctx, "p1", 12, edit12("Sprint 12b", "", false)); err != nil {
		t.Fatal(err)
	}
	if err := repo.JournalSprintDelete(ctx, "p1", 1, 12); err != nil {
		t.Fatal(err)
	}
	if _, ok := rowOf(t, repo, "12", issuerepo.EntitySprintEdit); ok {
		t.Error("the edit row is still pending")
	}
	row, ok := rowOf(t, repo, "12", issuerepo.EntitySprintDelete)
	if !ok || row.BeforeVal != "Sprint 12" {
		t.Errorf("the delete names the sprint as Jira has it: %+v", row)
	}
	if name, _, _, _ := sprintRow(t, db, 1); name != "Sprint 12" {
		t.Errorf("the edit was reverted: %q", name)
	}
}

func TestSprintDeleteRefusesAnythingButAFutureSprint(t *testing.T) {
	ctx := context.Background()
	for _, state := range []string{"active", "closed"} {
		repo, _ := seedSprint12(t, state)
		if err := repo.JournalSprintDelete(ctx, "p1", 1, 12); err == nil {
			t.Errorf("a %s sprint was queued for deletion", state)
		}
		if rows, _ := repo.ListPendingChanges(ctx, "p1"); len(rows) != 0 {
			t.Errorf("%s: nothing journalled: %+v", state, rows)
		}
	}
	repo, _ := seedSprint12(t, "future")
	if err := repo.JournalSprintDelete(ctx, "p1", 1, 77); err == nil {
		t.Error("a sprint the cache does not hold was queued for deletion")
	}
}
