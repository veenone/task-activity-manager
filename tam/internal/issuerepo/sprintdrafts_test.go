package issuerepo_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"testing"
	"time"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

func sprint15() issuerepo.DraftSprint {
	return issuerepo.DraftSprint{
		BoardID: 1, BoardName: "PLAT Scrum", Name: "Sprint 15", Goal: "Ship promos",
		StartDate: "2026-09-16T09:00:00.000+0000", EndDate: "2026-09-30T09:00:00.000+0000",
	}
}

// rowOf finds the one pending row of an entity type under a key.
func rowOf(t *testing.T, repo *issuerepo.Repository, key, entityType string) (journal.PendingChange, bool) {
	t.Helper()
	rows, err := repo.PendingForKey(context.Background(), "p1", key)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range rows {
		if p.EntityType == entityType {
			return p, true
		}
	}
	return journal.PendingChange{}, false
}

func TestADraftSprintTakesANegativeIdThatIsNeverHandedOutTwice(t *testing.T) {
	repo, db := newRepoWithDB(t)
	ctx := context.Background()
	first, err := repo.CreateDraftSprint(ctx, "p1", sprint15())
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.CreateDraftSprint(ctx, "p1", issuerepo.DraftSprint{BoardID: 1, Name: "Sprint 16"})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != -1 || second.ID != -2 || first.State != "future" || first.BoardID != 1 || first.Goal != "Ship promos" {
		t.Fatalf("first = %+v, second = %+v", first, second)
	}
	if err := repo.DiscardDraftSprint(ctx, "p1", second.ID); err != nil {
		t.Fatal(err)
	}
	third, err := repo.CreateDraftSprint(ctx, "p1", issuerepo.DraftSprint{BoardID: 1, Name: "Sprint 17"})
	if err != nil || third.ID != -3 {
		t.Fatalf("third = %+v, %v; a discarded id is never handed out again, so nothing stale can attach to a new draft", third, err)
	}

	var draft int
	var name string
	if err := db.QueryRow(`SELECT draft, name FROM sprint WHERE profile_id = 'p1' AND id = -1`).Scan(&draft, &name); err != nil || draft != 1 || name != "Sprint 15" {
		t.Errorf("sprint row = draft %d %q, %v", draft, name, err)
	}
	var gone int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sprint WHERE profile_id = 'p1' AND id = -2`).Scan(&gone); err != nil || gone != 0 {
		t.Errorf("the discarded draft's row is gone: %d, %v", gone, err)
	}

	rows, err := repo.ListPendingChanges(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, p := range rows {
		if p.EntityType == issuerepo.EntitySprintCreate {
			keys = append(keys, p.EntityKey)
		}
	}
	sort.Strings(keys)
	if len(keys) != 2 || keys[0] != "-1" || keys[1] != "-3" {
		t.Errorf("sprint_create keys = %v", keys)
	}
	p, _ := rowOf(t, repo, "-1", issuerepo.EntitySprintCreate)
	var carried issuerepo.DraftSprint
	if err := json.Unmarshal([]byte(p.AfterVal), &carried); err != nil || carried != sprint15() {
		t.Errorf("create row carries the draft: %+v, %v", carried, err)
	}

	if other, err := repo.CreateDraftSprint(ctx, "p2", sprint15()); err != nil || other.ID != -1 {
		t.Errorf("another profile counts from its own start: %+v, %v", other, err)
	}
	if _, err := repo.CreateDraftSprint(ctx, "p1", issuerepo.DraftSprint{BoardID: 1, Name: "  "}); err == nil {
		t.Error("a sprint with no name is refused")
	}
	if _, err := repo.CreateDraftSprint(ctx, "p1", issuerepo.DraftSprint{Name: "No board"}); err == nil {
		t.Error("a sprint with no board is refused")
	}
}

func TestEditingADraftSprintRenamesItEverywhereItIsNamed(t *testing.T) {
	repo, db := newRepoWithDB(t)
	ctx := context.Background()
	if err := repo.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatal(err)
	}
	s, err := repo.CreateDraftSprint(ctx, "p1", sprint15())
	if err != nil {
		t.Fatal(err)
	}
	id := strconv.Itoa(s.ID)
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-409", id, "Sprint 15"); err != nil {
		t.Fatal(err)
	}
	temp, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "In the draft sprint", SprintID: id, SprintName: "Sprint 15"})
	if err != nil {
		t.Fatal(err)
	}

	edit := sprint15()
	edit.Name, edit.Goal, edit.BoardID = "Sprint 15 promos", "", 99
	got, err := repo.EditDraftSprint(ctx, "p1", s.ID, edit)
	if err != nil || got.Name != "Sprint 15 promos" || got.BoardID != 1 || got.ID != s.ID {
		t.Fatalf("edit = %+v, %v; a draft keeps its id and its board", got, err)
	}

	iss, err := repo.GetIssue(ctx, "p1", "PLAT-409")
	if err != nil || iss.SprintID != id || iss.SprintName != "Sprint 15 promos" {
		t.Errorf("the card's sprint name follows: %+v, %v", iss, err)
	}
	move, ok := rowOf(t, repo, "PLAT-409", issuerepo.EntitySprintMove)
	if !ok || move.AfterVal != id+"|Sprint 15 promos" || move.BeforeVal != "12|Sprint 12" {
		t.Errorf("the journaled move names the new sprint name and keeps where it came from: %+v", move)
	}
	create, _ := rowOf(t, repo, temp, issuerepo.EntityIssueCreate)
	var d backend.IssueDraft
	if err := json.Unmarshal([]byte(create.AfterVal), &d); err != nil || d.SprintID != id || d.SprintName != "Sprint 15 promos" {
		t.Errorf("the draft issue's sprint name follows: %+v, %v", d, err)
	}
	var name, goal string
	if err := db.QueryRow(`SELECT name, goal FROM sprint WHERE profile_id = 'p1' AND id = ?`, s.ID).Scan(&name, &goal); err != nil || name != "Sprint 15 promos" || goal != "" {
		t.Errorf("sprint row = %q %q, %v", name, goal, err)
	}
	if _, err := repo.EditDraftSprint(ctx, "p1", -9, edit); !errors.Is(err, issuerepo.ErrDraftSprintGone) {
		t.Errorf("editing a draft that is not there = %v", err)
	}
}

func TestDiscardingADraftSprintPutsEveryCardItHeldBack(t *testing.T) {
	repo, db := newRepoWithDB(t)
	ctx := context.Background()
	if err := repo.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatal(err)
	}
	s, _ := repo.CreateDraftSprint(ctx, "p1", sprint15())
	id := strconv.Itoa(s.ID)
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-409", id, "Sprint 15"); err != nil {
		t.Fatal(err)
	}
	temp, _ := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "In the draft sprint", SprintID: id, SprintName: "Sprint 15"})

	create, ok := rowOf(t, repo, id, issuerepo.EntitySprintCreate)
	if !ok {
		t.Fatal("no sprint_create row")
	}
	if err := repo.DiscardPendingChange(ctx, "p1", create.ID); err != nil {
		t.Fatal(err)
	}

	if iss, _ := repo.GetIssue(ctx, "p1", "PLAT-409"); iss.SprintID != "12" || iss.SprintName != "Sprint 12" {
		t.Errorf("the card is back in the sprint it came from: %+v", iss)
	}
	if _, ok := rowOf(t, repo, "PLAT-409", issuerepo.EntitySprintMove); ok {
		t.Error("the move into the discarded sprint is gone from the journal")
	}
	if iss, _ := repo.GetIssue(ctx, "p1", temp); iss.SprintID != "" || iss.SprintName != "" {
		t.Errorf("the draft issue left the sprint: %+v", iss)
	}
	draftRow, _ := rowOf(t, repo, temp, issuerepo.EntityIssueCreate)
	var d backend.IssueDraft
	_ = json.Unmarshal([]byte(draftRow.AfterVal), &d)
	if d.SprintID != "" || d.SprintName != "" {
		t.Errorf("the draft issue's JSON left the sprint: %+v", d)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sprint WHERE profile_id = 'p1' AND id = ?`, s.ID).Scan(&n); err != nil || n != 0 {
		t.Errorf("the draft sprint row is gone: %d, %v", n, err)
	}
	if err := repo.DiscardDraftSprint(ctx, "p1", s.ID); !errors.Is(err, issuerepo.ErrDraftSprintGone) {
		t.Errorf("a second discard = %v", err)
	}
}

// journal.List orders newest first, by created_at then by id. journal.Put
// (movevalue.go's recordMove, which every board move including the second
// MoveToSprint below goes through) resets created_at to "now" on every
// update, not just on insert, so the issue_sprint row here does not keep the
// earlier timestamp its first Put wrote: after the second move it is exactly
// as new as the sprint_create row that was journaled between the two calls,
// and which of the two sorts first is a real second-boundary race, not a
// guarantee. pinCreatedAt below pins the issue_sprint row's created_at to a
// fixed past instant, so the sprint_create row - newer regardless of when
// this test happens to run - sorts first deterministically; its discard
// takes that move with it, and Discard all must not trip over the row it
// already removed.
func TestDiscardAllWithADraftSprintRevertsEachChangeOnce(t *testing.T) {
	repo, db := newRepoWithDB(t)
	ctx := context.Background()
	if err := repo.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatal(err)
	}
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-409", "13", "Sprint 13"); err != nil {
		t.Fatal(err)
	}
	s, _ := repo.CreateDraftSprint(ctx, "p1", sprint15())
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-409", strconv.Itoa(s.ID), "Sprint 15"); err != nil {
		t.Fatal(err)
	}
	pinCreatedAt(t, db, "p1", issuerepo.EntitySprintMove, "PLAT-409", "2000-01-01T00:00:00Z")

	n, err := repo.DiscardAllPendingChanges(ctx, "p1")
	if err != nil {
		t.Fatalf("discard all: %v", err)
	}
	if n != 1 {
		t.Errorf("discarded = %d, want 1: the sprint_create row sorts first (pinned older), and its own discard cascades the move away before the loop reaches it", n)
	}
	if iss, _ := repo.GetIssue(ctx, "p1", "PLAT-409"); iss.SprintID != "12" {
		t.Errorf("the card is back where it started: %+v", iss)
	}
	if rows, _ := repo.ListPendingChanges(ctx, "p1"); len(rows) != 0 {
		t.Errorf("nothing pending: %+v", rows)
	}
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-409", 0)
	discards := 0
	for _, a := range act {
		if a.Action == "discard" {
			discards++
		}
	}
	if discards != 1 {
		t.Errorf("the move was discarded once, not twice: %+v", act)
	}
}

// TestDiscardAllCountsTheMoveAndTheDraftSprintSeparatelyWhenTheMoveSortsFirst
// is the ordering TestDiscardAllWithADraftSprintRevertsEachChangeOnce pins
// away: here the issue_sprint row is pinned newer than the sprint_create
// row, so it sorts first in journal.List and is discarded (an ordinary
// revert, no cascade) before the loop ever reaches the sprint_create row.
// The sprint_create row is then still there for its own turn, and
// discardDraftSprint's walk over the journal finds no issue_sprint row left
// naming it, so it deletes only the sprint row itself. Both are found and
// discarded on their own turn, so the count is 2, not 1: which row sorts
// first changes what the cascade catches, never how many rows the profile
// ends up with reverted.
func TestDiscardAllCountsTheMoveAndTheDraftSprintSeparatelyWhenTheMoveSortsFirst(t *testing.T) {
	repo, db := newRepoWithDB(t)
	ctx := context.Background()
	if err := repo.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatal(err)
	}
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-409", "13", "Sprint 13"); err != nil {
		t.Fatal(err)
	}
	s, _ := repo.CreateDraftSprint(ctx, "p1", sprint15())
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-409", strconv.Itoa(s.ID), "Sprint 15"); err != nil {
		t.Fatal(err)
	}
	pinCreatedAt(t, db, "p1", issuerepo.EntitySprintMove, "PLAT-409", "2099-01-01T00:00:00Z")

	n, err := repo.DiscardAllPendingChanges(ctx, "p1")
	if err != nil {
		t.Fatalf("discard all: %v", err)
	}
	if n != 2 {
		t.Errorf("discarded = %d, want 2: the move sorts first (pinned newer) and is discarded on its own turn, leaving the sprint_create row to be found and discarded on its own turn right after", n)
	}
	if iss, _ := repo.GetIssue(ctx, "p1", "PLAT-409"); iss.SprintID != "12" {
		t.Errorf("the card is back where it started: %+v", iss)
	}
	if rows, _ := repo.ListPendingChanges(ctx, "p1"); len(rows) != 0 {
		t.Errorf("nothing pending: %+v", rows)
	}
	var gone int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sprint WHERE profile_id = 'p1' AND id = ?`, s.ID).Scan(&gone); err != nil || gone != 0 {
		t.Errorf("the draft sprint row is gone: %d, %v", gone, err)
	}
}

// pinCreatedAt overwrites one pending_change row's created_at directly, so a
// test can force journal.List's newest-first order (created_at DESC, id
// DESC) without depending on which side of a wall-clock second two journal
// writes happen to land on.
func pinCreatedAt(t *testing.T, db *sql.DB, profileID, entityType, entityKey, createdAt string) {
	t.Helper()
	res, err := db.Exec(
		`UPDATE pending_change SET created_at = ? WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
		createdAt, profileID, entityType, entityKey)
	if err != nil {
		t.Fatalf("pin created_at: %v", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		t.Fatalf("pin created_at: affected %d rows, want 1", n)
	}
}
