package testrepo

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"agile-suite/xtm/internal/backend"
)

const parameterizedRunLimit = 1000

func (r *Repository) ReplaceParameterizedRun(profileID string, run backend.ParameterizedRun) error {
	if err := validateParameterizedRun(run); err != nil {
		return err
	}
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin parameterized run replacement: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// This refresh used to delete the whole tree and reinsert it, which threw
	// away any result that had been set locally and not yet committed (#364).
	// It upserts instead, and keeps a status that has a pending change, the way
	// UpsertTests guards every editable field of a test against a sync.
	for _, iteration := range run.Iterations {
		parametersJSON, err := json.Marshal(iteration.Parameters)
		if err != nil {
			return fmt.Errorf("encode iteration %s parameters: %w", iteration.ID, err)
		}
		if _, err := tx.Exec(
			`INSERT INTO test_run_iteration
			 (profile_id, exec_key, test_key, iteration_id, run_id, rank, status, run_status,
			  dataset_source_kind, dataset_source_key, parameters_json)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(profile_id, exec_key, test_key, iteration_id) DO UPDATE SET
			   run_id              = excluded.run_id,
			   rank                = excluded.rank,
			   run_status          = excluded.run_status,
			   dataset_source_kind = excluded.dataset_source_kind,
			   dataset_source_key  = excluded.dataset_source_key,
			   parameters_json     = excluded.parameters_json,
			   status              = CASE WHEN EXISTS (
			       SELECT 1 FROM pending_change
			       WHERE pending_change.profile_id  = ?
			         AND pending_change.entity_type = ?
			         AND pending_change.entity_key  = ?
			         AND pending_change.field       = 'status'
			     ) THEN test_run_iteration.status ELSE excluded.status END`,
			profileID, run.ExecutionKey, run.TestKey, iteration.ID, run.ID, iteration.Rank,
			iteration.Status, run.Status, run.DatasetSourceKind, run.DatasetSourceKey, string(parametersJSON),
			profileID, entityTestRunIteration, iterationEntityKey(run.ExecutionKey, run.TestKey, iteration.ID),
		); err != nil {
			return fmt.Errorf("store iteration %s: %w", iteration.ID, err)
		}
		for _, step := range iteration.Steps {
			if _, err := tx.Exec(
				`INSERT INTO test_run_iteration_step
				 (profile_id, exec_key, test_key, iteration_id, step_result_id, step_definition_id,
				  rank, status, action, data, expected)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				 ON CONFLICT(profile_id, exec_key, test_key, iteration_id, step_result_id) DO UPDATE SET
				   step_definition_id = excluded.step_definition_id,
				   rank               = excluded.rank,
				   action             = excluded.action,
				   data               = excluded.data,
				   expected           = excluded.expected,
				   status             = CASE WHEN EXISTS (
				       SELECT 1 FROM pending_change
				       WHERE pending_change.profile_id  = ?
				         AND pending_change.entity_type = ?
				         AND pending_change.entity_key  = ?
				         AND pending_change.field       = 'status'
				     ) THEN test_run_iteration_step.status ELSE excluded.status END`,
				profileID, run.ExecutionKey, run.TestKey, iteration.ID, step.ID, step.DefinitionID,
				step.Rank, step.Status, step.Action, step.Data, step.Expected,
				profileID, entityTestRunIterStep, iterationStepEntityKey(run.ExecutionKey, run.TestKey, iteration.ID, step.ID),
			); err != nil {
				return fmt.Errorf("store iteration step %s: %w", step.ID, err)
			}
		}
	}
	if err := pruneParameterizedRun(tx, profileID, run); err != nil {
		return err
	}
	return tx.Commit()
}

// pruneParameterizedRun drops what the backend no longer reports, so a dataset
// that lost a row does not leave an iteration behind that nothing can commit.
func pruneParameterizedRun(tx *sql.Tx, profileID string, run backend.ParameterizedRun) error {
	iterationIDs := make([]any, 0, len(run.Iterations))
	marks := make([]string, 0, len(run.Iterations))
	for _, iteration := range run.Iterations {
		iterationIDs = append(iterationIDs, iteration.ID)
		marks = append(marks, "?")
	}
	kept := "(" + strings.Join(marks, ",") + ")"

	args := append([]any{profileID, run.ExecutionKey, run.TestKey}, iterationIDs...)
	if _, err := tx.Exec(
		`DELETE FROM test_run_iteration
		 WHERE profile_id = ? AND exec_key = ? AND test_key = ? AND iteration_id NOT IN `+kept, args...,
	); err != nil {
		return fmt.Errorf("drop iterations the backend no longer reports: %w", err)
	}
	if _, err := tx.Exec(
		`DELETE FROM test_run_iteration_step
		 WHERE profile_id = ? AND exec_key = ? AND test_key = ? AND iteration_id NOT IN `+kept, args...,
	); err != nil {
		return fmt.Errorf("drop steps of iterations the backend no longer reports: %w", err)
	}
	for _, iteration := range run.Iterations {
		stepIDs := make([]any, 0, len(iteration.Steps))
		stepMarks := make([]string, 0, len(iteration.Steps))
		for _, step := range iteration.Steps {
			stepIDs = append(stepIDs, step.ID)
			stepMarks = append(stepMarks, "?")
		}
		stepArgs := append([]any{profileID, run.ExecutionKey, run.TestKey, iteration.ID}, stepIDs...)
		query := `DELETE FROM test_run_iteration_step
		          WHERE profile_id = ? AND exec_key = ? AND test_key = ? AND iteration_id = ?`
		if len(stepMarks) > 0 {
			query += ` AND step_result_id NOT IN (` + strings.Join(stepMarks, ",") + `)`
		}
		if _, err := tx.Exec(query, stepArgs...); err != nil {
			return fmt.Errorf("drop steps iteration %s no longer has: %w", iteration.ID, err)
		}
	}
	return nil
}

// The pending-change entity keys for a parameterized result. They are built in
// one place because the store writes them, the refresh above reads them back to
// protect a local edit, and the commit path parses them.
func iterationEntityKey(execKey, testKey, iterationID string) string {
	return strings.Join([]string{execKey, testKey, iterationID}, ":")
}

func iterationStepEntityKey(execKey, testKey, iterationID, stepResultID string) string {
	return strings.Join([]string{execKey, testKey, iterationID, stepResultID}, ":")
}

func (r *Repository) ParameterizedRun(profileID, execKey, testKey string) (backend.ParameterizedRun, bool, error) {
	// The uncommitted flag comes from the journal rather than a column, so the
	// store cannot drift from what Commit would actually push (#366).
	rows, err := r.db.Query(
		`SELECT iteration_id, run_id, rank, status, run_status, dataset_source_kind,
		        dataset_source_key, parameters_json,
		        EXISTS (
		          SELECT 1 FROM pending_change
		          WHERE pending_change.profile_id  = ?
		            AND pending_change.entity_type = 'test_run_iteration'
		            AND pending_change.entity_key  = ? || ':' || ? || ':' || iteration_id
		            AND pending_change.field       = 'status'
		        )
		 FROM test_run_iteration
		 WHERE profile_id = ? AND exec_key = ? AND test_key = ?
		 ORDER BY rank`,
		profileID, execKey, testKey, profileID, execKey, testKey,
	)
	if err != nil {
		return backend.ParameterizedRun{}, false, fmt.Errorf("query parameterized run: %w", err)
	}
	defer rows.Close()
	run := backend.ParameterizedRun{ExecutionKey: execKey, TestKey: testKey, Iterations: []backend.RunIteration{}}
	for rows.Next() {
		var iteration backend.RunIteration
		var parametersJSON string
		if err := rows.Scan(
			&iteration.ID, &run.ID, &iteration.Rank, &iteration.Status, &run.Status,
			&run.DatasetSourceKind, &run.DatasetSourceKey, &parametersJSON, &iteration.Uncommitted,
		); err != nil {
			return backend.ParameterizedRun{}, false, fmt.Errorf("scan parameterized run: %w", err)
		}
		iteration.Parameters = []backend.IterationParameter{}
		if err := json.Unmarshal([]byte(parametersJSON), &iteration.Parameters); err != nil {
			return backend.ParameterizedRun{}, false, fmt.Errorf("decode iteration %s parameters: %w", iteration.ID, err)
		}
		steps, err := r.iterationSteps(profileID, execKey, testKey, iteration.ID)
		if err != nil {
			return backend.ParameterizedRun{}, false, err
		}
		iteration.Steps = steps
		run.Iterations = append(run.Iterations, iteration)
	}
	if err := rows.Err(); err != nil {
		return backend.ParameterizedRun{}, false, err
	}
	return run, len(run.Iterations) > 0, nil
}

func (r *Repository) iterationSteps(profileID, execKey, testKey, iterationID string) ([]backend.IterationStep, error) {
	rows, err := r.db.Query(
		`SELECT step_result_id, step_definition_id, rank, status, action, data, expected,
		        EXISTS (
		          SELECT 1 FROM pending_change
		          WHERE pending_change.profile_id  = ?
		            AND pending_change.entity_type = 'test_run_iteration_step'
		            AND pending_change.entity_key  = ? || ':' || ? || ':' || ? || ':' || step_result_id
		            AND pending_change.field       = 'status'
		        )
		 FROM test_run_iteration_step
		 WHERE profile_id = ? AND exec_key = ? AND test_key = ? AND iteration_id = ?
		 ORDER BY rank`,
		profileID, execKey, testKey, iterationID, profileID, execKey, testKey, iterationID,
	)
	if err != nil {
		return nil, fmt.Errorf("query iteration steps: %w", err)
	}
	defer rows.Close()
	steps := []backend.IterationStep{}
	for rows.Next() {
		var step backend.IterationStep
		if err := rows.Scan(&step.ID, &step.DefinitionID, &step.Rank, &step.Status, &step.Action,
			&step.Data, &step.Expected, &step.Uncommitted); err != nil {
			return nil, fmt.Errorf("scan iteration step: %w", err)
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func (r *Repository) SetIterationStatus(profileID, execKey, testKey, iterationID, status string) error {
	return r.setParameterizedStatus(profileID, execKey, testKey, iterationID, "", status)
}

func (r *Repository) SetIterationStepStatus(profileID, execKey, testKey, iterationID, stepResultID, status string) error {
	return r.setParameterizedStatus(profileID, execKey, testKey, iterationID, stepResultID, status)
}

func (r *Repository) setParameterizedStatus(profileID, execKey, testKey, iterationID, stepResultID, status string) error {
	if strings.TrimSpace(status) == "" {
		return fmt.Errorf("iteration status is required")
	}
	table := "test_run_iteration"
	entityType := entityTestRunIteration
	entityKey := iterationEntityKey(execKey, testKey, iterationID)
	query := `SELECT status FROM test_run_iteration WHERE profile_id = ? AND exec_key = ? AND test_key = ? AND iteration_id = ?`
	update := `UPDATE test_run_iteration SET status = ? WHERE profile_id = ? AND exec_key = ? AND test_key = ? AND iteration_id = ?`
	selectArgs := []any{profileID, execKey, testKey, iterationID}
	if stepResultID != "" {
		table = "test_run_iteration_step"
		entityType = entityTestRunIterStep
		entityKey = iterationStepEntityKey(execKey, testKey, iterationID, stepResultID)
		query = `SELECT status FROM test_run_iteration_step WHERE profile_id = ? AND exec_key = ? AND test_key = ? AND iteration_id = ? AND step_result_id = ?`
		update = `UPDATE test_run_iteration_step SET status = ? WHERE profile_id = ? AND exec_key = ? AND test_key = ? AND iteration_id = ? AND step_result_id = ?`
		selectArgs = append(selectArgs, stepResultID)
	}
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin %s status edit: %w", table, err)
	}
	defer func() { _ = tx.Rollback() }()
	var current string
	if err := tx.QueryRow(query, selectArgs...).Scan(&current); errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s does not belong to %s/%s", entityKey, execKey, testKey)
	} else if err != nil {
		return fmt.Errorf("read %s status: %w", table, err)
	}
	if current == status {
		return nil
	}
	updateArgs := append([]any{status}, selectArgs...)
	if _, err := tx.Exec(update, updateArgs...); err != nil {
		return fmt.Errorf("update %s status: %w", table, err)
	}
	if err := upsertPendingChange(tx, profileID, entityType, entityKey, "status", current, status, ""); err != nil {
		return err
	}
	if err := writeAudit(tx, profileID, entityType, entityKey, "edit-status-local", "status", current, status, ""); err != nil {
		return err
	}
	return tx.Commit()
}

func validateParameterizedRun(run backend.ParameterizedRun) error {
	if strings.TrimSpace(run.ID) == "" || strings.TrimSpace(run.ExecutionKey) == "" || strings.TrimSpace(run.TestKey) == "" {
		return fmt.Errorf("parameterized run identity is required")
	}
	if run.DatasetSourceKind != backend.DatasetSourceTest && run.DatasetSourceKind != backend.DatasetSourcePlan && run.DatasetSourceKind != backend.DatasetSourceExecution {
		return fmt.Errorf("invalid dataset source kind %q", run.DatasetSourceKind)
	}
	if strings.TrimSpace(run.DatasetSourceKey) == "" || strings.TrimSpace(run.Status) == "" {
		return fmt.Errorf("dataset source key and run status are required")
	}
	if len(run.Iterations) == 0 || len(run.Iterations) > parameterizedRunLimit {
		return fmt.Errorf("parameterized run must contain 1 to %d iterations", parameterizedRunLimit)
	}
	iterationIDs := map[string]struct{}{}
	iterationRanks := map[int]struct{}{}
	var parameterNames []string
	for iterationIndex, iteration := range run.Iterations {
		// A one-row dataset has no iteration id, because Xray gives it none:
		// it reports the run's parameters flat and no iterations at all, and
		// the reader keeps that absence rather than inventing an id a write
		// would then address to nothing (#362). The duplicate check below
		// still catches a second iteration arriving without one.
		if strings.TrimSpace(iteration.Status) == "" {
			return fmt.Errorf("iteration %d status is required", iterationIndex+1)
		}
		if _, duplicate := iterationIDs[iteration.ID]; duplicate {
			return fmt.Errorf("duplicate iteration id %q", iteration.ID)
		}
		if _, duplicate := iterationRanks[iteration.Rank]; duplicate {
			return fmt.Errorf("duplicate iteration rank %d", iteration.Rank)
		}
		iterationIDs[iteration.ID] = struct{}{}
		iterationRanks[iteration.Rank] = struct{}{}
		names := make([]string, len(iteration.Parameters))
		seenNames := map[string]struct{}{}
		for index, parameter := range iteration.Parameters {
			if strings.TrimSpace(parameter.Name) == "" {
				return fmt.Errorf("iteration %s has a blank parameter name", iteration.ID)
			}
			if _, duplicate := seenNames[parameter.Name]; duplicate {
				return fmt.Errorf("iteration %s has duplicate parameter %q", iteration.ID, parameter.Name)
			}
			seenNames[parameter.Name] = struct{}{}
			names[index] = parameter.Name
		}
		if iterationIndex == 0 {
			parameterNames = names
		} else if !equalStringSlices(names, parameterNames) {
			return fmt.Errorf("iteration %s parameters do not match the run", iteration.ID)
		}
		stepIDs := map[string]struct{}{}
		stepRanks := map[int]struct{}{}
		for _, step := range iteration.Steps {
			if strings.TrimSpace(step.ID) == "" || strings.TrimSpace(step.DefinitionID) == "" || strings.TrimSpace(step.Status) == "" {
				return fmt.Errorf("iteration %s has a step with missing identity or status", iteration.ID)
			}
			if _, duplicate := stepIDs[step.ID]; duplicate {
				return fmt.Errorf("iteration %s has duplicate step id %q", iteration.ID, step.ID)
			}
			if _, duplicate := stepRanks[step.Rank]; duplicate {
				return fmt.Errorf("iteration %s has duplicate step rank %d", iteration.ID, step.Rank)
			}
			stepIDs[step.ID] = struct{}{}
			stepRanks[step.Rank] = struct{}{}
		}
	}
	return nil
}

func equalStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
