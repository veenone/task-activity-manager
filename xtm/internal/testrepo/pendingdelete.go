package testrepo

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// The journal side of deleting a container, a precondition or a requirement,
// and of rewriting a key inside the key sets other pending rows hold. The
// three deletes used to carry their own copy of this and drifted apart.

// queueDelete records the local delete of an entity whose cached rows the
// caller has already removed. related lists the entity types whose pending
// rows for the same key the delete supersedes.
//
// An entity that was only created locally has nothing to delete in Jira, so
// its pending create is dropped instead and cancelled is true. Otherwise the
// delete is queued with snap, which discard restores from. noun names the
// entity in the pending row's field, the audit actions and the errors.
func queueDelete(tx *sql.Tx, profileID, key, createType, deleteType, noun, summary string, snap any, related ...string) (cancelled bool, err error) {
	var createID int64
	err = tx.QueryRow(
		`SELECT id FROM pending_change
		 WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
		profileID, createType, key,
	).Scan(&createID)
	cancelled = err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("probe pending %s create: %w", noun, err)
	}

	for _, entityType := range append([]string{createType}, related...) {
		if _, err := tx.Exec(
			`DELETE FROM pending_change WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
			profileID, entityType, key,
		); err != nil {
			return false, fmt.Errorf("clear superseded %s rows: %w", noun, err)
		}
	}
	if cancelled {
		return true, writeAudit(tx, profileID, createType, key, noun+"-create-cancelled", noun, summary, "", "")
	}

	encoded, _ := json.Marshal(snap)
	if err := upsertPendingChange(tx, profileID, deleteType, key, noun, string(encoded), "1", ""); err != nil {
		return false, err
	}
	return false, writeAudit(tx, profileID, deleteType, key, "delete-"+noun+"-local", noun, string(encoded), "", "")
}

// rewritePendingSets passes the before and after values of every pending row
// of entityType through rewrite. A row rewrite reports as a no-op is deleted,
// and one whose values come back unchanged is left alone.
func rewritePendingSets(tx *sql.Tx, profileID, entityType string, rewrite func(before, after string) (newBefore, newAfter string, noop bool)) error {
	rows, err := tx.Query(
		`SELECT id, before_val, after_val FROM pending_change
		 WHERE profile_id = ? AND entity_type = ?`,
		profileID, entityType)
	if err != nil {
		return fmt.Errorf("read %s rows: %w", entityType, err)
	}
	type row struct {
		id            int64
		before, after string
	}
	var rs []row
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.id, &x.before, &x.after); err != nil {
			rows.Close()
			return err
		}
		rs = append(rs, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, x := range rs {
		before, after, noop := rewrite(x.before, x.after)
		switch {
		case noop:
			if _, err := tx.Exec(
				`DELETE FROM pending_change WHERE profile_id = ? AND id = ?`, profileID, x.id,
			); err != nil {
				return fmt.Errorf("drop no-op %s row: %w", entityType, err)
			}
		case before != x.before || after != x.after:
			if _, err := tx.Exec(
				`UPDATE pending_change SET before_val = ?, after_val = ? WHERE profile_id = ? AND id = ?`,
				before, after, profileID, x.id,
			); err != nil {
				return fmt.Errorf("rewrite %s row: %w", entityType, err)
			}
		}
	}
	return nil
}

// encodeKeys is the JSON form of a key list, as the pending key sets store it.
func encodeKeys(keys []string) string {
	encoded, _ := json.Marshal(keys)
	return string(encoded)
}
