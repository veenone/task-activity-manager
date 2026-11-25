package issuerepo

import (
	"context"
	"database/sql"
	"fmt"

	"agile-suite/tam/internal/backend"
)

// replaceLinks swaps key's cached links for links, ids included. The sync
// calls it for every issue a page carries links for (#612), and the detail
// fetch for the issue it read.
func replaceLinks(ctx context.Context, tx *sql.Tx, profileID, key string, links []backend.Link) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM issue_link WHERE profile_id = ? AND from_key = ?`, profileID, key); err != nil {
		return fmt.Errorf("clear links for %s: %w", key, err)
	}
	for _, l := range links {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO issue_link (profile_id, from_key, to_key, link_type, direction, to_summary, to_type, link_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			profileID, key, l.Key, l.Type, l.Direction, l.Summary, l.IssueType, l.ID); err != nil {
			return fmt.Errorf("write link %s -> %s: %w", key, l.Key, err)
		}
	}
	return nil
}
