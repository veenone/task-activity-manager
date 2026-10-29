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
	args := []any{profileID, run.ExecutionKey, run.TestKey}
	if _, err := tx.Exec(
		`DELETE FROM test_run_iteration_step WHERE profile_id = ? AND exec_key = ? AND test_key = ?`, args...,
	); err != nil {
		return fmt.Errorf("clear iteration steps: %w", err)
	}
	if _, err := tx.Exec(
		`DELETE FROM test_run_iteration WHERE profile_id = ? AND exec_key = ? AND test_key = ?`, args...,
	); err != nil {
		return fmt.Errorf("clear iterations: %w", err)
	}
	for _, iteration := range run.Iterations {
		parametersJSON, err := json.Marshal(iteration.Parameters)
		if err != nil {
			return fmt.Errorf("encode iteration %s parameters: %w", iteration.ID, err)
		}
		if _, err := tx.Exec(
			`INSERT INTO test_run_iteration
			 (profile_id, exec_key, test_key, iteration_id, run_id, rank, status, run_status,
			  dataset_source_kind, dataset_source_key, parameters_json)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			profileID, run.ExecutionKey, run.TestKey, iteration.ID, run.ID, iteration.Rank,
			iteration.Status, run.Status, run.DatasetSourceKind, run.DatasetSourceKey, string(parametersJSON),
		); err != nil {
			return fmt.Errorf("insert iteration %s: %w", iteration.ID, err)
		}
		for _, step := range iteration.Steps {
			if _, err := tx.Exec(
				`INSERT INTO test_run_iteration_step
				 (profile_id, exec_key, test_key, iteration_id, step_result_id, step_definition_id,
				  rank, status, action, data, expected)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				profileID, run.ExecutionKey, run.TestKey, iteration.ID, step.ID, step.DefinitionID,
				step.Rank, step.Status, step.Action, step.Data, step.Expected,
			); err != nil {
				return fmt.Errorf("insert iteration step %s: %w", step.ID, err)
			}
		}
	}
	return tx.Commit()
}

func (r *Repository) ParameterizedRun(profileID, execKey, testKey string) (backend.ParameterizedRun, bool, error) {
	rows, err := r.db.Query(
		`SELECT iteration_id, run_id, rank, status, run_status, dataset_source_kind,
		        dataset_source_key, parameters_json
		 FROM test_run_iteration
		 WHERE profile_id = ? AND exec_key = ? AND test_key = ?
		 ORDER BY rank`,
		profileID, execKey, testKey,
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
			&run.DatasetSourceKind, &run.DatasetSourceKey, &parametersJSON,
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
		`SELECT step_result_id, step_definition_id, rank, status, action, data, expected
		 FROM test_run_iteration_step
		 WHERE profile_id = ? AND exec_key = ? AND test_key = ? AND iteration_id = ?
		 ORDER BY rank`,
		profileID, execKey, testKey, iterationID,
	)
	if err != nil {
		return nil, fmt.Errorf("query iteration steps: %w", err)
	}
	defer rows.Close()
	steps := []backend.IterationStep{}
	for rows.Next() {
		var step backend.IterationStep
		if err := rows.Scan(&step.ID, &step.DefinitionID, &step.Rank, &step.Status, &step.Action, &step.Data, &step.Expected); err != nil {
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
	entityKey := strings.Join([]string{execKey, testKey, iterationID}, ":")
	query := `SELECT status FROM test_run_iteration WHERE profile_id = ? AND exec_key = ? AND test_key = ? AND iteration_id = ?`
	update := `UPDATE test_run_iteration SET status = ? WHERE profile_id = ? AND exec_key = ? AND test_key = ? AND iteration_id = ?`
	selectArgs := []any{profileID, execKey, testKey, iterationID}
	if stepResultID != "" {
		table = "test_run_iteration_step"
		entityType = entityTestRunIterStep
		entityKey = strings.Join([]string{execKey, testKey, iterationID, stepResultID}, ":")
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
