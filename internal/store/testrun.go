package store

import "fmt"

// TestRunRow holds the per-execution run details for a single Test, as stored
// in the test_run table. Defects is a JSON array string (e.g. `["PROJ-1"]`).
type TestRunRow struct {
	ExecKey     string
	TestKey     string
	RunStatus   string
	StartedAt   string
	FinishedAt  string
	ExecutedBy  string
	Environment string
	Defects     string
}

// ReplaceRunsForExec atomically replaces all test_run rows for the given
// execution key with the supplied slice. It deletes existing rows for
// (profileID, execKey) and then inserts the new ones in a single transaction.
func (s *Store) ReplaceRunsForExec(profileID, execKey string, runs []TestRunRow) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(
		`DELETE FROM test_run WHERE profile_id = ? AND exec_key = ?`,
		profileID, execKey,
	); err != nil {
		return fmt.Errorf("clear test runs: %w", err)
	}
	for _, r := range runs {
		if _, err := tx.Exec(
			`INSERT INTO test_run
			  (profile_id, exec_key, test_key, run_status, started_at, finished_at, executed_by, environment, defects)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			profileID, r.ExecKey, r.TestKey, r.RunStatus,
			r.StartedAt, r.FinishedAt, r.ExecutedBy, r.Environment, r.Defects,
		); err != nil {
			return fmt.Errorf("insert test run: %w", err)
		}
	}
	return tx.Commit()
}

// UpsertExecPlan records that the given Test Execution belongs to the given
// Test Plan. It is a no-op if the association already exists.
func (s *Store) UpsertExecPlan(profileID, execKey, planKey string) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO exec_plan (profile_id, exec_key, plan_key) VALUES (?, ?, ?)`,
		profileID, execKey, planKey,
	)
	return err
}

// ReplaceExecPlans atomically replaces all exec_plan rows for the given
// execution key with the supplied plan keys. It deletes existing rows for
// (profileID, execKey) and then inserts the new ones in a single transaction.
func (s *Store) ReplaceExecPlans(profileID, execKey string, planKeys []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(
		`DELETE FROM exec_plan WHERE profile_id = ? AND exec_key = ?`,
		profileID, execKey,
	); err != nil {
		return fmt.Errorf("clear exec plans: %w", err)
	}
	for _, pk := range planKeys {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO exec_plan (profile_id, exec_key, plan_key) VALUES (?, ?, ?)`,
			profileID, execKey, pk,
		); err != nil {
			return fmt.Errorf("insert exec plan: %w", err)
		}
	}
	return tx.Commit()
}

// RunsForTest returns all test_run rows for the given test key, ordered by
// finished_at descending then exec_key, so the most recent run appears first.
func (s *Store) RunsForTest(profileID, testKey string) ([]TestRunRow, error) {
	rows, err := s.db.Query(
		`SELECT exec_key, test_key, run_status, started_at, finished_at, executed_by, environment, defects
		 FROM test_run
		 WHERE profile_id = ? AND test_key = ?
		 ORDER BY finished_at DESC, exec_key`,
		profileID, testKey,
	)
	if err != nil {
		return nil, fmt.Errorf("query test runs: %w", err)
	}
	defer rows.Close()
	var out []TestRunRow
	for rows.Next() {
		var r TestRunRow
		if err := rows.Scan(
			&r.ExecKey, &r.TestKey, &r.RunStatus,
			&r.StartedAt, &r.FinishedAt, &r.ExecutedBy, &r.Environment, &r.Defects,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
