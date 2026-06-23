package testrepo

// TestRunEntry is one execution-run of a test, with the execution's context.
type TestRunEntry struct {
	ExecKey     string   `json:"execKey"`
	ExecSummary string   `json:"execSummary"`
	PlanKeys    []string `json:"planKeys"`
	Environment string   `json:"environment"`
	FixVersions []string `json:"fixVersions"`
	RunStatus   string   `json:"runStatus"`
	StartedAt   string   `json:"startedAt"`
	FinishedAt  string   `json:"finishedAt"`
	ExecutedBy  string   `json:"executedBy"`
	Defects     []string `json:"defects"`
}

// GetTestRunHistory returns every execution-run of a test, newest finished_at
// first (then exec_key for a stable secondary order), enriched with the
// execution summary, fix versions, and associated Test Plans from the local
// cache.
func (r *Repository) GetTestRunHistory(profileID, testKey string) ([]TestRunEntry, error) {
	rows, err := r.db.Query(`
		SELECT tr.exec_key,
		       COALESCE(c.summary, ''),
		       tr.run_status,
		       tr.started_at,
		       tr.finished_at,
		       tr.executed_by,
		       tr.environment,
		       tr.defects,
		       COALESCE(c.fix_versions, '')
		FROM test_run tr
		LEFT JOIN test_container c
		       ON c.profile_id = tr.profile_id AND c.jira_key = tr.exec_key
		WHERE tr.profile_id = ? AND tr.test_key = ?
		ORDER BY tr.finished_at DESC, tr.exec_key`,
		profileID, testKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TestRunEntry
	for rows.Next() {
		var e TestRunEntry
		var defectsJSON, fixJSON string
		if err := rows.Scan(
			&e.ExecKey, &e.ExecSummary, &e.RunStatus,
			&e.StartedAt, &e.FinishedAt, &e.ExecutedBy, &e.Environment,
			&defectsJSON, &fixJSON,
		); err != nil {
			return nil, err
		}
		// decodeFixVersions reuses decodeEnvironments: returns [] for "" or malformed JSON.
		e.Defects = decodeFixVersions(defectsJSON)
		e.FixVersions = decodeFixVersions(fixJSON)
		// ExecPlansForExec is already defined in testrun.go; reuse it.
		plans, _ := r.ExecPlansForExec(profileID, e.ExecKey)
		if plans == nil {
			plans = []string{}
		}
		e.PlanKeys = plans
		out = append(out, e)
	}
	return out, rows.Err()
}
