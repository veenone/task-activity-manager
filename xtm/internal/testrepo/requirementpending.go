package testrepo

import (
	"database/sql"
	"encoding/json"
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

	return rewritePendingSets(tx, profileID, entityReqReqLinkSet, func(beforeJSON, afterJSON string) (string, string, bool) {
		old := decodeKeys(afterJSON)
		after := substituteKey(old, oldKey, newKey)
		if equalOrder(old, after) {
			return beforeJSON, afterJSON, false
		}
		var snap []reqqLinkSnap
		_ = json.Unmarshal([]byte(beforeJSON), &snap)
		before := make([]string, len(snap))
		for i, s := range snap {
			before[i] = s.ToKey
		}
		return beforeJSON, encodeKeys(after), equalOrder(before, after)
	})
}

// dropLocalRequirementLinks removes the requirement-to-requirement links of a
// requirement that was only created locally, cached and pending.
func dropLocalRequirementLinks(tx *sql.Tx, profileID, key string) error {
	if err := rewriteReqReqLinkSets(tx, profileID, key, ""); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`DELETE FROM requirement_link
		 WHERE profile_id = ? AND (from_requirement_key = ? OR to_requirement_key = ?)`,
		profileID, key, key,
	); err != nil {
		return fmt.Errorf("delete local requirement links: %w", err)
	}
	return nil
}

// rewriteRequirementSets rewrites a requirement key in every pending
// requirement_set row (before snapshot objects + after key list). newKey == ""
// drops the key. A row whose before and after end up equal is removed.
func rewriteRequirementSets(tx *sql.Tx, profileID, oldKey, newKey string) error {
	return rewritePendingSets(tx, profileID, entityRequirementSet, func(beforeJSON, afterJSON string) (string, string, bool) {
		before := substituteLinkSnaps(beforeJSON, oldKey, newKey)
		after := substituteKey(decodeKeys(afterJSON), oldKey, newKey)
		// A rewrite that leaves the covered set unchanged is a no-op coverage
		// edit; drop it rather than leave a stale row in the pending list.
		bj, _ := json.Marshal(before)
		return string(bj), encodeKeys(after), sameReqSet(before, after)
	})
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
