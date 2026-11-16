package testrepo

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// rewriteReqReqLinkSets rewrites a requirement key in every pending
// req_req_link_set row: the row's own key and its target list. newKey == ""
// removes the rows keyed by it and drops it from other rows' targets. A row
// whose targets end up equal to its before snapshot is removed.
func rewriteReqReqLinkSets(tx *sql.Tx, profileID, oldKey, newKey string) error {
	var err error
	if newKey == "" {
		_, err = tx.Exec(
			`DELETE FROM pending_change WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
			profileID, entityReqReqLinkSet, oldKey)
	} else {
		_, err = tx.Exec(
			`UPDATE pending_change SET entity_key = ?
			 WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
			newKey, profileID, entityReqReqLinkSet, oldKey)
	}
	if err != nil {
		return fmt.Errorf("rewrite req-link rows: %w", err)
	}

	rows, err := tx.Query(
		`SELECT id, before_val, after_val FROM pending_change
		 WHERE profile_id = ? AND entity_type = ?`,
		profileID, entityReqReqLinkSet)
	if err != nil {
		return fmt.Errorf("read req-link sets: %w", err)
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
		old := decodeKeys(x.after)
		after := substituteKey(old, oldKey, newKey)
		if equalOrder(old, after) {
			continue
		}
		var snap []reqqLinkSnap
		_ = json.Unmarshal([]byte(x.before), &snap)
		before := make([]string, len(snap))
		for i, s := range snap {
			before[i] = s.ToKey
		}
		if equalOrder(before, after) {
			if _, err := tx.Exec(
				`DELETE FROM pending_change WHERE profile_id = ? AND id = ?`, profileID, x.id,
			); err != nil {
				return fmt.Errorf("drop no-op req-link set: %w", err)
			}
			continue
		}
		aj, _ := json.Marshal(after)
		if _, err := tx.Exec(
			`UPDATE pending_change SET after_val = ? WHERE profile_id = ? AND id = ?`,
			string(aj), profileID, x.id,
		); err != nil {
			return fmt.Errorf("rewrite req-link set: %w", err)
		}
	}
	return nil
}

// cancelRequirementCreate handles the delete of a requirement that was only
// ever created locally: it drops the pending create and every edit or link row
// queued against the placeholder key, so nothing about it reaches Jira. It
// reports false, having changed nothing, when the requirement has no pending
// create.
func cancelRequirementCreate(tx *sql.Tx, profileID, key, summary string) (bool, error) {
	var createID int64
	err := tx.QueryRow(
		`SELECT id FROM pending_change
		 WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
		profileID, entityRequirementCreate, key,
	).Scan(&createID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("probe pending requirement create: %w", err)
	}

	if _, err := tx.Exec(
		`DELETE FROM pending_change
		 WHERE profile_id = ? AND entity_key = ? AND entity_type IN (?, ?)`,
		profileID, key, entityRequirementCreate, entityRequirementEdit,
	); err != nil {
		return false, fmt.Errorf("cancel requirement create rows: %w", err)
	}
	if err := rewriteReqReqLinkSets(tx, profileID, key, ""); err != nil {
		return false, err
	}
	if _, err := tx.Exec(
		`DELETE FROM requirement_link
		 WHERE profile_id = ? AND (from_requirement_key = ? OR to_requirement_key = ?)`,
		profileID, key, key,
	); err != nil {
		return false, fmt.Errorf("delete local requirement links: %w", err)
	}
	if err := writeAudit(
		tx, profileID, entityRequirementCreate, key, "requirement-create-cancelled",
		"requirement", summary, "", "",
	); err != nil {
		return false, err
	}
	return true, nil
}

// rewriteRequirementSets rewrites a requirement key in every pending
// requirement_set row (before snapshot objects + after key list). newKey == ""
// drops the key. A row whose before and after end up equal is removed.
func rewriteRequirementSets(tx *sql.Tx, profileID, oldKey, newKey string) error {
	rows, err := tx.Query(
		`SELECT id, before_val, after_val FROM pending_change
		 WHERE profile_id = ? AND entity_type = ?`,
		profileID, entityRequirementSet)
	if err != nil {
		return fmt.Errorf("read requirement sets: %w", err)
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
		before := substituteLinkSnaps(x.before, oldKey, newKey)
		after := substituteKey(decodeKeys(x.after), oldKey, newKey)
		// A rewrite that leaves the covered set unchanged is a no-op coverage
		// edit; drop it rather than leave a stale row in the pending list.
		if sameReqSet(before, after) {
			if _, err := tx.Exec(
				`DELETE FROM pending_change WHERE profile_id = ? AND id = ?`,
				profileID, x.id,
			); err != nil {
				return fmt.Errorf("drop no-op requirement set: %w", err)
			}
			continue
		}
		bj, _ := json.Marshal(before)
		aj, _ := json.Marshal(after)
		if _, err := tx.Exec(
			`UPDATE pending_change SET before_val = ?, after_val = ? WHERE profile_id = ? AND id = ?`,
			string(bj), string(aj), profileID, x.id,
		); err != nil {
			return fmt.Errorf("rewrite requirement set: %w", err)
		}
	}
	return nil
}

// sameReqSet reports whether a requirement_set's before snapshot (links) and
// after key list cover the same requirement keys, i.e. the edit is now a no-op.
func sameReqSet(before []reqLinkSnap, after []string) bool {
	if len(before) != len(after) {
		return false
	}
	keys := make(map[string]int, len(before))
	for _, s := range before {
		keys[s.Key]++
	}
	for _, k := range after {
		if keys[k] == 0 {
			return false
		}
		keys[k]--
	}
	return true
}

// substituteLinkSnaps rewrites or drops a requirement key inside a serialised
// []reqLinkSnap (the requirement_set before snapshot).
func substituteLinkSnaps(beforeJSON, oldKey, newKey string) []reqLinkSnap {
	var snaps []reqLinkSnap
	_ = json.Unmarshal([]byte(beforeJSON), &snaps)
	out := make([]reqLinkSnap, 0, len(snaps))
	for _, s := range snaps {
		if s.Key == oldKey {
			if newKey == "" {
				continue
			}
			s.Key = newKey
		}
		out = append(out, s)
	}
	return out
}
