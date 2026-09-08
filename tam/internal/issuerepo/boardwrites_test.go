package issuerepo_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// v2 is a later remote version than v1, for the case where Jira has moved on
// under a pending move.
const v2 = "2026-09-02T00:00:00Z"

// seedBoardCards writes three cards that between them name every status and
// sprint the board tests move between, so the store can find a status name
// for a drop target the way it does in the app: from another issue that
// already carries that status.
func seedBoardCards(t *testing.T, repo *issuerepo.Repository) {
	t.Helper()
	rows := []backend.Issue{
		{Key: "PLAT-1", ID: "101", Project: "PLAT", Type: backend.TypeStory, Summary: "one",
			Status: "To Do", StatusID: "1", SprintID: "12", SprintName: "Sprint 12", Rank: "0|a", Updated: v1},
		{Key: "PLAT-2", ID: "102", Project: "PLAT", Type: backend.TypeStory, Summary: "two",
			Status: "In Progress", StatusID: "3", SprintID: "12", SprintName: "Sprint 12", Rank: "0|b", Updated: v1},
		{Key: "PLAT-3", ID: "103", Project: "PLAT", Type: backend.TypeTask, Summary: "three",
			Status: "Done", StatusID: "5", Rank: "0|c", Updated: v1},
	}
	if err := repo.UpsertPage(context.Background(), "p1", rows, time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// oneRow is the profile's single pending change, or a failure naming what
// was there instead.
func oneRow(t *testing.T, repo *issuerepo.Repository, key string) journal.PendingChange {
	t.Helper()
	rows, err := repo.PendingForKey(context.Background(), "p1", key)
	if err != nil {
		t.Fatalf("pending for %s: %v", key, err)
	}
	if len(rows) != 1 {
		t.Fatalf("pending rows of %s = %+v, want exactly one", key, rows)
	}
	return rows[0]
}

func TestMoveToColumnJournalsTheTransitionAndMovesTheRow(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedBoardCards(t, repo)

	if err := repo.MoveToColumn(ctx, "p1", "PLAT-1", "3"); err != nil {
		t.Fatalf("move: %v", err)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.StatusID != "3" || iss.Status != "In Progress" || !iss.Pending {
		t.Errorf("row after the move: %+v, want the target column written locally", iss)
	}
	p := oneRow(t, repo, "PLAT-1")
	if p.EntityType != issuerepo.EntityTransition || p.Field != issuerepo.FieldStatusID {
		t.Errorf("journal row = %+v, want a transition", p)
	}
	// Both values carry "id|Name": the committer pushes the id, the Pending
	// changes dialog and the Activity tab read the name.
	if p.BeforeVal != "1|To Do" || p.AfterVal != "3|In Progress" || p.BaseVersion != v1 {
		t.Errorf("journal row = %+v, want id|Name either side against the row's updated stamp", p)
	}
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-1", 0)
	if len(act) != 1 || act[0].Action != "move" || act[0].AfterVal != "3|In Progress" {
		t.Errorf("audit = %+v", act)
	}
}

func TestMoveToColumnNamesAStatusTheCacheHasNeverSeenByItsID(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedBoardCards(t, repo)
	// A board column can collect a status no cached issue is in, and the
	// column configuration holds ids and no names.
	if err := repo.MoveToColumn(ctx, "p1", "PLAT-1", "10099"); err != nil {
		t.Fatalf("move: %v", err)
	}
	p := oneRow(t, repo, "PLAT-1")
	if issuerepo.MoveID(p.AfterVal) != "10099" {
		t.Errorf("after = %q, want the id whatever the name is", p.AfterVal)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.StatusID != "10099" {
		t.Errorf("row = %+v, want the target status id", iss)
	}
	// The name column stays empty rather than taking the id: a fabricated
	// name is what the next drop on this column would read back as the real
	// one, and IsDone reads that column too.
	if iss.Status != "" {
		t.Errorf("status name = %q, want no name invented for an id the cache has never seen", iss.Status)
	}
	if err := repo.MoveToColumn(ctx, "p1", "PLAT-2", "10099"); err != nil {
		t.Fatalf("second move: %v", err)
	}
	if p := oneRow(t, repo, "PLAT-2"); p.AfterVal != "10099|" {
		t.Errorf("after = %q, want the second drop to find no name either", p.AfterVal)
	}
}

func TestMoveToSprintJournalsTheSprintAndTheBacklog(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedBoardCards(t, repo)

	// Sprint 13 is a future sprint no issue is in yet, so no cached row can
	// name it: the caller, which holds the sprint list, hands the name in.
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-1", "13", "Sprint 13"); err != nil {
		t.Fatalf("move: %v", err)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.SprintID != "13" || iss.SprintName != "Sprint 13" {
		t.Errorf("row = %+v, want the sprint written locally", iss)
	}
	p := oneRow(t, repo, "PLAT-1")
	if p.EntityType != issuerepo.EntitySprintMove || p.Field != issuerepo.FieldSprintID {
		t.Errorf("journal row = %+v, want a sprint move", p)
	}
	if p.BeforeVal != "12|Sprint 12" || p.AfterVal != "13|Sprint 13" || p.BaseVersion != v1 {
		t.Errorf("journal row = %+v", p)
	}

	// The backlog is a destination, not an absence: the row stays, with an
	// empty value, and the columns are cleared.
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-1", "", ""); err != nil {
		t.Fatalf("backlog: %v", err)
	}
	p = oneRow(t, repo, "PLAT-1")
	if p.BeforeVal != "12|Sprint 12" || p.AfterVal != "" {
		t.Errorf("journal row = %+v, want the first before value kept and an empty target", p)
	}
	iss, _ = repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.SprintID != "" || iss.SprintName != "" {
		t.Errorf("row = %+v, want the sprint cleared", iss)
	}

	// A caller with no name to give falls back to the name a cached issue
	// in that sprint already carries.
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-3", "12", ""); err != nil {
		t.Fatalf("fallback: %v", err)
	}
	if p := oneRow(t, repo, "PLAT-3"); p.AfterVal != "12|Sprint 12" {
		t.Errorf("after = %q, want the name the cache knows for sprint 12", p.AfterVal)
	}
}

func TestRankIssueJournalsItsNeighbourAndWritesNoColumn(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedBoardCards(t, repo)

	if err := repo.RankIssue(ctx, "p1", "PLAT-1", "PLAT-2", true, 7); err != nil {
		t.Fatalf("rank: %v", err)
	}
	p := oneRow(t, repo, "PLAT-1")
	if p.EntityType != issuerepo.EntityRank || p.Field != issuerepo.FieldRank {
		t.Errorf("journal row = %+v, want a rank", p)
	}
	// The board is the third segment: one key can sit on two boards whose
	// orders disagree, and the commit pass re-derives the neighbour from
	// the order of the board the drop was made on.
	if p.BeforeVal != "" || p.AfterVal != "before|PLAT-2|7" || p.BaseVersion != v1 {
		t.Errorf("journal row = %+v, want the neighbour, its side, and its board, and no before value", p)
	}
	if key, before, boardID := issuerepo.ParseRank(p.AfterVal); key != "PLAT-2" || !before || boardID != 7 {
		t.Errorf("parsed = %q, %v, %d", key, before, boardID)
	}
	// A row journaled before the board joined the value still parses.
	if key, before, boardID := issuerepo.ParseRank("after|PLAT-9"); key != "PLAT-9" || before || boardID != 0 {
		t.Errorf("two-segment value parsed as %q, %v, %d, want board 0", key, before, boardID)
	}
	// Decision 4: a made-up LexoRank in the cache would be a second source
	// of truth the next sync overwrites.
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.Rank != "0|a" {
		t.Errorf("rank column = %q, want the synced value untouched", iss.Rank)
	}
	if !iss.Pending {
		t.Error("the row carries a pending change and must say so")
	}

	// The other side, and a second rank replacing the first.
	if err := repo.RankIssue(ctx, "p1", "PLAT-1", "PLAT-3", false, 7); err != nil {
		t.Fatalf("re-rank: %v", err)
	}
	p = oneRow(t, repo, "PLAT-1")
	if p.AfterVal != "after|PLAT-3|7" {
		t.Errorf("after = %q, want the newest drop", p.AfterVal)
	}
	// The same neighbour and side on another board is another drop, not the
	// one already journaled.
	if err := repo.RankIssue(ctx, "p1", "PLAT-1", "PLAT-3", false, 8); err != nil {
		t.Fatalf("other board: %v", err)
	}
	if p = oneRow(t, repo, "PLAT-1"); p.AfterVal != "after|PLAT-3|8" {
		t.Errorf("after = %q, want the board the newest drop was made on", p.AfterVal)
	}
}

func TestASecondMoveReplacesTheFirstAndKeepsItsBeforeValue(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedBoardCards(t, repo)

	if err := repo.MoveToColumn(ctx, "p1", "PLAT-1", "3"); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := repo.MoveToColumn(ctx, "p1", "PLAT-1", "5"); err != nil {
		t.Fatalf("second: %v", err)
	}
	p := oneRow(t, repo, "PLAT-1")
	// The before value of the *first* row has to survive, or a later
	// discard puts the card in a column it was never in.
	if p.BeforeVal != "1|To Do" || p.AfterVal != "5|Done" {
		t.Errorf("journal row = %+v, want one row from the original column to the newest target", p)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.StatusID != "5" {
		t.Errorf("row = %+v", iss)
	}
}

func TestAMoveBackToWhereItStartedDeletesTheRowAndAuditsTheUndo(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedBoardCards(t, repo)

	// PLAT-1 is the only cached issue in status 1, so once it has moved, no
	// row names that status any more and the target is journaled as a bare
	// id. The undo still has to fire: the comparison is on ids, never on
	// the id|Name text.
	if err := repo.MoveToColumn(ctx, "p1", "PLAT-1", "3"); err != nil {
		t.Fatalf("out: %v", err)
	}
	if err := repo.MoveToColumn(ctx, "p1", "PLAT-1", "1"); err != nil {
		t.Fatalf("home: %v", err)
	}
	rows, _ := repo.PendingForKey(ctx, "p1", "PLAT-1")
	if len(rows) != 0 {
		t.Errorf("pending = %+v, want the row gone rather than a change to nothing", rows)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.StatusID != "1" || iss.Status != "To Do" || iss.Pending {
		t.Errorf("row = %+v, want the original column and its name back", iss)
	}
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-1", 0)
	if len(act) != 2 || act[0].Action != "undo" || act[0].AfterVal != "1|To Do" {
		t.Errorf("audit = %+v, want the undo recorded", act)
	}

	// The same rule on a sprint move, and on ids alone: the sprint was
	// renamed in Jira between the two drops, and a rename is not a move.
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-1", "13", "Sprint 13"); err != nil {
		t.Fatalf("sprint out: %v", err)
	}
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-1", "12", "Sprint 12 (renamed)"); err != nil {
		t.Fatalf("sprint home: %v", err)
	}
	rows, _ = repo.PendingForKey(ctx, "p1", "PLAT-1")
	if len(rows) != 0 {
		t.Errorf("pending = %+v, want the sprint row gone", rows)
	}
	iss, _ = repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.SprintID != "12" || iss.SprintName != "Sprint 12" {
		t.Errorf("row = %+v, want the original sprint back", iss)
	}
}

func TestADropOnTheValueTheRowAlreadyHasChangesNothing(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedBoardCards(t, repo)

	if err := repo.MoveToColumn(ctx, "p1", "PLAT-1", "1"); err != nil {
		t.Fatalf("move: %v", err)
	}
	rows, _ := repo.PendingForKey(ctx, "p1", "PLAT-1")
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-1", 0)
	if len(rows) != 0 || len(act) != 0 {
		t.Errorf("pending = %+v, audit = %+v; want a drop that changes nothing to journal nothing", rows, act)
	}
}

func TestABoardMoveRefusesAKeyTheCacheDoesNotHold(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedBoardCards(t, repo)

	if err := repo.MoveToColumn(ctx, "p1", "PLAT-404", "3"); !errors.Is(err, issuerepo.ErrNotFound) {
		t.Errorf("move of an unknown key = %v, want ErrNotFound", err)
	}
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-404", "13", "Sprint 13"); !errors.Is(err, issuerepo.ErrNotFound) {
		t.Errorf("sprint move of an unknown key = %v, want ErrNotFound", err)
	}
	// A rank against a card nobody has seen would be pushed to Jira as a key
	// that does not resolve.
	err := repo.RankIssue(ctx, "p1", "PLAT-1", "PLAT-404", true, 1)
	if err == nil || !strings.Contains(err.Error(), "PLAT-404") {
		t.Errorf("rank against an unknown neighbour = %v, want it refused by name", err)
	}
	if err := repo.RankIssue(ctx, "p1", "PLAT-1", "PLAT-1", true, 1); err == nil {
		t.Error("an issue ranked against itself must be refused")
	}
	if err := repo.MoveToColumn(ctx, "p1", "PLAT-1", "  "); err == nil {
		t.Error("a column move with no status must be refused")
	}
}

func TestADraftIsMovedInPlaceAndJournalsNothingNew(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedBoardCards(t, repo)
	key, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "drafted"})
	if err != nil {
		t.Fatalf("draft: %v", err)
	}

	if err := repo.MoveToColumn(ctx, "p1", key, "3"); err != nil {
		t.Fatalf("move: %v", err)
	}
	if err := repo.MoveToSprint(ctx, "p1", key, "12", "Sprint 12"); err != nil {
		t.Fatalf("sprint: %v", err)
	}
	// A rank has neither a column nor a draft field, so it is where a
	// draft's drag stops, and it must not fail the caller.
	if err := repo.RankIssue(ctx, "p1", key, "PLAT-1", true, 1); err != nil {
		t.Fatalf("rank: %v", err)
	}

	rows, _ := repo.PendingForKey(ctx, "p1", key)
	if len(rows) != 1 || rows[0].EntityType != issuerepo.EntityIssueCreate {
		t.Fatalf("pending = %+v, want only the create row", rows)
	}
	var d backend.IssueDraft
	if err := json.Unmarshal([]byte(rows[0].AfterVal), &d); err != nil {
		t.Fatalf("decode draft: %v", err)
	}
	if d.StatusID != "3" || d.SprintID != "12" || d.SprintName != "Sprint 12" {
		t.Errorf("draft JSON = %+v, want where it was dropped", d)
	}
	iss, _ := repo.GetIssue(ctx, "p1", key)
	if iss.StatusID != "3" || iss.SprintID != "12" {
		t.Errorf("draft row = %+v, want its columns moved too", iss)
	}
	// The status text stays Draft: the column a draft sits in is the status
	// id alone, and a real status name would claim a state Jira never gave.
	if iss.Status != issuerepo.StatusDraft || !iss.Draft {
		t.Errorf("draft row = %+v, want it still reading as a draft", iss)
	}
}

func TestDiscardingAMoveRestoresTheColumnsItChanged(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedBoardCards(t, repo)

	// Two moves of the same card: the discard has to land on the column the
	// card started in, not the one the second move came from.
	if err := repo.MoveToColumn(ctx, "p1", "PLAT-1", "3"); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := repo.MoveToColumn(ctx, "p1", "PLAT-1", "5"); err != nil {
		t.Fatalf("second: %v", err)
	}
	p := oneRow(t, repo, "PLAT-1")
	if err := repo.DiscardPendingChange(ctx, "p1", p.ID); err != nil {
		t.Fatalf("discard: %v", err)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.StatusID != "1" || iss.Status != "To Do" || iss.Pending {
		t.Errorf("row = %+v, want the column the card started in", iss)
	}

	// A sprint move reverts both of its columns.
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-1", "", ""); err != nil {
		t.Fatalf("sprint: %v", err)
	}
	p = oneRow(t, repo, "PLAT-1")
	if err := repo.DiscardPendingChange(ctx, "p1", p.ID); err != nil {
		t.Fatalf("discard sprint: %v", err)
	}
	iss, _ = repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.SprintID != "12" || iss.SprintName != "Sprint 12" {
		t.Errorf("row = %+v, want the sprint and its name back", iss)
	}
}

func TestDiscardingARankOnlyDropsTheRow(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedBoardCards(t, repo)

	if err := repo.RankIssue(ctx, "p1", "PLAT-1", "PLAT-2", false, 1); err != nil {
		t.Fatalf("rank: %v", err)
	}
	n, err := repo.DiscardKey(ctx, "p1", "PLAT-1")
	if err != nil || n != 1 {
		t.Fatalf("discard = %d, %v", n, err)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.Rank != "0|a" || iss.Pending {
		t.Errorf("row = %+v, want nothing to put back and no pending flag", iss)
	}
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-1", 0)
	if len(act) != 2 || act[0].Action != "discard" {
		t.Errorf("audit = %+v", act)
	}
}

// diverge writes a column straight onto the row, behind the journal's back,
// which is the one way a cached column stops holding what a pending move
// wrote there: a sync cannot do it, because the replay puts the pending
// target back over every row it refreshes.
func diverge(t *testing.T, db *sql.DB, key, status, statusID string) {
	t.Helper()
	if _, err := db.Exec(
		`UPDATE issue SET status = ?, status_id = ?, updated = ? WHERE profile_id = 'p1' AND key = ?`,
		status, statusID, v2, key); err != nil {
		t.Fatalf("diverge %s: %v", key, err)
	}
}

func TestDiscardingAMoveLeavesAColumnThatHasMovedOnAlone(t *testing.T) {
	repo, db := newRepoDB(t)
	ctx := context.Background()
	seedBoardCards(t, repo)

	if err := repo.MoveToColumn(ctx, "p1", "PLAT-1", "3"); err != nil {
		t.Fatalf("move: %v", err)
	}
	diverge(t, db, "PLAT-1", "Blocked", "10098")

	p := oneRow(t, repo, "PLAT-1")
	if err := repo.DiscardPendingChange(ctx, "p1", p.ID); err != nil {
		t.Fatalf("discard: %v", err)
	}
	rows, _ := repo.PendingForKey(ctx, "p1", "PLAT-1")
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if len(rows) != 0 {
		t.Errorf("pending = %+v, want the row gone whatever the column says", rows)
	}
	// The card is no longer where this app put it, so the before value is
	// not this app's to write back; the next sync settles the column.
	if iss.StatusID != "10098" {
		t.Errorf("row = %+v, want the value that is there now kept", iss)
	}
}

func TestADragHomeLeavesAColumnThatHasMovedOnAlone(t *testing.T) {
	repo, db := newRepoDB(t)
	ctx := context.Background()
	seedBoardCards(t, repo)

	if err := repo.MoveToColumn(ctx, "p1", "PLAT-1", "3"); err != nil {
		t.Fatalf("move: %v", err)
	}
	diverge(t, db, "PLAT-1", "Blocked", "10098")

	// An undo puts a column back exactly as a discard does, so it asks the
	// same question first. One drag home means one thing whichever of the
	// two handles it.
	if err := repo.MoveToColumn(ctx, "p1", "PLAT-1", "1"); err != nil {
		t.Fatalf("home: %v", err)
	}
	rows, _ := repo.PendingForKey(ctx, "p1", "PLAT-1")
	if len(rows) != 0 {
		t.Errorf("pending = %+v, want the journal row gone", rows)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.StatusID != "10098" {
		t.Errorf("row = %+v, want the column that has moved on kept", iss)
	}
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-1", 0)
	if len(act) != 2 || act[0].Action != "undo" {
		t.Errorf("audit = %+v, want the undo recorded", act)
	}
}

func TestDiscardingAMoveAfterASyncThatOnlyFreshenedTheStampPutsTheCardBack(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedBoardCards(t, repo)

	if err := repo.MoveToColumn(ctx, "p1", "PLAT-1", "3"); err != nil {
		t.Fatalf("move: %v", err)
	}
	// Someone commented on the issue in Jira. The sync brings the row back
	// with a newer stamp and the status it always had, and the replay puts
	// the pending target over it again: the column the row holds now is
	// this app's own write, not a value Jira moved the card to.
	fresh := []backend.Issue{{Key: "PLAT-1", ID: "101", Project: "PLAT", Type: backend.TypeStory, Summary: "one",
		Status: "To Do", StatusID: "1", SprintID: "12", SprintName: "Sprint 12", Rank: "0|a", Updated: v2}}
	if err := repo.UpsertPage(ctx, "p1", fresh, time.Now(), false); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1"); iss.StatusID != "3" {
		t.Fatalf("row after the sync = %+v, want the pending move still drawn", iss)
	}
	p := oneRow(t, repo, "PLAT-1")
	if err := repo.DiscardPendingChange(ctx, "p1", p.ID); err != nil {
		t.Fatalf("discard: %v", err)
	}
	// Discard means the same thing on a board row as on a field edit. A
	// card left at its pending target with no journal row behind it is in a
	// state that exists in neither Jira nor the journal.
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.StatusID != "1" || iss.Status != "To Do" || iss.Pending {
		t.Errorf("row = %+v, want the column the card started in", iss)
	}
}

func TestASecondDropOnARenamedSprintIsNotAMove(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedBoardCards(t, repo)

	if err := repo.MoveToSprint(ctx, "p1", "PLAT-1", "13", "Sprint 13"); err != nil {
		t.Fatalf("first: %v", err)
	}
	// The sprint was renamed in Jira between two identical drops. The card
	// has not moved, and every comparison here is on ids.
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-1", "13", "Sprint 13 renamed"); err != nil {
		t.Fatalf("second: %v", err)
	}
	if p := oneRow(t, repo, "PLAT-1"); p.AfterVal != "13|Sprint 13" {
		t.Errorf("after = %q, want the journal row left as it was", p.AfterVal)
	}
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-1", 0)
	if len(act) != 1 {
		t.Errorf("audit = %+v, want one move rather than a second that did not happen", act)
	}
}

func TestAFullSyncKeepsAPendingMoveOnScreen(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedBoardCards(t, repo)

	if err := repo.MoveToColumn(ctx, "p1", "PLAT-1", "3"); err != nil {
		t.Fatalf("move: %v", err)
	}
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-2", "", ""); err != nil {
		t.Fatalf("sprint: %v", err)
	}
	// A full sync deletes and reinserts every row. Without the board types
	// in the replay it would put both cards back where Jira has them while
	// the journal still said they moved, and the Backlog and the board
	// would then disagree about the same issue.
	same := []backend.Issue{
		{Key: "PLAT-1", ID: "101", Project: "PLAT", Type: backend.TypeStory, Summary: "one",
			Status: "To Do", StatusID: "1", SprintID: "12", SprintName: "Sprint 12", Rank: "0|a", Updated: v1},
		{Key: "PLAT-2", ID: "102", Project: "PLAT", Type: backend.TypeStory, Summary: "two",
			Status: "In Progress", StatusID: "3", SprintID: "12", SprintName: "Sprint 12", Rank: "0|b", Updated: v1},
	}
	if err := repo.UpsertPage(ctx, "p1", same, time.Now(), true); err != nil {
		t.Fatalf("full sync: %v", err)
	}
	one, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if one.StatusID != "3" || one.Status != "In Progress" {
		t.Errorf("PLAT-1 = %+v, want the pending move still drawn", one)
	}
	two, _ := repo.GetIssue(ctx, "p1", "PLAT-2")
	if two.SprintID != "" || two.SprintName != "" {
		t.Errorf("PLAT-2 = %+v, want the pending move to the backlog still drawn", two)
	}
	if len(oneRow(t, repo, "PLAT-1").AfterVal) == 0 {
		t.Error("the journal row must survive the sync too")
	}
}

func TestRekeyRepointsARankJournaledAgainstADraftAndKeepsItsBoard(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedBoardCards(t, repo)
	draft, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "drafted"})
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if err := repo.RankIssue(ctx, "p1", "PLAT-1", draft, true, 7); err != nil {
		t.Fatalf("rank: %v", err)
	}
	// The draft gets the key Jira assigned. A rank still naming TAM-NEW-n
	// would push a temporary key, which is the bug Phase 2 fixed for parents.
	if err := repo.Rekey(ctx, "p1", draft, "PLAT-99"); err != nil {
		t.Fatalf("rekey: %v", err)
	}
	p := oneRow(t, repo, "PLAT-1")
	// The board the drop was made on has to survive the repoint: the draft
	// neighbour is exactly the case the commit pass needs it for, and a
	// value rebuilt without it would leave the rank naming a board 0 that
	// does not exist.
	if p.AfterVal != "before|PLAT-99|7" {
		t.Errorf("rank after the rekey = %q, want the real key and the board it was dropped on", p.AfterVal)
	}
}

func TestPendingMovesFoldsTheThreeTypesPerIssue(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedBoardCards(t, repo)

	if err := repo.MoveToColumn(ctx, "p1", "PLAT-1", "3"); err != nil {
		t.Fatalf("column: %v", err)
	}
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-1", "", ""); err != nil {
		t.Fatalf("sprint: %v", err)
	}
	if err := repo.RankIssue(ctx, "p1", "PLAT-2", "PLAT-3", true, 1); err != nil {
		t.Fatalf("rank: %v", err)
	}
	// An ordinary field edit is not a board intent and must not appear here.
	if err := repo.EditField(ctx, "p1", "PLAT-3", "summary", "renamed"); err != nil {
		t.Fatalf("edit: %v", err)
	}

	moves, err := repo.PendingMoves(ctx, "p1")
	if err != nil {
		t.Fatalf("pending moves: %v", err)
	}
	if len(moves) != 2 {
		t.Fatalf("moves = %+v, want one per issue that has a board intent", moves)
	}
	one, two := moves[0], moves[1]
	if one.Key != "PLAT-1" || !one.HasTransition || one.StatusID != "3" {
		t.Errorf("PLAT-1 = %+v, want its transition folded in", one)
	}
	// The backlog is an empty sprint id with the flag set, which is the
	// only thing that tells it apart from a card with no sprint move.
	if !one.HasSprint || one.SprintID != "" || one.HasRank {
		t.Errorf("PLAT-1 = %+v, want a sprint move to the backlog and no rank", one)
	}
	if two.Key != "PLAT-2" || !two.HasRank || two.RankNeighbour != "PLAT-3" || !two.RankBefore {
		t.Errorf("PLAT-2 = %+v, want the neighbour and the side it was dropped on", two)
	}
	if two.HasTransition || two.HasSprint {
		t.Errorf("PLAT-2 = %+v, want only the rank", two)
	}
}
