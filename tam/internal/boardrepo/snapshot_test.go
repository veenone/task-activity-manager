package boardrepo_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
)

// The two shapes the writer alternates between. They differ in their
// columns and in their membership at once, the way a boards sync does, so a
// read that mixes one board's columns with the other's cards draws a shape
// that is neither.
var (
	smallBoard = map[string][]string{"": {"PLAT-1"}}
	wholeBoard = map[string][]string{"": {"PLAT-1", "PLAT-2"}}
)

// replacements is how many times the writer swaps the board between the two
// shapes. Two would be enough for a reader that happened to land between
// them; this many is what makes a reader spinning beside it land there on
// every run rather than on a lucky one.
const replacements = 30

// boardShape is the drawn board as one line: each column with the keys it
// holds, and the count of cards no column collected. Both halves matter.
// Old columns against new membership draws a card into no column at all and
// shows up in the unmapped count, while new columns against old membership
// draws an empty column that the counts alone would not report.
func boardShape(view boardrepo.BoardView) string {
	parts := make([]string, 0, len(view.Columns))
	for i, c := range view.Columns {
		keys := []string{}
		for _, lane := range view.Lanes {
			for _, card := range lane.Cells[i] {
				keys = append(keys, card.Key)
			}
		}
		parts = append(parts, c.Name+"["+strings.Join(keys, " ")+"]")
	}
	return fmt.Sprintf("%s unmapped=%d", strings.Join(parts, " "), view.Unmapped)
}

// TestABoardReadSeesOneBoardOrTheOther drives a reader in a loop while a
// writer replaces the board back and forth between two shapes. Every read
// has to answer with one whole shape or the other: a board is written in
// one transaction, so no moment exists in which half of one and half of the
// other is the truth, and a reader that draws one has read across a write
// rather than inside a snapshot.
//
// Without the read transaction this fails: Board takes half a dozen
// statements, and on the handle each one sees whatever is committed as it
// runs.
func TestABoardReadSeesOneBoardOrTheOther(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	issues := newIssues(card("PLAT-1", "To Do", "1"), card("PLAT-2", "Done", "5"))
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}

	replace := func(cols []backend.BoardColumn, keys map[string][]string) {
		if err := r.ReplaceBoard(ctx, "p1", board, cols, nil, keys); err != nil {
			t.Errorf("replace board: %v", err)
		}
	}
	replace(oneColumn(), smallBoard)

	// The shapes as they read when nothing is being written, taken from the
	// store itself rather than spelled out, so the test cannot disagree with
	// the view about how a whole board looks.
	small := boardShape(mustBoard(t, r, issues))
	replace(twoColumns(), wholeBoard)
	whole := boardShape(mustBoard(t, r, issues))
	if small == whole {
		t.Fatalf("both shapes read as %q; the test cannot tell them apart", small)
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	torn := make(chan string, 1)
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
			if got := boardShape(view); got != small && got != whole {
				select {
				case torn <- got:
				default:
				}
				return
			}
		}
	}()

	for i := 0; i < replacements; i++ {
		if i%2 == 0 {
			replace(oneColumn(), smallBoard)
			continue
		}
		replace(twoColumns(), wholeBoard)
	}
	close(stop)
	<-done
	select {
	case got := <-torn:
		t.Fatalf("a read drew %q, want %q or %q and never a board made of both", got, small, whole)
	default:
	}
}

// mustBoard reads board 1 the way the Boards view does, failing the test on
// an error rather than handing a zero view back.
func mustBoard(t *testing.T, r *boardrepo.Repository, issues boardrepo.IssueSource) boardrepo.BoardView {
	t.Helper()
	view, err := r.Board(context.Background(), issues, "p1", 1, "", boardrepo.SwimlaneNone)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	return view
}
