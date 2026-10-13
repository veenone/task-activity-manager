package testrepo

import (
	"fmt"
	"strings"
)

// Jira has no label admin API: a label exists only as a value on issues. So
// renaming, merging and deleting a label are each a BulkEditLabels over the
// cached tests that carry it, one pending change per test, and they reach
// only tests this profile has synced.

// ListTestsWithLabel returns the keys of the tests carrying label, matched
// whole and case-sensitively, as Jira matches labels.
func (r *Repository) ListTestsWithLabel(profileID, label string) ([]string, error) {
	rows, err := r.db.Query(
		`SELECT jira_key, labels FROM test_case
		 WHERE profile_id = ? AND instr(labels, ?) > 0 ORDER BY jira_key`,
		profileID, label)
	if err != nil {
		return nil, fmt.Errorf("tests with label: %w", err)
	}
	defer rows.Close()
	keys := []string{}
	for rows.Next() {
		var key, stored string
		if err := rows.Scan(&key, &stored); err != nil {
			return nil, err
		}
		if hasLabel(stored, label) {
			keys = append(keys, key)
		}
	}
	return keys, rows.Err()
}

// LabelEditsPending counts the pending label edits on tests that carry label,
// before or after the edit. Mirrors ComponentEditsPending.
func (r *Repository) LabelEditsPending(profileID, label string) (int, error) {
	rows, err := r.db.Query(`
		SELECT COALESCE(tc.labels, ''), COALESCE(pc.before_val, ''), COALESCE(pc.after_val, '')
		FROM pending_change pc
		LEFT JOIN test_case tc ON tc.profile_id = pc.profile_id AND tc.jira_key = pc.entity_key
		WHERE pc.profile_id = ? AND pc.entity_type = ? AND pc.field = 'labels'`,
		profileID, entityTestCase)
	if err != nil {
		return 0, fmt.Errorf("pending label edits: %w", err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var current, before, after string
		if err := rows.Scan(&current, &before, &after); err != nil {
			return 0, err
		}
		if hasLabel(current, label) || hasLabel(before, label) || hasLabel(after, label) {
			n++
		}
	}
	return n, rows.Err()
}

// RenameLabel queues adding newLabel and removing oldLabel on every test that
// carries oldLabel. When newLabel already exists this is a merge, and a test
// carrying both ends with one copy.
func (r *Repository) RenameLabel(profileID, oldLabel, newLabel string) (BulkEditResult, error) {
	empty := BulkEditResult{Succeeded: []string{}, Failed: []BulkFailure{}}
	if err := validateLabel(newLabel); err != nil {
		return empty, err
	}
	if newLabel == oldLabel {
		return empty, fmt.Errorf("label %q already has that name", oldLabel)
	}
	keys, err := r.labelTargets(profileID, oldLabel)
	if err != nil {
		return empty, err
	}
	return r.BulkEditLabels(profileID, keys, []string{newLabel}, []string{oldLabel})
}

// DeleteLabel queues removing label from every test that carries it.
func (r *Repository) DeleteLabel(profileID, label string) (BulkEditResult, error) {
	keys, err := r.labelTargets(profileID, label)
	if err != nil {
		return BulkEditResult{Succeeded: []string{}, Failed: []BulkFailure{}}, err
	}
	return r.BulkEditLabels(profileID, keys, nil, []string{label})
}

// labelTargets refuses while pending label edits name label, since those
// edits were made against labels this change would rewrite, then returns
// the tests carrying it.
func (r *Repository) labelTargets(profileID, label string) ([]string, error) {
	n, err := r.LabelEditsPending(profileID, label)
	if err != nil {
		return nil, err
	}
	if n > 0 {
		noun := "edit"
		if n > 1 {
			noun = "edits"
		}
		return nil, fmt.Errorf("Commit or discard the %d pending label %s on tests that use %q first.", n, noun, label)
	}
	return r.ListTestsWithLabel(profileID, label)
}

func hasLabel(stored, label string) bool {
	for _, l := range strings.Fields(stored) {
		if l == label {
			return true
		}
	}
	return false
}
