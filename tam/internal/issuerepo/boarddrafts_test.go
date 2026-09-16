package issuerepo_test

import (
	"context"
	"encoding/json"
	"sort"
	"testing"
	"time"

	"agile-suite/tam/internal/issuerepo"
)

func board14() issuerepo.DraftBoard {
	return issuerepo.DraftBoard{
		Name: "PLAT Scrum 2", Type: "scrum", FilterName: "PLAT board filter", JQL: "project = PLAT",
	}
}

func TestADraftBoardTakesANegativeIdAndJournalsExactlyOneCreateRow(t *testing.T) {
	repo, db := newRepoWithDB(t)
	ctx := context.Background()

	made, err := repo.CreateDraftBoard(ctx, "p1", board14())
	if err != nil {
		t.Fatal(err)
	}
	if made.ID != -1 || made.Name != "PLAT Scrum 2" || made.Type != "scrum" {
		t.Fatalf("made = %+v", made)
	}

	var name, boardType string
	var draft int
	if err := db.QueryRow(`SELECT name, type, draft FROM board WHERE profile_id = 'p1' AND id = -1`).
		Scan(&name, &boardType, &draft); err != nil || draft != 1 || name != "PLAT Scrum 2" || boardType != "scrum" {
		t.Errorf("board row = %q %q draft %d, %v", name, boardType, draft, err)
	}

	rows, err := repo.ListPendingChanges(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, p := range rows {
		if p.EntityType == issuerepo.EntityBoardCreate {
			keys = append(keys, p.EntityKey)
		}
	}
	if len(keys) != 1 || keys[0] != "-1" {
		t.Fatalf("board_create keys = %v, want exactly one, -1", keys)
	}

	p, ok := rowOf(t, repo, "-1", issuerepo.EntityBoardCreate)
	if !ok {
		t.Fatal("no board_create row")
	}
	var carried issuerepo.DraftBoard
	if err := json.Unmarshal([]byte(p.AfterVal), &carried); err != nil || carried != board14() {
		t.Errorf("create row carries the draft: %+v, %v", carried, err)
	}
}

func TestTwoDraftBoardsInARowGetDifferentNegativeIds(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()

	first, err := repo.CreateDraftBoard(ctx, "p1", board14())
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.CreateDraftBoard(ctx, "p1", issuerepo.DraftBoard{Name: "PLAT Kanban", Type: "kanban"})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != -1 || second.ID != -2 {
		t.Fatalf("first = %+v, second = %+v", first, second)
	}

	rows, err := repo.ListPendingChanges(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, p := range rows {
		if p.EntityType == issuerepo.EntityBoardCreate {
			keys = append(keys, p.EntityKey)
		}
	}
	sort.Strings(keys)
	if len(keys) != 2 || keys[0] != "-1" || keys[1] != "-2" {
		t.Errorf("board_create keys = %v", keys)
	}
}

func TestCreateDraftBoardRefusesANamelessBoard(t *testing.T) {
	repo := newRepo(t)
	if _, err := repo.CreateDraftBoard(context.Background(), "p1", issuerepo.DraftBoard{Type: "scrum"}); err == nil {
		t.Error("a board with no name is refused")
	}
}

// TestRekeyBoardRepointsEveryTableItOwns seeds one row in every table
// RekeyBoard has to repoint -- a column, a board_issue row, and a sprint,
// all under the draft's negative id -- plus the same three rows on another,
// unrelated board, so a rekey that repointed everything would still fail
// this one-sided check.
func TestRekeyBoardRepointsEveryTableItOwns(t *testing.T) {
	repo, db := newRepoWithDB(t)
	ctx := context.Background()

	made, err := repo.CreateDraftBoard(ctx, "p1", board14())
	if err != nil {
		t.Fatal(err)
	}
	draftID := made.ID

	// One row per table RekeyBoard has to repoint, under the draft id.
	if _, err := db.Exec(`INSERT INTO board_column (profile_id, board_id, position, name, status_ids) VALUES ('p1', ?, 0, 'To Do', '["1"]')`, draftID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO board_issue (profile_id, board_id, sprint_id, key, position) VALUES ('p1', ?, '', 'PLAT-1', 0)`, draftID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sprint (profile_id, id, board_id, name, state) VALUES ('p1', 50, ?, 'Sprint 50', 'future')`, draftID); err != nil {
		t.Fatal(err)
	}

	// The same three rows on another, unrelated board: RekeyBoard must
	// leave every one of these alone.
	if _, err := db.Exec(`INSERT INTO board (profile_id, id, name, type) VALUES ('p1', 9, 'Other board', 'scrum')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO board_column (profile_id, board_id, position, name, status_ids) VALUES ('p1', 9, 0, 'Doing', '["2"]')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO board_issue (profile_id, board_id, sprint_id, key, position) VALUES ('p1', 9, '', 'PLAT-2', 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sprint (profile_id, id, board_id, name, state) VALUES ('p1', 51, 9, 'Sprint 51', 'future')`); err != nil {
		t.Fatal(err)
	}

	if err := repo.RekeyBoard(ctx, "p1", draftID, 100); err != nil {
		t.Fatal(err)
	}

	var draft int
	var name string
	if err := db.QueryRow(`SELECT name, draft FROM board WHERE profile_id = 'p1' AND id = 100`).Scan(&name, &draft); err != nil {
		t.Fatalf("read rekeyed board: %v", err)
	}
	if name != "PLAT Scrum 2" || draft != 0 {
		t.Errorf("board = %q draft %d, want real and named", name, draft)
	}
	var goneBoard int
	if err := db.QueryRow(`SELECT COUNT(*) FROM board WHERE profile_id = 'p1' AND id = ?`, draftID).Scan(&goneBoard); err != nil || goneBoard != 0 {
		t.Errorf("no board row keeps the draft id: %d, %v", goneBoard, err)
	}

	var columnBoard int
	if err := db.QueryRow(`SELECT board_id FROM board_column WHERE profile_id = 'p1' AND name = 'To Do'`).Scan(&columnBoard); err != nil || columnBoard != 100 {
		t.Errorf("board_column repointed: %d, %v", columnBoard, err)
	}
	var issueBoard int
	if err := db.QueryRow(`SELECT board_id FROM board_issue WHERE profile_id = 'p1' AND key = 'PLAT-1'`).Scan(&issueBoard); err != nil || issueBoard != 100 {
		t.Errorf("board_issue repointed: %d, %v", issueBoard, err)
	}
	var sprintBoard int
	if err := db.QueryRow(`SELECT board_id FROM sprint WHERE profile_id = 'p1' AND id = 50`).Scan(&sprintBoard); err != nil || sprintBoard != 100 {
		t.Errorf("sprint repointed: %d, %v", sprintBoard, err)
	}

	for _, q := range []string{
		`SELECT COUNT(*) FROM board_column WHERE profile_id = 'p1' AND board_id = ?`,
		`SELECT COUNT(*) FROM board_issue WHERE profile_id = 'p1' AND board_id = ?`,
		`SELECT COUNT(*) FROM sprint WHERE profile_id = 'p1' AND board_id = ?`,
	} {
		var n int
		if err := db.QueryRow(q, draftID).Scan(&n); err != nil || n != 0 {
			t.Errorf("nothing left naming the draft board (%s): %d, %v", q, n, err)
		}
	}

	if _, ok := rowOf(t, repo, "-1", issuerepo.EntityBoardCreate); ok {
		t.Error("the board_create row is gone")
	}

	// Board 9's own rows are untouched.
	var otherColumnBoard, otherIssueBoard, otherSprintBoard int
	if err := db.QueryRow(`SELECT board_id FROM board_column WHERE profile_id = 'p1' AND name = 'Doing'`).Scan(&otherColumnBoard); err != nil || otherColumnBoard != 9 {
		t.Errorf("another board's column is left alone: %d, %v", otherColumnBoard, err)
	}
	if err := db.QueryRow(`SELECT board_id FROM board_issue WHERE profile_id = 'p1' AND key = 'PLAT-2'`).Scan(&otherIssueBoard); err != nil || otherIssueBoard != 9 {
		t.Errorf("another board's issue is left alone: %d, %v", otherIssueBoard, err)
	}
	if err := db.QueryRow(`SELECT board_id FROM sprint WHERE profile_id = 'p1' AND id = 51`).Scan(&otherSprintBoard); err != nil || otherSprintBoard != 9 {
		t.Errorf("another board's sprint is left alone: %d, %v", otherSprintBoard, err)
	}
}

// TestRekeyBoardRepointsARankJournaledAgainstIt covers the journal row
// RekeyBoard has to repoint that is not a table: a rank's after_val packs
// the board it was dropped on (RankValue), and a rank dropped on another
// board must be left alone the same way another board's cached rows are.
func TestRekeyBoardRepointsARankJournaledAgainstIt(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	if err := repo.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatal(err)
	}
	made, err := repo.CreateDraftBoard(ctx, "p1", board14())
	if err != nil {
		t.Fatal(err)
	}
	draftID := made.ID

	if err := repo.RankIssue(ctx, "p1", "PLAT-409", "PLAT-412", true, draftID); err != nil {
		t.Fatal(err)
	}
	if err := repo.RankIssue(ctx, "p1", "PLAT-412", "PLAT-409", false, 9); err != nil {
		t.Fatal(err)
	}

	if err := repo.RekeyBoard(ctx, "p1", draftID, 100); err != nil {
		t.Fatal(err)
	}

	rank409, ok := rowOf(t, repo, "PLAT-409", issuerepo.EntityRank)
	if !ok {
		t.Fatal("no rank row for PLAT-409")
	}
	if _, _, board := issuerepo.ParseRank(rank409.AfterVal); board != 100 {
		t.Errorf("the rank dropped on the draft board now names the real one: %d", board)
	}
	rank412, ok := rowOf(t, repo, "PLAT-412", issuerepo.EntityRank)
	if !ok {
		t.Fatal("no rank row for PLAT-412")
	}
	if _, _, board := issuerepo.ParseRank(rank412.AfterVal); board != 9 {
		t.Errorf("a rank dropped on another board is left alone: %d", board)
	}
}
