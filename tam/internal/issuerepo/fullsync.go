package issuerepo

import (
	"context"
	"fmt"
	"time"
)

// FinishFullSync is the last step of a full sync whose every page has landed
// with syncedAt. It drops the synced rows no page wrote, which are the issues
// Jira no longer returns, along with the cached links and details a full
// sync refetches. Drafts stay.
//
// It runs only once the pages are all in. Clearing the cache inside the first
// page's transaction left a sync that failed on page 3 with two pages of
// issues and lists that looked current (#534); until this runs, a failed full
// sync leaves the previous rows in place beside the ones it refreshed.
func (r *Repository) FinishFullSync(ctx context.Context, profileID string, syncedAt time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	draft := DraftPrefix + "%"
	if _, err := tx.ExecContext(ctx, `DELETE FROM issue_link WHERE profile_id = ? AND from_key NOT LIKE ?`, profileID, draft); err != nil {
		return fmt.Errorf("clear links: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM issue WHERE profile_id = ? AND key NOT LIKE ? AND synced_at <> ?`,
		profileID, draft, syncedAt.UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("clear issues: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE issue SET detail_json = NULL, detail_fetched_at = NULL WHERE profile_id = ? AND key NOT LIKE ?`,
		profileID, draft); err != nil {
		return fmt.Errorf("clear details: %w", err)
	}
	return tx.Commit()
}
