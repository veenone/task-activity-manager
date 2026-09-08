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

// seedSprintRow writes one row of the boards sync's sprint table, which is
// where the name of a sprint no issue is in yet has to come from.
func seedSprintRow(t *testing.T, db *sql.DB, id int, name string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO sprint (profile_id, id, board_id, name, state, start_date, end_date) VALUES (?, ?, 1, ?, 'future', '', '')`,
		"p1", id, name); err != nil {
		t.Fatalf("seed sprint %d: %v", id, err)
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
	if iss.StatusID != "10099" || iss.Status != "10099" {
		t.Errorf("row = %+v, want the id shown in place of a name nobody knows", iss)
	}
}

func TestMoveToSprintJournalsTheSprintAndTheBacklog(t *testing.T) {
	repo, db := newRepoDB(t)
	ctx := context.Background()
	seedBoardCards(t, repo)
	// Sprint 13 is a future sprint no issue is in yet, so its name can only
	// come from the boards sync's own table.
	seedSprintRow(t, db, 13, "Sprint 13")

	if err := repo.MoveToSprint(ctx, "p1", "PLAT-1", "13"); err != nil {
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
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-1", ""); err != nil {
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
}

func TestRankIssueJournalsItsNeighbourAndWritesNoColumn(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedBoardCards(t, repo)

	if err := repo.RankIssue(ctx, "p1", "PLAT-1", "PLAT-2", true); err != nil {
		t.Fatalf("rank: %v", err)
	}
	p := oneRow(t, repo, "PLAT-1")
	if p.EntityType != issuerepo.EntityRank || p.Field != issuerepo.FieldRank {
		t.Errorf("journal row = %+v, want a rank", p)
	}
	if p.BeforeVal != "" || p.AfterVal != "before|PLAT-2" || p.BaseVersion != v1 {
		t.Errorf("journal row = %+v, want the neighbour and its side and no before value", p)
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
	if err := repo.RankIssue(ctx, "p1", "PLAT-1", "PLAT-3", false); err != nil {
		t.Fatalf("re-rank: %v", err)
	}
	p = oneRow(t, repo, "PLAT-1")
	if p.AfterVal != "after|PLAT-3" {
		t.Errorf("after = %q, want the newest drop", p.AfterVal)
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
	repo, db := newRepoDB(t)
	ctx := context.Background()
	seedBoardCards(t, repo)
	seedSprintRow(t, db, 13, "Sprint 13")

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

	// The same rule on a sprint move.
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-1", "13"); err != nil {
		t.Fatalf("sprint out: %v", err)
	}
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-1", "12"); err != nil {
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
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-404", "13"); !errors.Is(err, issuerepo.ErrNotFound) {
		t.Errorf("sprint move of an unknown key = %v, want ErrNotFound", err)
	}
	// A rank against a card nobody has seen would be pushed to Jira as a key
	// that does not resolve.
	err := repo.RankIssue(ctx, "p1", "PLAT-1", "PLAT-404", true)
	if err == nil || !strings.Contains(err.Error(), "PLAT-404") {
		t.Errorf("rank against an unknown neighbour = %v, want it refused by name", err)
	}
	if err := repo.RankIssue(ctx, "p1", "PLAT-1", "PLAT-1", true); err == nil {
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
	if err := repo.MoveToSprint(ctx, "p1", key, "12"); err != nil {
		t.Fatalf("sprint: %v", err)
	}
	// A rank has neither a column nor a draft field, so it is where a
	// draft's drag stops, and it must not fail the caller.
	if err := repo.RankIssue(ctx, "p1", key, "PLAT-1", true); err != nil {
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
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-1", ""); err != nil {
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

	if err := repo.RankIssue(ctx, "p1", "PLAT-1", "PLAT-2", false); err != nil {
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

func TestDiscardingAMoveLeavesTheColumnAloneWhenJiraHasMovedOn(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedBoardCards(t, repo)

	if err := repo.MoveToColumn(ctx, "p1", "PLAT-1", "3"); err != nil {
		t.Fatalf("move: %v", err)
	}
	// A sync brings the row back with a newer version: whatever the card's
	// column is now, it is not the one the move was made against.
	fresh := []backend.Issue{{Key: "PLAT-1", ID: "101", Project: "PLAT", Type: backend.TypeStory, Summary: "one",
		Status: "Blocked", StatusID: "10098", SprintID: "12", SprintName: "Sprint 12", Rank: "0|a", Updated: v2}}
	if err := repo.UpsertPage(ctx, "p1", fresh, time.Now(), false); err != nil {
		t.Fatalf("sync: %v", err)
	}
	p := oneRow(t, repo, "PLAT-1")
	if err := repo.DiscardPendingChange(ctx, "p1", p.ID); err != nil {
		t.Fatalf("discard: %v", err)
	}
	rows, _ := repo.PendingForKey(ctx, "p1", "PLAT-1")
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if len(rows) != 0 {
		t.Errorf("pending = %+v, want the row gone whatever the base version says", rows)
	}
	// Writing a stale before_val over a fresher remote is worse than leaving
	// the value in place; the next sync is what settles it.
	if iss.StatusID == "1" {
		t.Errorf("row = %+v, want the stale before value not written back", iss)
	}
}

func TestAFullSyncKeepsAPendingMoveOnScreen(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedBoardCards(t, repo)

	if err := repo.MoveToColumn(ctx, "p1", "PLAT-1", "3"); err != nil {
		t.Fatalf("move: %v", err)
	}
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-2", ""); err != nil {
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

func TestRekeyRepointsARankJournaledAgainstADraft(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedBoardCards(t, repo)
	draft, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "drafted"})
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if err := repo.RankIssue(ctx, "p1", "PLAT-1", draft, true); err != nil {
		t.Fatalf("rank: %v", err)
	}
	// The draft gets the key Jira assigned. A rank still naming TAM-NEW-n
	// would push a temporary key, which is the bug Phase 2 fixed for parents.
	if err := repo.Rekey(ctx, "p1", draft, "PLAT-99"); err != nil {
		t.Fatalf("rekey: %v", err)
	}
	p := oneRow(t, repo, "PLAT-1")
	if p.AfterVal != "before|PLAT-99" {
		t.Errorf("rank after the rekey = %q, want the real key", p.AfterVal)
	}
}

func TestPendingMovesFoldsTheThreeTypesPerIssue(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedBoardCards(t, repo)

	if err := repo.MoveToColumn(ctx, "p1", "PLAT-1", "3"); err != nil {
		t.Fatalf("column: %v", err)
	}
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-1", ""); err != nil {
		t.Fatalf("sprint: %v", err)
	}
	if err := repo.RankIssue(ctx, "p1", "PLAT-2", "PLAT-3", true); err != nil {
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
