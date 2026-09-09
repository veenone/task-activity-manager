package boardrepo_test

import (
	"context"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
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

// cardsIn is how many cards the view drew, over every column. The reader
// below compares it against the number of columns, which is the invariant
// the two board shapes hold.
func cardsIn(view boardrepo.BoardView) int {
	n := 0
	for _, c := range view.Columns {
		n += c.Total
	}
	return n
}

// TestReplaceBoardLandsColumnsAndMembershipTogether writes two boards while
// a reader shaped like the Boards view runs beside it: one Board read, over
// and over. One column per card is the invariant the seed and the
// replacement both hold, so a reader that ever sees a different count has
// caught a board with its columns replaced and its membership still old,
// which is what the four separate transactions allowed.
//
// The reader makes one call and not two, which is the point rather than a
// convenience. Two calls are two snapshots however carefully each of them
// reads, so nothing a store can do would make a pair of them atomic; what
// can be promised is that one read is one board, and Repository.Board is
// the read the view actually makes.
func TestReplaceBoardLandsColumnsAndMembershipTogether(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	// Both cards sit in the column that exists in either shape, so the
	// number of cards drawn is the size of the membership and nothing else.
	issues := newIssues(card("PLAT-1", "To Do", "1"), card("PLAT-2", "To Do", "1"))

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
			view, err := r.Board(ctx, issues, "p1", 1, "", boardrepo.SwimlaneNone)
			if err != nil {
				continue
			}
			if cards := cardsIn(view); len(view.Columns) != cards {
				select {
				case torn <- [2]int{len(view.Columns), cards}:
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

// TestReplaceBoardKeepsEachBoardsCopyOfASharedSprint covers the ordinary
// configuration that used to take the whole boards pass down: two scrum
// boards over one project filter, so Jira hands both of them sprint 12.
// Sprints are cleared and written a board at a time, so the second board's
// insert collided with the first while the key was (profile_id, id).
func TestReplaceBoardKeepsEachBoardsCopyOfASharedSprint(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	shared := backend.Sprint{ID: 12, Name: "Sprint 12", State: "active", StartDate: "2026-08-18T09:00:00Z"}

	for _, b := range sampleBoards() {
		sprints := []backend.Sprint{{ID: shared.ID, BoardID: b.ID, Name: shared.Name, State: shared.State, StartDate: shared.StartDate}}
		if err := r.ReplaceBoard(ctx, "p1", b, oneColumn(), sprints, nil); err != nil {
			t.Fatalf("replace board %d: %v", b.ID, err)
		}
	}

	for _, b := range sampleBoards() {
		got, err := r.ListSprints(ctx, "p1", b.ID)
		if err != nil {
			t.Fatalf("board %d sprints: %v", b.ID, err)
		}
		if len(got) != 1 || got[0].ID != 12 || got[0].BoardID != b.ID {
			t.Errorf("board %d sprints = %+v, want its own copy of sprint 12", b.ID, got)
		}
	}
}

// TestReplaceBoardWritesADuplicatedKeyOnce covers the other duplicate the
// key made fatal: a startAt walk over a board whose ranks move mid-walk
// hands the same issue back on two pages, and one collision used to abort
// the board's whole write.
func TestReplaceBoardWritesADuplicatedKeyOnce(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	keys := map[string][]string{"": {"PLAT-1", "PLAT-2", "PLAT-1", "PLAT-3"}}

	if err := r.ReplaceBoard(ctx, "p1", board, oneColumn(), nil, keys); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got := boardKeys(t, db, "p1", 1, "")
	want := []string{"PLAT-1", "PLAT-2", "PLAT-3"}
	if len(got) != len(want) {
		t.Fatalf("membership = %v, want the repeat written once", got)
	}
	for i, key := range want {
		if got[i] != key {
			t.Errorf("membership = %v, want %v in the order the pages arrived", got, want)
		}
	}
}

// TestReplaceSprintsLeavesTheRestOfTheBoardAlone is why the sprint list has
// a writer of its own. writeSprints is private and its other caller,
// ReplaceBoard, deletes and rewrites the board's columns and every one of
// its membership rows on the way past: reaching for that to record a sprint
// that has just started would wipe the board the user is looking at.
func TestReplaceSprintsLeavesTheRestOfTheBoardAlone(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	keys := map[string][]string{"": {"PLAT-1", "PLAT-2"}, "13": {"PLAT-2"}}
	sprints := []backend.Sprint{{ID: 13, BoardID: 1, Name: "Sprint 13", State: "future"}}
	if err := r.ReplaceBoard(ctx, "p1", board, twoColumns(), sprints, keys); err != nil {
		t.Fatalf("seed: %v", err)
	}

	started := []backend.Sprint{{ID: 13, BoardID: 1, Name: "Sprint 13", State: "active", StartDate: "2026-09-09T09:00:00.000+0000"}}
	if err := r.ReplaceSprints(ctx, "p1", 1, started); err != nil {
		t.Fatalf("replace sprints: %v", err)
	}

	got, err := r.ListSprints(ctx, "p1", 1)
	if err != nil || len(got) != 1 || got[0].State != "active" {
		t.Fatalf("sprints = %+v, %v, want the started copy", got, err)
	}
	cols, err := r.Columns(ctx, "p1", 1)
	if err != nil || len(cols) != 2 {
		t.Errorf("columns = %+v, %v, want both still there", cols, err)
	}
	if own := boardKeys(t, db, "p1", 1, ""); len(own) != 2 {
		t.Errorf("board membership = %v, want it untouched", own)
	}
	if inSprint := boardKeys(t, db, "p1", 1, "13"); len(inSprint) != 1 || inSprint[0] != "PLAT-2" {
		t.Errorf("sprint membership = %v, want it untouched", inSprint)
	}
}

// TestReplaceSprintIssuesRewritesOneScope is the write a half-finished
// completion corrects itself with: the cards it pushed out have left the
// sprint in Jira, and the sprint's own scope is the only thing that may
// change to say so.
func TestReplaceSprintIssuesRewritesOneScope(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	keys := map[string][]string{"": {"PLAT-1", "PLAT-2", "PLAT-3"}, "12": {"PLAT-1", "PLAT-2", "PLAT-3"}}
	if err := r.ReplaceBoard(ctx, "p1", board, oneColumn(), sampleSprints(), keys); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := r.ReplaceSprintIssues(ctx, "p1", 1, "12", []string{"PLAT-3"}); err != nil {
		t.Fatalf("replace sprint issues: %v", err)
	}

	if got := boardKeys(t, db, "p1", 1, "12"); len(got) != 1 || got[0] != "PLAT-3" {
		t.Errorf("sprint membership = %v, want only the card that stayed", got)
	}
	if got := boardKeys(t, db, "p1", 1, ""); len(got) != 3 {
		t.Errorf("board membership = %v, want the board's own list untouched", got)
	}
	sp, err := r.ListSprints(ctx, "p1", 1)
	if err != nil || len(sp) != len(sampleSprints()) {
		t.Errorf("sprints = %+v, %v, want them untouched", sp, err)
	}
}

// TestReplaceSprintsForgetsTheMembershipOfASprintThatIsGone is the other
// half of rewriting the sprint list. A sprint that has dropped out of it
// cannot be read again, since every membership read reaches its scope
// through a sprint the list still holds, so its rows would sit in
// board_issue for good and the table would grow by a whole sprint every
// time one was deleted in Jira.
func TestReplaceSprintsForgetsTheMembershipOfASprintThatIsGone(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	keys := map[string][]string{
		"":   {"PLAT-1", "PLAT-2", "PLAT-3"},
		"12": {"PLAT-1", "PLAT-2"},
		"13": {"PLAT-3"},
	}
	if err := r.ReplaceBoard(ctx, "p1", board, oneColumn(), sampleSprints(), keys); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Sprint 13 has been deleted in Jira; the list comes back without it.
	kept := []backend.Sprint{{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active"}}
	if err := r.ReplaceSprints(ctx, "p1", 1, kept); err != nil {
		t.Fatalf("replace sprints: %v", err)
	}

	if got := boardKeys(t, db, "p1", 1, "13"); len(got) != 0 {
		t.Errorf("membership of the sprint that is gone = %v, want it dropped with the sprint", got)
	}
	if got := boardKeys(t, db, "p1", 1, "12"); len(got) != 2 {
		t.Errorf("membership of the sprint that stayed = %v, want it untouched", got)
	}
	if got := boardKeys(t, db, "p1", 1, ""); len(got) != 3 {
		t.Errorf("board membership = %v, want the board's own list untouched", got)
	}
}

// TestSprintIssuesReadsTheBoardsOwnOrder is the read a completion subtracts
// the cards it moved from. The order is the board's rank order, which is
// what the view draws, and not the key order the completion's own search
// answers in.
func TestSprintIssuesReadsTheBoardsOwnOrder(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	keys := map[string][]string{"": {"PLAT-3", "PLAT-1"}, "12": {"PLAT-3", "PLAT-1"}}
	if err := r.ReplaceBoard(ctx, "p1", board, oneColumn(), sampleSprints(), keys); err != nil {
		t.Fatalf("seed: %v", err)
	}

	got, err := r.SprintIssues(ctx, "p1", 1, "12")
	if err != nil {
		t.Fatalf("sprint issues: %v", err)
	}
	if len(got) != 2 || got[0] != "PLAT-3" || got[1] != "PLAT-1" {
		t.Errorf("sprint issues = %v, want the board's own order", got)
	}
}

// TestBoardHasSprintAnswersForOneBoard is what a ceremony asks before it
// acts. Jira hands the same sprint to every board whose filter reaches it,
// so the question is never "does this sprint exist" but "is it this board's",
// and the sprint table's key is what answers it.
func TestBoardHasSprintAnswersForOneBoard(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	if err := r.ReplaceBoard(ctx, "p1", board, oneColumn(), sampleSprints(), nil); err != nil {
		t.Fatalf("seed: %v", err)
	}

	for _, tc := range []struct {
		name     string
		boardID  int
		sprintID string
		want     bool
	}{
		{"a sprint of this board", 1, "12", true},
		{"a sprint of another board", 2, "12", false},
		{"a sprint nobody holds", 1, "99", false},
		{"no sprint at all", 1, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.BoardHasSprint(ctx, "p1", tc.boardID, tc.sprintID)
			if err != nil {
				t.Fatalf("board has sprint: %v", err)
			}
			if got != tc.want {
				t.Errorf("board %d holds sprint %q = %v, want %v", tc.boardID, tc.sprintID, got, tc.want)
			}
		})
	}
}
