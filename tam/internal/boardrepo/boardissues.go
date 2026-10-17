package boardrepo

import (
	"context"
	"fmt"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/dbtx"
)

// allBoardKeysSQL is every card the board holds, in any sprint or its
// backlog, once each. A card cached under two scopes (a sprint and the
// backlog of a board whose filter reaches it both ways) is one card.
const allBoardKeysSQL = `
	SELECT key FROM board_issue WHERE profile_id = ? AND board_id = ?
	GROUP BY key ORDER BY MIN(sprint_id = ''), MIN(position), key`

// BoardIssues is every cached card on the board, with no view budget: the
// board allocation (#288) adds up who holds what, and a sum short of the
// cards past MaxCardsPerView would understate whoever holds them. Drafts and
// pending moves are not replayed; the allocation describes the board as the
// last sync left it, and says so.
func (r *Repository) BoardIssues(ctx context.Context, issues IssueSource, profileID string, boardID int) ([]backend.Issue, error) {
	var out []backend.Issue
	err := r.inReadTx(ctx, func(q dbtx.Querier) error {
		rows, err := q.QueryContext(ctx, allBoardKeysSQL, profileID, boardID)
		if err != nil {
			return fmt.Errorf("board %d issue keys: %w", boardID, err)
		}
		defer rows.Close()
		var keys []string
		for rows.Next() {
			var key string
			if err := rows.Scan(&key); err != nil {
				return err
			}
			keys = append(keys, key)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		out, err = issues.IssuesByKeys(ctx, q, profileID, keys)
		return err
	})
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []backend.Issue{}
	}
	return out, nil
}
