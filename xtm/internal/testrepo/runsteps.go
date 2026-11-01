// The cache and journal for step results on a plain execution (#371 Phase 6).
//
// This mirrors parameterizedrun.go one level flatter: there is no iteration
// between the execution and the step, and no parameters anywhere. The two
// guarantees that file earned are kept, because they were bugs rather than
// preferences. A refresh upserts and protects a status that has a pending
// change, since deleting and reinserting destroyed a local pick (#364). The
// uncommitted flag is read from the journal rather than stored in a column, so
// the view cannot claim a push Commit would not make (#366).

package testrepo

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"agile-suite/xtm/internal/backend"
)

// runStepResultLimit is a guard against a backend answering with something
// unbounded, matching the iteration limit next door.
const runStepResultLimit = 1000

// ReplaceRunStepResults caches what the backend reports for one test in one
// execution, keeping any status that is journalled for Commit.
func (r *Repository) ReplaceRunStepResults(profileID, execKey, testKey string, steps []backend.RunStepResult) error {
	if err := validateRunStepResults(profileID, execKey, testKey, steps); err != nil {
		return err
	}
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin run step result replacement: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, step := range steps {
		if _, err := tx.Exec(
			`INSERT INTO test_run_step
			 (profile_id, exec_key, test_key, step_result_id, step_definition_id,
			  rank, status, action, data, expected, comment)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(profile_id, exec_key, test_key, step_result_id) DO UPDATE SET
			   step_definition_id = excluded.step_definition_id,
			   rank               = excluded.rank,
			   action             = excluded.action,
			   data               = excluded.data,
			   expected           = excluded.expected,
			   comment            = excluded.comment,
			   status             = CASE WHEN EXISTS (
			       SELECT 1 FROM pending_change
			       WHERE pending_change.profile_id  = ?
			         AND pending_change.entity_type = ?
			         AND pending_change.entity_key  = ?
			         AND pending_change.field       = 'status'
			     ) THEN test_run_step.status ELSE excluded.status END`,
			profileID, execKey, testKey, step.ID, step.DefinitionID,
			step.Rank, step.Status, step.Action, step.Data, step.Expected, step.Comment,
			profileID, entityTestRunStep, runStepEntityKey(execKey, testKey, step.ID),
		); err != nil {
			return fmt.Errorf("store step result %s: %w", step.ID, err)
		}
	}

	// Drop what the backend no longer reports, scoped to this execution: the
	// same test in another cycle has its own results and they are not stale.
	kept := make([]any, 0, len(steps))
	marks := make([]string, 0, len(steps))
	for _, step := range steps {
		kept = append(kept, step.ID)
		marks = append(marks, "?")
	}
	args := append([]any{profileID, execKey, testKey}, kept...)
	if _, err := tx.Exec(
		`DELETE FROM test_run_step
		 WHERE profile_id = ? AND exec_key = ? AND test_key = ?
		   AND step_result_id NOT IN (`+strings.Join(marks, ",")+`)`, args...,
	); err != nil {
		return fmt.Errorf("drop step results the backend no longer reports: %w", err)
	}
	return tx.Commit()
}

// RunStepResults reads the cached step results of one test's execution, in
// step order. found is false when nothing has been cached for it, which is how
// the app layer knows to ask the backend.
func (r *Repository) RunStepResults(profileID, execKey, testKey string) ([]backend.RunStepResult, bool, error) {
	rows, err := r.db.Query(
		`SELECT step_result_id, step_definition_id, rank, status, action, data, expected, comment,
		        EXISTS (
		          SELECT 1 FROM pending_change
		          WHERE pending_change.profile_id  = ?
		            AND pending_change.entity_type = 'test_run_step'
		            AND pending_change.entity_key  = ? || ':' || ? || ':' || step_result_id
		            AND pending_change.field       = 'status'
		        )
		 FROM test_run_step
		 WHERE profile_id = ? AND exec_key = ? AND test_key = ?
		 ORDER BY rank, step_result_id`,
		profileID, execKey, testKey, profileID, execKey, testKey,
	)
	if err != nil {
		return nil, false, fmt.Errorf("query run step results: %w", err)
	}
	defer rows.Close()
	steps := []backend.RunStepResult{}
	for rows.Next() {
		var step backend.RunStepResult
		if err := rows.Scan(&step.ID, &step.DefinitionID, &step.Rank, &step.Status,
			&step.Action, &step.Data, &step.Expected, &step.Comment, &step.Uncommitted); err != nil {
			return nil, false, fmt.Errorf("scan run step result: %w", err)
		}
		steps = append(steps, step)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	return steps, len(steps) > 0, nil
}

// SetRunStepStatus records a result locally and journals it for Commit. The
// write to the backend happens there, as it does for every other edit: the
// store is a cache plus a journal and Jira is the system of record.
func (r *Repository) SetRunStepStatus(profileID, execKey, testKey, stepResultID, status string) error {
	if strings.TrimSpace(status) == "" {
		return fmt.Errorf("a step result status is required")
	}
	if strings.TrimSpace(stepResultID) == "" {
		return fmt.Errorf("a step result id is required")
	}
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin step result status edit: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var current string
	if err := tx.QueryRow(
		`SELECT status FROM test_run_step
		 WHERE profile_id = ? AND exec_key = ? AND test_key = ? AND step_result_id = ?`,
		profileID, execKey, testKey, stepResultID,
	).Scan(&current); errors.Is(err, sql.ErrNoRows) {
		// Journalling this anyway would queue a row keyed by an id the backend
		// never reported, which no Commit could ever push and no resolution
		// could clear.
		return fmt.Errorf("step result %s does not belong to %s in %s", stepResultID, testKey, execKey)
	} else if err != nil {
		return fmt.Errorf("read step result status: %w", err)
	}
	if current == status {
		return nil
	}

	entityKey := runStepEntityKey(execKey, testKey, stepResultID)
	if _, err := tx.Exec(
		`UPDATE test_run_step SET status = ?
		 WHERE profile_id = ? AND exec_key = ? AND test_key = ? AND step_result_id = ?`,
		status, profileID, execKey, testKey, stepResultID,
	); err != nil {
		return fmt.Errorf("update step result status: %w", err)
	}
	if err := upsertPendingChange(tx, profileID, entityTestRunStep, entityKey, "status", current, status, ""); err != nil {
		return err
	}
	if err := writeAudit(tx, profileID, entityTestRunStep, entityKey, "edit-status-local", "status", current, status, ""); err != nil {
		return err
	}
	return tx.Commit()
}

// runStepEntityKey is the pending-change key for one step result. It is built
// in one place because the store writes it, the refresh above reads it back to
// protect a local edit, and the commit path parses it. Issue keys contain no
// colon, so the three parts are unambiguous.
func runStepEntityKey(execKey, testKey, stepResultID string) string {
	return strings.Join([]string{execKey, testKey, stepResultID}, ":")
}

func validateRunStepResults(profileID, execKey, testKey string, steps []backend.RunStepResult) error {
	if strings.TrimSpace(profileID) == "" || strings.TrimSpace(execKey) == "" || strings.TrimSpace(testKey) == "" {
		return fmt.Errorf("profile, execution and test are required")
	}
	if len(steps) == 0 || len(steps) > runStepResultLimit {
		return fmt.Errorf("a run's step results must number 1 to %d", runStepResultLimit)
	}
	seen := make(map[string]struct{}, len(steps))
	for _, step := range steps {
		if strings.TrimSpace(step.ID) == "" || strings.TrimSpace(step.Status) == "" {
			return fmt.Errorf("a step result needs an id and a status")
		}
		if _, duplicate := seen[step.ID]; duplicate {
			return fmt.Errorf("duplicate step result id %q", step.ID)
		}
		seen[step.ID] = struct{}{}
	}
	return nil
}
