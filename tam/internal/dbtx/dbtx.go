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
