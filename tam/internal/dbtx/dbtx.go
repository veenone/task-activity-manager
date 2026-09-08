// Package dbtx holds the one transaction helper the store packages share.
// issuerepo and boardrepo each had a byte-identical copy, with a comment in
// one pointing at the other; a helper two packages use is a module of its
// own rather than a duplicated dozen lines that can drift apart.
package dbtx

import (
	"context"
	"database/sql"
)

// In runs fn inside one transaction and commits it, so a write that touches
// a row, the journal, and the audit trail either lands whole or not at all.
// A failing fn leaves the deferred rollback to undo everything fn did.
func In(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// Querier is the read subset of *sql.DB and *sql.Tx, so a read helper can
// run either on the handle or inside one transaction, and the caller
// decides which without the helper knowing.
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// InRead runs fn inside one deferred transaction and rolls it back, so a
// read that takes several statements sees the database as of its first one
// rather than as of each. SQLite takes the snapshot at that first read and
// holds it until the transaction ends, and database/sql pins a transaction
// to a single connection, so every statement fn runs is on that snapshot.
// There is nothing to commit: fn writes nothing, and the rollback is how
// the snapshot is released.
func InRead(ctx context.Context, db *sql.DB, fn func(q Querier) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	return fn(tx)
}
