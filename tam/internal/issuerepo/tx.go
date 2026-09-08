package issuerepo

import (
	"context"
	"database/sql"

	"agile-suite/tam/internal/dbtx"
)

// inTx runs fn inside one transaction, through the helper boardrepo shares.
// Every write in this package that needs more than one statement goes
// through here rather than open-coding the same begin, deferred rollback,
// and commit.
func (r *Repository) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return dbtx.In(ctx, r.db, fn)
}
