package boardrepo_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/tamstore"
)

var syncedAt = time.Date(2026, 9, 5, 10, 42, 0, 0, time.UTC)

// newRepo opens a fresh tam.db and returns the repository with the handle
// beside it, so a test can read board_issue directly: the membership table
// has no reader of its own, the view is what consumes it.
func newRepo(t *testing.T) (*boardrepo.Repository, *sql.DB) {
	t.Helper()
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return boardrepo.New(db.DB()), db.DB()
}

func sampleBoards() []backend.Board {
	return []backend.Board{
		{ID: 2, Name: "PLAT Kanban", Type: backend.BoardTypeKanban},
		{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum},
	}
}

func sampleColumns() []backend.BoardColumn {
	return []backend.BoardColumn{
		{Name: "Backlog", StatusIDs: []string{}},
		{Name: "To Do", StatusIDs: []string{"1"}},
		{Name: "In Progress", StatusIDs: []string{"3", "4"}},
		{Name: "Review", StatusIDs: []string{"10001"}},
		{Name: "Done", StatusIDs: []string{"5", "6"}},
	}
}

func sampleSprints() []backend.Sprint {
	return []backend.Sprint{
		{ID: 11, BoardID: 1, Name: "Sprint 11", State: "closed", StartDate: "2026-08-04T09:00:00Z", EndDate: "2026-08-18T09:00:00Z"},
		{ID: 13, BoardID: 1, Name: "Sprint 13", State: "future", StartDate: "2026-09-01T09:00:00Z", EndDate: "2026-09-15T09:00:00Z"},
		{ID: 14, BoardID: 1, Name: "Sprint 14", State: "future", StartDate: "2026-09-15T09:00:00Z", EndDate: "2026-09-29T09:00:00Z"},
		{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active", StartDate: "2026-08-18T09:00:00Z", EndDate: "2026-09-01T09:00:00Z"},
	}
}

func TestUpsertAndReadBackTheBoardAndItsShape(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	if err := r.UpsertBoards(ctx, "p1", sampleBoards(), syncedAt); err != nil {
		t.Fatalf("boards: %v", err)
	}
	if err := r.UpsertColumns(ctx, "p1", 1, sampleColumns()); err != nil {
		t.Fatalf("columns: %v", err)
	}
	if err := r.UpsertSprints(ctx, "p1", 1, sampleSprints()); err != nil {
		t.Fatalf("sprints: %v", err)
	}

	boards, err := r.ListBoards(ctx, "p1")
	if err != nil {
		t.Fatalf("list boards: %v", err)
	}
	if len(boards) != 2 || boards[0].Name != "PLAT Kanban" || boards[1].Name != "PLAT Scrum" {
		t.Fatalf("boards = %+v, want them by name", boards)
	}
	if boards[1].ID != 1 || boards[1].Type != backend.BoardTypeScrum {
		t.Errorf("scrum board = %+v", boards[1])
	}

	cols, err := r.Columns(ctx, "p1", 1)
	if err != nil {
		t.Fatalf("columns: %v", err)
	}
	want := sampleColumns()
	if len(cols) != len(want) {
		t.Fatalf("columns = %+v", cols)
	}
	for i, w := range want {
		if cols[i].Name != w.Name {
			t.Errorf("column %d = %q, want %q in board order", i, cols[i].Name, w.Name)
		}
		if len(cols[i].StatusIDs) != len(w.StatusIDs) {
			t.Errorf("column %q status ids = %v, want %v", cols[i].Name, cols[i].StatusIDs, w.StatusIDs)
		}
	}
	if cols[0].StatusIDs == nil {
		t.Error("a column with no statuses reads back as an empty list, not nil")
	}
	if len(cols[2].StatusIDs) != 2 || cols[2].StatusIDs[1] != "4" {
		t.Errorf("In Progress status ids = %v", cols[2].StatusIDs)
	}

	sprints, err := r.ListSprints(ctx, "p1", 1)
	if err != nil {
		t.Fatalf("sprints: %v", err)
	}
	gotOrder := []int{}
	for _, s := range sprints {
		gotOrder = append(gotOrder, s.ID)
		if s.BoardID != 1 {
			t.Errorf("sprint %d is on board %d", s.ID, s.BoardID)
		}
	}
	// Active first, then the future ones by start date, then closed.
	wantOrder := []int{12, 13, 14, 11}
	for i, id := range wantOrder {
		if gotOrder[i] != id {
			t.Fatalf("sprint order = %v, want %v", gotOrder, wantOrder)
		}
	}
	if sprints[0].Name != "Sprint 12" || sprints[0].EndDate != "2026-09-01T09:00:00Z" {
		t.Errorf("active sprint = %+v", sprints[0])
	}
}

func TestUpsertBoardsUpdatesInPlace(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	if err := r.UpsertBoards(ctx, "p1", sampleBoards(), syncedAt); err != nil {
		t.Fatalf("boards: %v", err)
	}
	renamed := []backend.Board{{ID: 1, Name: "Platform Scrum", Type: backend.BoardTypeScrum}}
	if err := r.UpsertBoards(ctx, "p1", renamed, syncedAt.Add(time.Hour)); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	boards, err := r.ListBoards(ctx, "p1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(boards) != 2 {
		t.Fatalf("boards = %+v, want the rename not to drop the other board", boards)
	}
	if boards[0].Name != "PLAT Kanban" || boards[1].Name != "Platform Scrum" {
		t.Errorf("boards = %+v", boards)
	}
}

func TestUpsertColumnsReplacesTheBoardsColumns(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	if err := r.UpsertColumns(ctx, "p1", 1, sampleColumns()); err != nil {
		t.Fatalf("columns: %v", err)
	}
	// The board went from five columns to three; the two Jira dropped must
	// not survive, and an upsert alone would have left them.
	shorter := sampleColumns()[:3]
	if err := r.UpsertColumns(ctx, "p1", 1, shorter); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	cols, err := r.Columns(ctx, "p1", 1)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(cols) != 3 {
		t.Fatalf("columns = %+v, want the three that are left", cols)
	}
	if cols[2].Name != "In Progress" {
		t.Errorf("last column = %q", cols[2].Name)
	}
}

func TestUpsertIssueKeysReplacesTheSprintsMembership(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	if err := r.UpsertIssueKeys(ctx, "p1", 1, "12", []string{"PLAT-412", "PLAT-409", "PLAT-401"}); err != nil {
		t.Fatalf("keys: %v", err)
	}
	if err := r.UpsertIssueKeys(ctx, "p1", 1, "", []string{"PLAT-412", "PLAT-347"}); err != nil {
		t.Fatalf("board keys: %v", err)
	}
	// PLAT-409 was moved out of sprint 12; the sprint must forget it.
	if err := r.UpsertIssueKeys(ctx, "p1", 1, "12", []string{"PLAT-412", "PLAT-401"}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	got := boardKeys(t, db, "p1", 1, "12")
	if len(got) != 2 || got[0] != "PLAT-412" || got[1] != "PLAT-401" {
		t.Errorf("sprint 12 = %v, want the card moved out to be gone", got)
	}
	// The board's own list is a different scope and is untouched.
	if got := boardKeys(t, db, "p1", 1, ""); len(got) != 2 || got[1] != "PLAT-347" {
		t.Errorf("whole board = %v", got)
	}
}

func TestUpsertSprintsReplacesTheBoardsSprints(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	if err := r.UpsertSprints(ctx, "p1", 1, sampleSprints()); err != nil {
		t.Fatalf("sprints: %v", err)
	}
	// Sprint 14 was deleted in Jira; it must not stay in the picker.
	left := []backend.Sprint{}
	for _, s := range sampleSprints() {
		if s.ID != 14 {
			left = append(left, s)
		}
	}
	if err := r.UpsertSprints(ctx, "p1", 1, left); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	sprints, err := r.ListSprints(ctx, "p1", 1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(sprints) != 3 {
		t.Fatalf("sprints = %+v, want the deleted one gone", sprints)
	}
	for _, s := range sprints {
		if s.ID == 14 {
			t.Error("sprint 14 was deleted in Jira and is still cached")
		}
	}
}

func TestRemoveBoardsTakesTheChildrenWithIt(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	seedTwoBoards(t, r, "p1")

	if err := r.RemoveBoards(ctx, "p1", []int{1}); err != nil {
		t.Fatalf("remove: %v", err)
	}
	boards, err := r.ListBoards(ctx, "p1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(boards) != 1 || boards[0].ID != 2 {
		t.Fatalf("boards = %+v, want only board 2", boards)
	}
	if cols, _ := r.Columns(ctx, "p1", 1); len(cols) != 0 {
		t.Errorf("board 1 still has columns: %+v", cols)
	}
	if sprints, _ := r.ListSprints(ctx, "p1", 1); len(sprints) != 0 {
		t.Errorf("board 1 still has sprints: %+v", sprints)
	}
	if keys := boardKeys(t, db, "p1", 1, ""); len(keys) != 0 {
		t.Errorf("board 1 still has issue keys: %v", keys)
	}
	// Board 2 kept everything of its own.
	if cols, _ := r.Columns(ctx, "p1", 2); len(cols) != len(sampleColumns()) {
		t.Errorf("board 2 columns = %+v", cols)
	}
	if keys := boardKeys(t, db, "p1", 2, ""); len(keys) != 1 {
		t.Errorf("board 2 keys = %v", keys)
	}
	// Removing nothing is not an error and changes nothing.
	if err := r.RemoveBoards(ctx, "p1", nil); err != nil {
		t.Errorf("remove none: %v", err)
	}
}

func TestBoardTablesAreScopedToTheProfile(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	seedTwoBoards(t, r, "p1")
	seedTwoBoards(t, r, "p2")

	if err := r.RemoveBoards(ctx, "p1", []int{1, 2}); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if boards, _ := r.ListBoards(ctx, "p2"); len(boards) != 2 {
		t.Errorf("p2 boards = %+v, want p1's removal to leave them alone", boards)
	}
	if cols, _ := r.Columns(ctx, "p2", 1); len(cols) != len(sampleColumns()) {
		t.Errorf("p2 columns = %+v", cols)
	}
	if sprints, _ := r.ListSprints(ctx, "p2", 1); len(sprints) != len(sampleSprints()) {
		t.Errorf("p2 sprints = %+v", sprints)
	}
	if keys := boardKeys(t, db, "p2", 1, ""); len(keys) == 0 {
		t.Error("p2 lost its board keys")
	}
}

func TestPurgeProfileClearsTheFourBoardTables(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	seedTwoBoards(t, r, "p1")
	seedTwoBoards(t, r, "p2")

	if err := r.PurgeProfile(ctx, "p1"); err != nil {
		t.Fatalf("purge: %v", err)
	}
	if boards, _ := r.ListBoards(ctx, "p1"); len(boards) != 0 {
		t.Errorf("boards after purge = %+v", boards)
	}
	if cols, _ := r.Columns(ctx, "p1", 1); len(cols) != 0 {
		t.Errorf("columns after purge = %+v", cols)
	}
	if sprints, _ := r.ListSprints(ctx, "p1", 1); len(sprints) != 0 {
		t.Errorf("sprints after purge = %+v", sprints)
	}
	if keys := boardKeys(t, db, "p1", 1, ""); len(keys) != 0 {
		t.Errorf("issue keys after purge = %v", keys)
	}
	if boards, _ := r.ListBoards(ctx, "p2"); len(boards) != 2 {
		t.Errorf("p2 boards = %+v, want the other profile untouched", boards)
	}
	if keys := boardKeys(t, db, "p2", 1, ""); len(keys) == 0 {
		t.Error("p2 lost its board keys to p1's purge")
	}
}

// seedTwoBoards writes both sample boards for a profile, with columns,
// sprints, and one issue key each.
func seedTwoBoards(t *testing.T, r *boardrepo.Repository, profileID string) {
	t.Helper()
	ctx := context.Background()
	if err := r.UpsertBoards(ctx, profileID, sampleBoards(), syncedAt); err != nil {
		t.Fatalf("boards: %v", err)
	}
	for _, id := range []int{1, 2} {
		if err := r.UpsertColumns(ctx, profileID, id, sampleColumns()); err != nil {
			t.Fatalf("columns of %d: %v", id, err)
		}
		if err := r.UpsertIssueKeys(ctx, profileID, id, "", []string{"PLAT-412"}); err != nil {
			t.Fatalf("keys of %d: %v", id, err)
		}
	}
	if err := r.UpsertSprints(ctx, profileID, 1, sampleSprints()); err != nil {
		t.Fatalf("sprints: %v", err)
	}
}

// boardKeys reads the cached membership straight out of board_issue, in the
// order the sync stored it.
func boardKeys(t *testing.T, db *sql.DB, profileID string, boardID int, sprintID string) []string {
	t.Helper()
	rows, err := db.Query(
		`SELECT key FROM board_issue WHERE profile_id = ? AND board_id = ? AND sprint_id = ? ORDER BY position`,
		profileID, boardID, sprintID)
	if err != nil {
		t.Fatalf("board keys: %v", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, key)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return out
}
