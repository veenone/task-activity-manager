package boardrepo

import (
	"context"
	"fmt"
)

// The commit pass pushes a rank against the card above it in the board's
// final local order, re-derived here at push time rather than taken from
// the key that was journaled at drop time: a neighbour that moved or left
// makes the journaled key meaningless. This is the read that order comes
// from, kept out of view.go because it answers a committer, not a view: no
// lanes, no caps, no counts, just the keys.

// Order pairs the board tables with the issue cache, so a caller holding
// both hands the committer one value that answers CellOrder. The committer
// never imports this package: it declares the one-method interface Order
// satisfies, the way boardrepo declares IssueSource rather than importing
// issuerepo.
type Order struct {
	Boards *Repository
	Issues IssueSource
}

// CellOrder is the board's final local order, top to bottom: every card the
// board holds, with the journal's pending transitions, sprint moves, and
// ranks applied, columns left to right and each column's cards in the order
// the view draws them. It is the whole board, never one sprint, because
// Jira's rank is one order across the board and a card ranked in a sprint
// view still moves in that one order.
//
// One deliberate difference from what the view draws: nothing is capped.
// MaxCardsPerCell decides what a person can read, not where a card belongs.
// Drafts never arrive here at all, since the order is built from the key
// list Jira answered the board endpoint with, which is the reason a draft
// can neither be ranked nor anchor another card's rank.
//
// A board the store does not hold answers with an error, which is what the
// commit pass reports when it drops that board's ranks.
func (o Order) CellOrder(ctx context.Context, profileID string, boardID int) ([]string, error) {
	cols, err := o.Boards.Columns(ctx, profileID, boardID)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("board %d is not in the store; sync the boards first", boardID)
	}
	boardKeys, err := o.Boards.issueKeys(ctx, profileID, boardID, "")
	if err != nil {
		return nil, err
	}
	moves, err := o.Issues.PendingMoves(ctx, profileID)
	if err != nil {
		return nil, err
	}
	cards, err := o.Issues.IssuesByKeys(ctx, profileID, boardKeys)
	if err != nil {
		return nil, err
	}
	byStatus, draftColumn := columnIndex(cols)
	cards = applyMoves(cards, moves, "")
	cards = rankCards(cards, moves, byStatus, draftColumn, SwimlaneNone)

	// One bucket per column, filled in the order the cards arrive, so a
	// column reads exactly as the cell does.
	buckets := make([][]string, len(cols))
	for _, card := range cards {
		col, ok := placeCard(card, byStatus, draftColumn)
		if !ok {
			// A status no column collects is not on the board, and the view
			// counts it in Unmapped rather than drawing it. A card the
			// board never draws cannot anchor a rank.
			continue
		}
		buckets[col] = append(buckets[col], card.Key)
	}
	out := make([]string, 0, len(cards))
	for _, b := range buckets {
		out = append(out, b...)
	}
	return out, nil
}
