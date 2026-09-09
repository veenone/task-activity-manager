package boardrepo

import (
	"context"
	"fmt"

	"agile-suite/tam/internal/dbtx"
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
	var out []string
	err := o.Boards.inReadTx(ctx, func(q dbtx.Querier) error {
		keys, err := cellOrder(ctx, q, o.Issues, profileID, boardID)
		out = keys
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// cellOrder is the order itself, every statement on one snapshot, so the
// columns a card is placed into and the membership it is placed from are
// the same board. A rank pushed from an order half of one sync and half of
// the next would anchor against a card that board never drew.
func cellOrder(ctx context.Context, q dbtx.Querier, issues IssueSource, profileID string, boardID int) ([]string, error) {
	cols, err := columnsOf(ctx, q, profileID, boardID)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("board %d is not in the store; sync the boards first", boardID)
	}
	boardKeys, err := issueKeys(ctx, q, profileID, boardID, "")
	if err != nil {
		return nil, err
	}
	moves, err := issues.PendingMoves(ctx, q, profileID)
	if err != nil {
		return nil, err
	}
	cards, err := issues.IssuesByKeys(ctx, q, profileID, boardKeys)
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
