package testrepo

import (
	"database/sql"
	"fmt"
)

// The helpers here let a sync writer respect the pending journal: a change
// that is still waiting for commit wins over what the sync just read.

// pendingKeys returns the entity keys that have a pending change of the given
// type.
func pendingKeys(tx *sql.Tx, profileID, entityType string) (map[string]bool, error) {
	rows, err := tx.Query(
		`SELECT DISTINCT entity_key FROM pending_change
		 WHERE profile_id = ? AND entity_type = ?`,
		profileID, entityType)
	if err != nil {
		return nil, fmt.Errorf("read pending %s keys: %w", entityType, err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		out[key] = true
	}
	return out, rows.Err()
}

// reapplyFieldEdits writes every pending field edit of entityType back onto
// its column in table, which is keyed by (profile_id, jira_key). fields maps
// a pending row's field name to the column it edits.
func reapplyFieldEdits(tx *sql.Tx, profileID, table, entityType string, fields map[string]string) error {
	for field, col := range fields {
		if _, err := tx.Exec(
			fmt.Sprintf(
				`UPDATE %[1]s SET %[2]s = (
				   SELECT p.after_val FROM pending_change p
				   WHERE p.profile_id = %[1]s.profile_id AND p.entity_type = ?
				     AND p.entity_key = %[1]s.jira_key AND p.field = ?)
				 WHERE profile_id = ? AND jira_key IN (
				   SELECT entity_key FROM pending_change
				   WHERE profile_id = ? AND entity_type = ? AND field = ?)`, table, col),
			entityType, field, profileID, profileID, entityType, field,
		); err != nil {
			return fmt.Errorf("reapply pending %s %s edits: %w", entityType, field, err)
		}
	}
	return nil
}
