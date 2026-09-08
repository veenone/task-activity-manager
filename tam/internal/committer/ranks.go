package committer

import (
	"context"
	"fmt"
	"sort"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/issuerepo"
)

// The rank group, kept in its own file because it is the one board write
// that only means something relative to another card: it re-derives every
// neighbour from the board's final local order at push time, which none of
// the other two passes do.

// pushRanks is the last group, after every transition and sprint move has
// landed, because both change where a card sits. Each board's ranks push in
// that board's final local order, top to bottom, every card anchored
// against the card above it that actually landed.
//
// A rank needs no issue refresh afterwards, which halves the calls in the
// group that has the most rows. A rank whose board is gone from the store,
// whose card has left that board, or that has nothing left to anchor
// against is dropped with its reason rather than pushed against a card that
// is not there; its journal row stays, so the user can undo the move or
// sync the boards and commit again.
func (e *Engine) pushRanks(ctx context.Context, profileID string, w boardWriter, ranks []journal.PendingChange, res *Result) {
	if len(ranks) == 0 {
		return
	}
	byBoard := map[int][]journal.PendingChange{}
	var boards []int
	for _, p := range ranks {
		_, _, boardID := issuerepo.ParseRank(p.AfterVal)
		if _, seen := byBoard[boardID]; !seen {
			boards = append(boards, boardID)
		}
		byBoard[boardID] = append(byBoard[boardID], p)
	}
	sort.Ints(boards)
	for _, boardID := range boards {
		e.pushBoardRanks(ctx, profileID, w, boardID, byBoard[boardID], res)
	}
}

// pushBoardRanks pushes one board's ranks in that board's order.
func (e *Engine) pushBoardRanks(ctx context.Context, profileID string, w boardWriter, boardID int, rows []journal.PendingChange, res *Result) {
	if e.order == nil {
		e.dropRanks(rows, res, fmt.Errorf("this commit was built with no board order to rank against"))
		return
	}
	order, err := e.order.CellOrder(ctx, profileID, boardID)
	if err != nil {
		e.dropRanks(rows, res, fmt.Errorf("board %d's order could not be read: %w", boardID, err))
		return
	}
	want := make(map[string]journal.PendingChange, len(rows))
	for _, p := range rows {
		want[p.EntityKey] = p
	}
	// prev is the last card this board is known to hold in its final
	// position: one nobody moved, or one whose rank Jira just took. A rank
	// that failed leaves prev where it was, because anchoring the next card
	// against a card Jira never moved would commit an order the board never
	// showed.
	prev := ""
	for i, key := range order {
		p, ok := want[key]
		if !ok {
			prev = key
			continue
		}
		delete(want, key)
		anchor, before, err := rankAnchor(order, i, prev, boardID)
		if err != nil {
			res.Failures = append(res.Failures, boardFailure(p, err, false))
			continue
		}
		if err := w.RankIssue(ctx, key, anchor, before); err != nil {
			res.Failures = append(res.Failures, boardFailure(p, err, true))
			continue
		}
		e.clearMove(ctx, profileID, p, res)
		res.Moved = append(res.Moved, Moved{Key: key, EntityType: p.EntityType, Target: anchor, Side: rankSide(before)})
		prev = key
	}
	// Whatever is left never appeared in the order: the card has gone from
	// the board, so the cell the rank was measured in no longer holds it.
	for _, p := range rows {
		if _, still := want[p.EntityKey]; !still {
			continue
		}
		res.Failures = append(res.Failures, boardFailure(p, fmt.Errorf("%s is no longer on board %d, so there is nothing to rank it against", p.EntityKey, boardID), false))
	}
}

// rankAnchor is the card one rank is pushed against and which side of it.
// Every card but the topmost goes after the card above it, which is how two
// cards ranked against each other keep one order rather than becoming a
// cycle.
//
// The topmost card has nothing above it, so it goes before the card below
// it instead: "X before Y" and "X after W" put X in exactly the same place
// in Jira's one global rank, and only the card at index 0 can take this
// branch, so the two can never disagree about a pair. Dragging a card to
// the top of the leftmost column is one of the commonest gestures on a
// board, and anchoring only downwards is what used to drop it.
//
// Two cards have nothing left to rank against: the only card on a board,
// and a card whose every predecessor failed its own rank.
func rankAnchor(order []string, i int, prev string, boardID int) (string, bool, error) {
	if prev != "" {
		return prev, false, nil
	}
	if i > 0 {
		return "", false, fmt.Errorf("no card above %s on board %d was ranked, so there is nothing to rank it against", order[i], boardID)
	}
	if len(order) < 2 {
		return "", false, fmt.Errorf("%s is the only card on board %d, so there is nothing to rank it against", order[i], boardID)
	}
	return order[1], true, nil
}

// rankSide is how a Moved names the side a rank went, in the words the
// journal already spells them with.
func rankSide(before bool) string {
	if before {
		return issuerepo.RankSideBefore
	}
	return issuerepo.RankSideAfter
}

// dropRanks reports every rank of one board with the same reason.
func (e *Engine) dropRanks(rows []journal.PendingChange, res *Result, err error) {
	for _, p := range rows {
		res.Failures = append(res.Failures, boardFailure(p, err, false))
	}
}
