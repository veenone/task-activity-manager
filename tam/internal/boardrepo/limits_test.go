package boardrepo_test

import (
	"context"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
)

func limit(n int) *int { return &n }

// limitColumns are three columns of the one board shape this file is about:
// one with a pair of limits, one with a maximum of nothing, and one with no
// limit at all, which is the ordinary case.
func limitColumns() []backend.BoardColumn {
	return []backend.BoardColumn{
		{Name: "To Do", StatusIDs: []string{"1"}, Constraint: backend.ConstraintIssueCount},
		{Name: "In Progress", StatusIDs: []string{"3"}, Min: limit(1), Max: limit(2), Constraint: backend.ConstraintIssueCount},
		{Name: "Done", StatusIDs: []string{"5"}, Max: limit(0), Constraint: backend.ConstraintIssueCount},
	}
}

// A limit read back has to be the limit that was written, and a column with
// none has to read as having none. Both go through the same nullable
// columns, so a store that flattened an unset limit to zero would report
// every ordinary column as over a limit of nothing.
func TestColumnLimitsSurviveTheStore(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	board := backend.Board{ID: 1, Name: "PLAT Kanban", Type: backend.BoardTypeKanban}
	if err := r.ReplaceBoard(ctx, "p1", board, limitColumns(), nil, nil); err != nil {
		t.Fatalf("replace board: %v", err)
	}

	cols, err := r.Columns(ctx, "p1", 1)
	if err != nil {
		t.Fatalf("columns: %v", err)
	}
	if len(cols) != 3 {
		t.Fatalf("columns = %+v, want 3", cols)
	}
	if cols[0].Min != nil || cols[0].Max != nil {
		t.Errorf("To Do = min %v max %v, want no limit", cols[0].Min, cols[0].Max)
	}
	if cols[1].Min == nil || *cols[1].Min != 1 || cols[1].Max == nil || *cols[1].Max != 2 {
		t.Errorf("In Progress = min %v max %v, want 1 and 2", cols[1].Min, cols[1].Max)
	}
	if cols[2].Max == nil || *cols[2].Max != 0 {
		t.Errorf("Done max = %v, want a stored zero rather than an unset limit", cols[2].Max)
	}
	if cols[1].Constraint != backend.ConstraintIssueCount {
		t.Errorf("In Progress constraint = %q, want the board's own", cols[1].Constraint)
	}
}

// The view's column heads are where the limit is read, so each one carries
// its own pair and the count that pair is measured against. Counted is that
// count and not Total: a board counting without subtasks measures a
// different number from the one the head prints as cards.
func TestBoardColumnsCarryTheLimitAndTheCountedCards(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	cols := limitColumns()
	for i := range cols {
		cols[i].Constraint = backend.ConstraintExclSubtasks
	}
	cards := []backend.Issue{
		{Key: "PLAT-1", Type: backend.TypeStory, Status: "In Progress", StatusID: "3"},
		{Key: "PLAT-2", Type: backend.TypeSubtask, ParentKey: "PLAT-1", Status: "In Progress", StatusID: "3"},
		{Key: "PLAT-3", Type: backend.TypeTask, Status: "In Progress", StatusID: "3"},
	}
	issues := seedBoard(t, r, cols, cards)

	view, err := r.Board(ctx, issues, "p1", 1, "", boardrepo.SwimlaneNone)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	progress := view.Columns[1]
	if progress.Total != 3 {
		t.Fatalf("In Progress total = %d, want the three cards it holds", progress.Total)
	}
	// Two of the three are not subtasks, and this board's limit counts
	// without them, which is one card inside a maximum of two.
	if progress.Counted != 2 {
		t.Errorf("In Progress counted = %d, want 2, the cards this board's limit measures", progress.Counted)
	}
	if progress.Max == nil || *progress.Max != 2 {
		t.Errorf("In Progress max = %v, want the stored limit on the column head", progress.Max)
	}
	if progress.Constraint != backend.ConstraintExclSubtasks {
		t.Errorf("In Progress constraint = %q, want what the board counts", progress.Constraint)
	}
	if view.Columns[0].Max != nil {
		t.Errorf("To Do max = %v, want a column with no limit to carry none", view.Columns[0].Max)
	}
}
