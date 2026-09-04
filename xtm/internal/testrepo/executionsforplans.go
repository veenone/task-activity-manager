package testrepo

import "fmt"

// ExecutionsForPlans returns the Test Executions that share at least one Test
// with the given Test Plans, ordered by key. An empty planKeys returns every
// Execution (same as ListContainers(profileID, "testexec")). Used to cascade the
// dashboard's Execution filter from the selected Plan(s) (FR-9, #5a).
func (r *Repository) ExecutionsForPlans(profileID string, planKeys []string) ([]Container, error) {
	keys := nonEmptyKeys(planKeys)
	if len(keys) == 0 {
		return r.ListContainers(profileID, "testexec")
	}

	q := `SELECT DISTINCT c.jira_key, c.kind, c.summary, c.status
	      FROM test_container c
	      JOIN test_container_test e
	        ON e.profile_id = c.profile_id AND e.container_key = c.jira_key
	      WHERE c.profile_id = ? AND c.kind = 'testexec'
	        AND e.test_key IN (
	          SELECT test_key FROM test_container_test
	          WHERE profile_id = ? AND container_key IN (` + sqlPlaceholders(len(keys)) + `))
	      ORDER BY c.jira_key`
	args := []any{profileID, profileID}
	for _, k := range keys {
		args = append(args, k)
	}

	rows, err := r.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("executions for plans: %w", err)
	}
	defer rows.Close()
	out := []Container{}
	for rows.Next() {
		var c Container
		if err := rows.Scan(&c.Key, &c.Kind, &c.Summary, &c.Status); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
