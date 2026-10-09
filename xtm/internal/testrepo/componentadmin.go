package testrepo

import "fmt"

// ComponentEditsPending counts pending components edits on tests that carry
// name, either in the synced value or in the queued one. A rename or delete
// of that component would leave those edits naming a component Jira no
// longer has, so the Components view refuses while any exist.
func (r *Repository) ComponentEditsPending(profileID, name string) (int, error) {
	pattern := componentFilterPattern(name)
	var n int
	err := r.db.QueryRow(`
		SELECT COUNT(*) FROM pending_change pc
		LEFT JOIN test_case tc ON tc.profile_id = pc.profile_id AND tc.jira_key = pc.entity_key
		WHERE pc.profile_id = ? AND pc.entity_type = ? AND pc.field = 'components'
		  AND (tc.components LIKE ? OR pc.after_val LIKE ?)`,
		profileID, entityTestCase, pattern, pattern,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("pending component edits: %w", err)
	}
	return n, nil
}

// RenameComponentOnTests replaces oldName with newName in every cached test's
// components, after Jira has renamed it. A test that already carries newName
// keeps one copy.
func (r *Repository) RenameComponentOnTests(profileID, oldName, newName string) (int, error) {
	return r.rewriteComponent(profileID, oldName, func(names []string) []string {
		out := make([]string, 0, len(names))
		seen := map[string]bool{}
		for _, n := range names {
			if n == oldName {
				n = newName
			}
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
		return out
	})
}

// RemoveComponentFromTests drops name from every cached test's components,
// after Jira has deleted it.
func (r *Repository) RemoveComponentFromTests(profileID, name string) (int, error) {
	return r.rewriteComponent(profileID, name, func(names []string) []string {
		out := make([]string, 0, len(names))
		for _, n := range names {
			if n != name {
				out = append(out, n)
			}
		}
		return out
	})
}

// rewriteComponent applies fn to the components of every test carrying name,
// in one transaction.
func (r *Repository) rewriteComponent(profileID, name string, fn func([]string) []string) (int, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.Query(
		`SELECT jira_key, components FROM test_case WHERE profile_id = ? AND components LIKE ?`,
		profileID, componentFilterPattern(name))
	if err != nil {
		return 0, fmt.Errorf("find tests with component %q: %w", name, err)
	}
	type change struct{ key, value string }
	var changes []change
	for rows.Next() {
		var key, stored string
		if err := rows.Scan(&key, &stored); err != nil {
			rows.Close()
			return 0, err
		}
		next := encodeComponents(fn(decodeComponents(stored)))
		if next != stored {
			changes = append(changes, change{key, next})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, c := range changes {
		if _, err := tx.Exec(
			`UPDATE test_case SET components = ? WHERE profile_id = ? AND jira_key = ?`,
			c.value, profileID, c.key); err != nil {
			return 0, fmt.Errorf("rewrite components on %s: %w", c.key, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(changes), nil
}
