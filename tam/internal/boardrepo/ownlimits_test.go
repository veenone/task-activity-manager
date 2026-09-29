package boardrepo_test

import (
	"context"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
)

// plainColumns are three columns Jira sets no limit on, which is the board
// most of this file is about: the one whose admin never set a limit, where a
// limit of the user's own is the only one there will ever be.
func plainColumns() []backend.BoardColumn {
	return []backend.BoardColumn{
		{Name: "To Do", StatusIDs: []string{"1"}},
		{Name: "In Progress", StatusIDs: []string{"3"}},
		{Name: "Done", StatusIDs: []string{"5"}},
	}
}

// headByName finds one column head of a composed board.
func headByName(t *testing.T, heads []boardrepo.ColumnView, name string) boardrepo.ColumnView {
	t.Helper()
	for _, h := range heads {
		if h.Name == name {
			return h
		}
	}
	t.Fatalf("no column %q among %+v", name, heads)
	return boardrepo.ColumnView{}
}

// A limit the user set has to reach the column head, since the head is what
// both the board and the report's capacity section read.
func TestAColumnCarriesTheLimitTheUserSet(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	issues := seedBoard(t, r, plainColumns(), nil)
	if err := r.SetColumnLimit(ctx, "p1", 1, "In Progress", "3"); err != nil {
		t.Fatalf("set the limit: %v", err)
	}

	heads, err := r.ColumnHeads(ctx, issues, "p1", 1, "")
	if err != nil {
		t.Fatalf("column heads: %v", err)
	}
	progress := headByName(t, heads, "In Progress")
	if progress.LocalMax == nil || *progress.LocalMax != 3 {
		t.Errorf("In Progress localMax = %v, want the 3 the user typed", progress.LocalMax)
	}
	if progress.Max != nil {
		t.Errorf("In Progress max = %v, want Jira's own limit left absent", progress.Max)
	}
	if todo := headByName(t, heads, "To Do"); todo.LocalMax != nil {
		t.Errorf("To Do localMax = %v, want no limit on a column nobody set one on", todo.LocalMax)
	}
}

// The reorder case, which is the whole reason this limit is not on
// board_column: a boards sync replaces a board's columns wholesale, and a
// limit keyed on a column's position would land on whichever column ended up
// at that position.
func TestAUsersLimitFollowsItsColumnThroughAReorder(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	issues := seedBoard(t, r, plainColumns(), nil)
	if err := r.SetColumnLimit(ctx, "p1", 1, "In Progress", "3"); err != nil {
		t.Fatalf("set the limit: %v", err)
	}

	// Jira's admin reorders the board: In Progress moves from the middle to
	// the end, and Done takes the position the limit was set at.
	reordered := []backend.BoardColumn{
		{Name: "To Do", StatusIDs: []string{"1"}},
		{Name: "Done", StatusIDs: []string{"5"}},
		{Name: "In Progress", StatusIDs: []string{"3"}},
	}
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	if err := r.ReplaceBoard(ctx, "p1", board, reordered, nil, nil); err != nil {
		t.Fatalf("resync the reordered board: %v", err)
	}

	heads, err := r.ColumnHeads(ctx, issues, "p1", 1, "")
	if err != nil {
		t.Fatalf("column heads: %v", err)
	}
	if len(heads) != 3 || heads[1].Name != "Done" {
		t.Fatalf("columns = %+v, want the resync to have reordered them", heads)
	}
	progress := headByName(t, heads, "In Progress")
	if progress.LocalMax == nil || *progress.LocalMax != 3 {
		t.Errorf("In Progress localMax = %v, want the limit to have followed the column", progress.LocalMax)
	}
	if done := headByName(t, heads, "Done"); done.LocalMax != nil {
		t.Errorf("Done localMax = %v, want the limit not to have moved to the column now in that position", done.LocalMax)
	}
}

// Clearing is retyping nothing, and it removes the row rather than storing a
// limit of zero, which is a limit a board can really set.
func TestAnEmptyLimitClearsTheOneStored(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	issues := seedBoard(t, r, plainColumns(), nil)
	for _, v := range []string{"3", "  "} {
		if err := r.SetColumnLimit(ctx, "p1", 1, "In Progress", v); err != nil {
			t.Fatalf("set the limit to %q: %v", v, err)
		}
	}
	heads, err := r.ColumnHeads(ctx, issues, "p1", 1, "")
	if err != nil {
		t.Fatalf("column heads: %v", err)
	}
	if progress := headByName(t, heads, "In Progress"); progress.LocalMax != nil {
		t.Errorf("In Progress localMax = %v, want the limit cleared rather than set to zero", progress.LocalMax)
	}
}

// A limit is typed by a user, so it is input: a word, a negative number, a
// fraction and a number no board could mean are all routine, and each has to
// be refused rather than stored.
func TestATypedLimitIsCheckedBeforeItIsStored(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	issues := seedBoard(t, r, plainColumns(), nil)
	for _, typed := range []string{"five", "-1", "3.5", "100000"} {
		if err := r.SetColumnLimit(ctx, "p1", 1, "In Progress", typed); err == nil {
			t.Errorf("SetColumnLimit(%q) was accepted, want it refused", typed)
		}
	}
	// A limit of zero is not one of those: a column limited to nothing is a
	// column that is over the moment it holds a card.
	if err := r.SetColumnLimit(ctx, "p1", 1, "In Progress", "0"); err != nil {
		t.Fatalf("set a limit of zero: %v", err)
	}
	heads, err := r.ColumnHeads(ctx, issues, "p1", 1, "")
	if err != nil {
		t.Fatalf("column heads: %v", err)
	}
	if progress := headByName(t, heads, "In Progress"); progress.LocalMax == nil || *progress.LocalMax != 0 {
		t.Errorf("In Progress localMax = %v, want a stored zero", progress.LocalMax)
	}
}

// A purge of the profile takes the limits with it, and so does removing the
// board: a limit is a number about a column of that board, and once the
// board's columns are gone there is nothing left for it to describe.
func TestPurgingAndRemovingSweepTheUsersLimits(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		drop func(r *boardrepo.Repository) error
	}{
		{"purge the profile", func(r *boardrepo.Repository) error { return r.PurgeProfile(ctx, "p1") }},
		{"remove the board", func(r *boardrepo.Repository) error { return r.RemoveBoards(ctx, "p1", []int{1}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, db := newRepo(t)
			seedBoard(t, r, plainColumns(), nil)
			if err := r.SetColumnLimit(ctx, "p1", 1, "In Progress", "3"); err != nil {
				t.Fatalf("set the limit: %v", err)
			}
			// The other half of a deletion: one profile's second board and
			// another profile's board keep their own limits. A sweep with no
			// WHERE behind it passes the assertion above on its own.
			for _, kept := range [][2]any{{"p1", 2}, {"p2", 1}} {
				if err := r.SetColumnLimit(ctx, kept[0].(string), kept[1].(int), "In Progress", "7"); err != nil {
					t.Fatalf("set the limit to keep: %v", err)
				}
			}
			if err := tc.drop(r); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			var left int
			if err := db.QueryRow(
				`SELECT count(*) FROM board_column_limit WHERE profile_id = 'p1' AND board_id = 1`,
			).Scan(&left); err != nil {
				t.Fatalf("count the limits left: %v", err)
			}
			if left != 0 {
				t.Errorf("%d limits left after %s, want none", left, tc.name)
			}
			// A purge of the profile takes its other board with it; removing
			// one board does not.
			wantKept := 1
			if tc.name == "purge the profile" {
				wantKept = 0
			}
			var mine, other int
			if err := db.QueryRow(
				`SELECT count(*) FROM board_column_limit WHERE profile_id = 'p1' AND board_id = 2`,
			).Scan(&mine); err != nil {
				t.Fatalf("count the other board's limits: %v", err)
			}
			if err := db.QueryRow(
				`SELECT count(*) FROM board_column_limit WHERE profile_id = 'p2'`,
			).Scan(&other); err != nil {
				t.Fatalf("count the other profile's limits: %v", err)
			}
			if mine != wantKept {
				t.Errorf("board 2 kept %d limits after %s, want %d", mine, tc.name, wantKept)
			}
			if other != 1 {
				t.Errorf("p2 kept %d limits after %s, want its own limit untouched", other, tc.name)
			}
		})
	}
}
