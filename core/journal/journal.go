// Package journal is the pending-change journal and audit trail each suite
// app keeps in its own database: the two tables, the transaction-level
// helpers every local edit goes through, and the reads that Commit and the
// activity views need. The helpers were lifted from XTM's testrepo, which
// now delegates to them. Reverting an entity's columns on discard stays in
// each app, since only the app knows its columns.
package journal

import (
	"database/sql"
	"errors"
	"fmt"
	"os/user"
	"strings"
	"time"
)

// DDL creates the two tables and their indexes. Both apps include it in
// their store's base schema; every statement is idempotent.
const DDL = `
CREATE TABLE IF NOT EXISTS pending_change (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	profile_id   TEXT NOT NULL,
	entity_type  TEXT NOT NULL,
	entity_key   TEXT NOT NULL,
	field        TEXT NOT NULL,
	before_val   TEXT NOT NULL DEFAULT '',
	after_val    TEXT NOT NULL DEFAULT '',
	base_version TEXT NOT NULL DEFAULT '',
	created_at   TEXT NOT NULL,
	UNIQUE (profile_id, entity_type, entity_key, field)
);
CREATE TABLE IF NOT EXISTS audit_log (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	profile_id  TEXT NOT NULL,
	occurred_at TEXT NOT NULL,
	actor       TEXT NOT NULL DEFAULT '',
	entity_type TEXT NOT NULL,
	entity_key  TEXT NOT NULL,
	action      TEXT NOT NULL,
	field       TEXT NOT NULL DEFAULT '',
	before_val  TEXT NOT NULL DEFAULT '',
	after_val   TEXT NOT NULL DEFAULT '',
	note        TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_pending_change_profile ON pending_change(profile_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_profile_time ON audit_log(profile_id, occurred_at DESC);`

// ErrNotFound is returned by Get for an id the profile does not have.
var ErrNotFound = errors.New("journal: pending change not found")

// PendingChange is one journaled field change. BaseVersion is the remote
// version the edit was made against; Commit compares it with the remote.
type PendingChange struct {
	ID          int64  `json:"id"`
	EntityType  string `json:"entityType"`
	EntityKey   string `json:"entityKey"`
	Field       string `json:"field"`
	BeforeVal   string `json:"beforeVal"`
	AfterVal    string `json:"afterVal"`
	BaseVersion string `json:"baseVersion"`
	CreatedAt   string `json:"createdAt"`
}

// AuditEntry is one row of the local audit trail: who, what, when, and the
// before and after values.
type AuditEntry struct {
	ID         int64  `json:"id"`
	OccurredAt string `json:"occurredAt"`
	Actor      string `json:"actor"`
	EntityType string `json:"entityType"`
	EntityKey  string `json:"entityKey"`
	Action     string `json:"action"`
	Field      string `json:"field"`
	BeforeVal  string `json:"beforeVal"`
	AfterVal   string `json:"afterVal"`
	Note       string `json:"note"`
}

// Execer is what the write helpers need; *sql.Tx and *sql.DB both satisfy
// it. Writes belong inside the caller's transaction, next to the entity
// update they record.
type Execer interface {
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
}

// Querier is what the reads need; *sql.Tx and *sql.DB both satisfy it.
type Querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// Upsert records a field change, keeping one row per entity and field. A
// first edit inserts; a later edit updates after_val and keeps the original
// before_val; a value returning to the original before_val deletes the row.
func Upsert(tx Execer, profileID, entityType, entityKey, field, currentVal, newValue, baseVersion string) error {
	var existingBefore string
	err := tx.QueryRow(
		`SELECT before_val FROM pending_change
		 WHERE profile_id = ? AND entity_type = ? AND entity_key = ? AND field = ?`,
		profileID, entityType, entityKey, field,
	).Scan(&existingBefore)

	now := time.Now().UTC().Format(time.RFC3339)

	if errors.Is(err, sql.ErrNoRows) {
		_, ierr := tx.Exec(
			`INSERT INTO pending_change
			   (profile_id, entity_type, entity_key, field, before_val, after_val, base_version, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			profileID, entityType, entityKey, field, currentVal, newValue, baseVersion, now,
		)
		if ierr != nil {
			return fmt.Errorf("insert pending change: %w", ierr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("read existing pending: %w", err)
	}

	if newValue == existingBefore {
		if _, derr := tx.Exec(
			`DELETE FROM pending_change
			 WHERE profile_id = ? AND entity_type = ? AND entity_key = ? AND field = ?`,
			profileID, entityType, entityKey, field,
		); derr != nil {
			return fmt.Errorf("delete pending: %w", derr)
		}
		return nil
	}

	if _, uerr := tx.Exec(
		`UPDATE pending_change SET after_val = ?, created_at = ?
		 WHERE profile_id = ? AND entity_type = ? AND entity_key = ? AND field = ?`,
		newValue, now, profileID, entityType, entityKey, field,
	); uerr != nil {
		return fmt.Errorf("update pending: %w", uerr)
	}
	return nil
}

// Put records or updates a field change without the revert check, for
// callers that have already decided the edit is genuine against a freshly
// read base.
func Put(tx Execer, profileID, entityType, entityKey, field, currentVal, newValue, baseVersion string) error {
	var existingBefore string
	err := tx.QueryRow(
		`SELECT before_val FROM pending_change
		 WHERE profile_id = ? AND entity_type = ? AND entity_key = ? AND field = ?`,
		profileID, entityType, entityKey, field,
	).Scan(&existingBefore)

	now := time.Now().UTC().Format(time.RFC3339)

	if errors.Is(err, sql.ErrNoRows) {
		_, ierr := tx.Exec(
			`INSERT INTO pending_change
			   (profile_id, entity_type, entity_key, field, before_val, after_val, base_version, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			profileID, entityType, entityKey, field, currentVal, newValue, baseVersion, now,
		)
		if ierr != nil {
			return fmt.Errorf("insert pending change: %w", ierr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("read existing pending: %w", err)
	}

	if _, uerr := tx.Exec(
		`UPDATE pending_change SET after_val = ?, created_at = ?
		 WHERE profile_id = ? AND entity_type = ? AND entity_key = ? AND field = ?`,
		newValue, now, profileID, entityType, entityKey, field,
	); uerr != nil {
		return fmt.Errorf("update pending: %w", uerr)
	}
	return nil
}

// Audit appends one entry to the trail, stamped with the OS user.
func Audit(tx Execer, profileID, entityType, entityKey, action, field, beforeVal, afterVal, note string) error {
	if _, err := tx.Exec(
		`INSERT INTO audit_log
		   (profile_id, occurred_at, actor, entity_type, entity_key, action, field, before_val, after_val, note)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		profileID, time.Now().UTC().Format(time.RFC3339),
		Actor(), entityType, entityKey, action, field, beforeVal, afterVal, note,
	); err != nil {
		return fmt.Errorf("audit log: %w", err)
	}
	return nil
}

// Actor returns the OS username for the audit trail, or "user" when it
// cannot be resolved.
func Actor() string {
	u, err := user.Current()
	if err != nil || u == nil || u.Username == "" {
		return "user"
	}
	return u.Username
}

const pendingColumns = `id, entity_type, entity_key, field, before_val, after_val, base_version, created_at`

func scanPending(rows *sql.Rows) ([]PendingChange, error) {
	defer rows.Close()
	out := []PendingChange{}
	for rows.Next() {
		var p PendingChange
		if err := rows.Scan(&p.ID, &p.EntityType, &p.EntityKey, &p.Field,
			&p.BeforeVal, &p.AfterVal, &p.BaseVersion, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// List returns every pending change for the profile, newest first.
func List(db Querier, profileID string) ([]PendingChange, error) {
	rows, err := db.Query(
		`SELECT `+pendingColumns+` FROM pending_change WHERE profile_id = ?
		 ORDER BY created_at DESC, id DESC`, profileID)
	if err != nil {
		return nil, fmt.Errorf("list pending changes: %w", err)
	}
	return scanPending(rows)
}

// ListForKey returns the pending changes of one entity key across entity
// types, oldest first, so a commit pass applies them in the order they were
// made. TAM keeps a draft's create row and its later edits under one key.
func ListForKey(db Querier, profileID, entityKey string) ([]PendingChange, error) {
	rows, err := db.Query(
		`SELECT `+pendingColumns+` FROM pending_change
		 WHERE profile_id = ? AND entity_key = ? ORDER BY id`,
		profileID, entityKey)
	if err != nil {
		return nil, fmt.Errorf("list pending changes for %s: %w", entityKey, err)
	}
	return scanPending(rows)
}

// Get returns one pending change by id, or ErrNotFound.
func Get(db Querier, profileID string, id int64) (PendingChange, error) {
	var p PendingChange
	err := db.QueryRow(
		`SELECT `+pendingColumns+` FROM pending_change WHERE profile_id = ? AND id = ?`, profileID, id,
	).Scan(&p.ID, &p.EntityType, &p.EntityKey, &p.Field, &p.BeforeVal, &p.AfterVal, &p.BaseVersion, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return PendingChange{}, ErrNotFound
	}
	if err != nil {
		return PendingChange{}, fmt.Errorf("read pending change: %w", err)
	}
	return p, nil
}

// Delete removes the given pending changes of the profile.
func Delete(tx Execer, profileID string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	marks := strings.TrimSuffix(strings.Repeat("?, ", len(ids)), ", ")
	args := make([]any, 0, len(ids)+1)
	args = append(args, profileID)
	for _, id := range ids {
		args = append(args, id)
	}
	if _, err := tx.Exec(`DELETE FROM pending_change WHERE profile_id = ? AND id IN (`+marks+`)`, args...); err != nil {
		return fmt.Errorf("delete pending changes: %w", err)
	}
	return nil
}

// DeleteForKey removes every pending change of one entity key and returns
// how many went.
func DeleteForKey(tx Execer, profileID, entityKey string) (int, error) {
	res, err := tx.Exec(
		`DELETE FROM pending_change WHERE profile_id = ? AND entity_key = ?`,
		profileID, entityKey)
	if err != nil {
		return 0, fmt.Errorf("delete pending changes for %s: %w", entityKey, err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// SetBaseVersion rebases every pending change of one entity key onto
// version, which is what an override resolution does.
func SetBaseVersion(tx Execer, profileID, entityKey, version string) error {
	if _, err := tx.Exec(
		`UPDATE pending_change SET base_version = ? WHERE profile_id = ? AND entity_key = ?`,
		version, profileID, entityKey); err != nil {
		return fmt.Errorf("rebase pending changes for %s: %w", entityKey, err)
	}
	return nil
}

// Entries returns audit entries newest first, for one entity when entityKey
// is set or for the whole profile when it is empty. limit defaults to 200
// and caps at 1000.
func Entries(db Querier, profileID, entityKey string, limit int) ([]AuditEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	where := `profile_id = ?`
	args := []any{profileID}
	if entityKey != "" {
		where += ` AND entity_key = ?`
		args = append(args, entityKey)
	}
	rows, err := db.Query(
		`SELECT id, occurred_at, actor, entity_type, entity_key, action, field, before_val, after_val, note
		 FROM audit_log WHERE `+where+` ORDER BY occurred_at DESC, id DESC LIMIT ?`,
		append(args, limit)...)
	if err != nil {
		return nil, fmt.Errorf("list audit entries: %w", err)
	}
	defer rows.Close()
	out := []AuditEntry{}
	for rows.Next() {
		var a AuditEntry
		if err := rows.Scan(&a.ID, &a.OccurredAt, &a.Actor, &a.EntityType, &a.EntityKey,
			&a.Action, &a.Field, &a.BeforeVal, &a.AfterVal, &a.Note); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
