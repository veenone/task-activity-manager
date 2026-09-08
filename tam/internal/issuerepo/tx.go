package issuerepo

import (
	"context"
	"database/sql"
)

// inTx runs fn inside one transaction and commits it, so a write that
// touches the row, the journal, and the audit trail either lands whole or
// not at all. Every write in this package that needs more than one
// statement goes through here rather than open-coding the same begin,
// deferred rollback, and commit; boardrepo has the same helper over its own
// tables.
func (r *Repository) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
