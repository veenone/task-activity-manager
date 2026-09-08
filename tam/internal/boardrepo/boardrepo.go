// Package boardrepo is the store layer over tam.db for the Boards view: the
// boards a profile has synced, their columns, their sprints, and which issue
// keys each board holds. The cards themselves stay in the issue cache;
// boardrepo reads them through IssueSource so it never has to import
// issuerepo, and composes the view in view.go.
//
// Every method takes the profile id, because every table is scoped by it.
package boardrepo

import (
	"context"
	"database/sql"
	"fmt"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/dbtx"
)

// Repository runs the queries. It holds no state beyond the handle.
type Repository struct {
	db *sql.DB
}

// New wraps an open tam.db handle.
func New(db *sql.DB) *Repository { return &Repository{db: db} }

// Board is one cached board.
type Board struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// Sprint is one cached sprint. State is Jira's own lowercase value: active,
// future, or closed.
type Sprint struct {
	ID        int    `json:"id"`
	BoardID   int    `json:"boardId"`
	Name      string `json:"name"`
	State     string `json:"state"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

// IssueSource is what the view needs from the issue cache. Keeping it an
// interface is what lets boardrepo compose a board without importing
// issuerepo; app.go passes the issue repository, which already has all
// three methods.
//
// Every method takes the querier its statement runs on, because a board
// read is one snapshot: the board tables and the issue cache are read
// inside the same transaction, and a source that opened its own handle
// would read the cache as of a later moment than the columns it is being
// placed into.
type IssueSource interface {
	// IssuesByKeys returns the cached rows for the board's own keys, in the
	// order they were asked for.
	IssuesByKeys(ctx context.Context, q dbtx.Querier, profileID string, keys []string) ([]backend.Issue, error)
	// DraftIssues returns the profile's local drafts. Jira's board issue
	// list can never name a draft key, so the board reads them separately
	// or they never reach a board at all.
	DraftIssues(ctx context.Context, q dbtx.Querier, profileID string) ([]backend.Issue, error)
	// PendingMoves returns the board intents the journal holds, one per
	// issue. The view applies them as it places the cards, so a card is
	// drawn where it was dropped and not where the last sync left it.
	PendingMoves(ctx context.Context, q dbtx.Querier, profileID string) ([]backend.PendingMove, error)
}

// PurgeProfile drops everything the board tables hold for a profile. The
// issue cache is issuerepo's to purge: two purges naming the same tables is
// the drift that leaves one behind when a fifth table arrives.
func (r *Repository) PurgeProfile(ctx context.Context, profileID string) error {
	return r.inTx(ctx, func(tx *sql.Tx) error {
		for _, table := range []string{"board", "board_column", "board_issue", "sprint"} {
			if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE profile_id = ?`, profileID); err != nil {
				return fmt.Errorf("purge %s for %s: %w", table, profileID, err)
			}
		}
		return nil
	})
}
