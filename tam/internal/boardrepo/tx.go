package boardrepo

import (
	"context"
	"database/sql"

	"agile-suite/tam/internal/dbtx"
)

// inTx runs fn inside one transaction, through the helper issuerepo shares,
// so a replace never leaves the table holding a delete without its inserts.
func (r *Repository) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return dbtx.In(ctx, r.db, fn)
}

// inReadTx runs fn inside one deferred transaction, so a read that takes
// several statements sees one board rather than the moment between two
// writes. ReplaceBoard writes a board's row, columns, sprints and
// membership together; a reader issuing its statements on the handle sees
// whatever is committed as each one runs, which is how a board with new
// columns and old membership reaches the view.
//
// fn is handed a querier and not the transaction: a read has nothing to
// exec, and the narrower type is what lets the same helpers run on the
// handle where no snapshot is wanted.
func (r *Repository) inReadTx(ctx context.Context, fn func(q dbtx.Querier) error) error {
	return dbtx.InRead(ctx, r.db, fn)
}
