package boardrepo_test

import (
	"context"
	"database/sql"
	"testing"

	"agile-suite/tam/internal/backend"
)

func oneColumn() []backend.BoardColumn {
	return []backend.BoardColumn{{Name: "To Do", StatusIDs: []string{"1"}}}
}

func twoColumns() []backend.BoardColumn {
	return []backend.BoardColumn{
		{Name: "To Do", StatusIDs: []string{"1"}},
		{Name: "Done", StatusIDs: []string{"5"}},
	}
}

// countKeys reads how many keys one board holds for a sprint scope. It
// takes no *testing.T, so the reader goroutine below can call it.
func countKeys(db *sql.DB, profileID string, boardID int, sprintID string) (int, error) {
	var n int
	err := db.QueryRow(
		`SELECT count(*) FROM board_issue WHERE profile_id = ? AND board_id = ? AND sprint_id = ?`,
		profileID, boardID, sprintID).Scan(&n)
	return n, err
}

// TestReplaceBoardLandsColumnsAndMembershipTogether writes two boards while
// a reader shaped like the Boards view runs beside it: columns, then
// membership, over and over. One column per card is the invariant the seed
// and the replacement both hold, so a reader that ever sees a different
// count has caught a board with its columns replaced and its membership
// still old, which is what the four separate transactions allowed.
func TestReplaceBoardLandsColumnsAndMembershipTogether(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()

	before := map[string][]string{"": {"PLAT-1"}}
	for _, b := range sampleBoards() {
		if err := r.ReplaceBoard(ctx, "p1", b, oneColumn(), nil, before); err != nil {
			t.Fatalf("seed board %d: %v", b.ID, err)
		}
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	torn := make(chan [2]int, 1)
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			cols, err := r.Columns(ctx, "p1", 1)
			if err != nil {
				continue
			}
			keys, err := countKeys(db, "p1", 1, "")
			if err != nil {
				continue
			}
			if len(cols) != keys {
				select {
				case torn <- [2]int{len(cols), keys}:
				default:
				}
				return
			}
		}
	}()

	after := map[string][]string{"": {"PLAT-1", "PLAT-2"}}
	for _, b := range sampleBoards() {
		if err := r.ReplaceBoard(ctx, "p1", b, twoColumns(), nil, after); err != nil {
			t.Fatalf("replace board %d: %v", b.ID, err)
		}
	}
	close(stop)
	<-done
	select {
	case got := <-torn:
		t.Fatalf("a read saw %d columns against %d cards, want the two to change together", got[0], got[1])
	default:
	}

	for _, id := range []int{1, 2} {
		cols, err := r.Columns(ctx, "p1", id)
		if err != nil || len(cols) != 2 {
			t.Fatalf("board %d columns = %+v, %v, want both new columns", id, cols, err)
		}
		if got := boardKeys(t, db, "p1", id, ""); len(got) != 2 || got[1] != "PLAT-2" {
			t.Errorf("board %d membership = %v, want the new pair", id, got)
		}
	}
}

// TestReplaceBoardForgetsTheMembershipOfASprintItNoLongerCarries covers the
// scope the four Upsert calls could never reach: a sprint that stopped
// being active, or was deleted in Jira, is no longer among the scopes the
// sync writes, so only clearing the whole board first takes its cards away.
func TestReplaceBoardForgetsTheMembershipOfASprintItNoLongerCarries(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}

	sprints := []backend.Sprint{
		{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active"},
		{ID: 13, BoardID: 1, Name: "Sprint 13", State: "future"},
	}
	keys := map[string][]string{
		"":   {"PLAT-1", "PLAT-2"},
		"12": {"PLAT-1"},
		"13": {"PLAT-2"},
	}
	if err := r.ReplaceBoard(ctx, "p1", board, oneColumn(), sprints, keys); err != nil {
		t.Fatalf("first replace: %v", err)
	}

	// Sprint 12 closed and sprint 13 was deleted: this run reads neither.
	next := []backend.Sprint{{ID: 12, BoardID: 1, Name: "Sprint 12", State: "closed"}}
	if err := r.ReplaceBoard(ctx, "p1", board, oneColumn(), next, map[string][]string{"": {"PLAT-1"}}); err != nil {
		t.Fatalf("second replace: %v", err)
	}

	if got := boardKeys(t, db, "p1", 1, "13"); len(got) != 0 {
		t.Errorf("sprint 13 membership = %v, want it gone with the sprint", got)
	}
	if got := boardKeys(t, db, "p1", 1, "12"); len(got) != 0 {
		t.Errorf("sprint 12 membership = %v, want it gone now that the sprint is closed", got)
	}
	if got := boardKeys(t, db, "p1", 1, ""); len(got) != 1 || got[0] != "PLAT-1" {
		t.Errorf("board membership = %v, want this run's list", got)
	}
	sp, err := r.ListSprints(ctx, "p1", 1)
	if err != nil || len(sp) != 1 || sp[0].ID != 12 {
		t.Fatalf("sprints = %+v, %v, want only sprint 12 left", sp, err)
	}
}
