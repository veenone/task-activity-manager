package testrepo

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// DatasetParameter is one column of a test's dataset: a parameter that a
// ${placeholder} in the test's steps or definition refers to (#267).
type DatasetParameter struct {
	Name          string `json:"name"`
	Combinatorial bool   `json:"combinatorial"`
}

// Dataset is a test's parameter table: parameters as columns, one row per
// iteration. An empty Dataset means the test has none.
type Dataset struct {
	Parameters []DatasetParameter `json:"parameters"`
	Rows       [][]string         `json:"rows"`
}

// GetTestDataset returns the cached dataset for a test and whether one has
// been fetched at all.
//
// The second return is the whole point of the table. An empty dataset is a
// real answer, not a miss: most tests are not parameterized, so treating "no
// parameters" as "not cached" would send the detail panel back to Jira every
// time any ordinary test is opened. Callers refetch on !found, not on empty.
//
// contextKey is "" for the test's own dataset.
func (r *Repository) GetTestDataset(profileID, testKey, contextKey string) (Dataset, bool, error) {
	var paramsJSON, rowsJSON string
	err := r.db.QueryRow(
		`SELECT parameters_json, rows_json FROM test_dataset
		   WHERE profile_id = ? AND test_key = ? AND context_key = ?`,
		profileID, testKey, contextKey,
	).Scan(&paramsJSON, &rowsJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return Dataset{}, false, nil
	}
	if err != nil {
		return Dataset{}, false, fmt.Errorf("read dataset for %s: %w", testKey, err)
	}

	ds := Dataset{}
	if err := json.Unmarshal([]byte(paramsJSON), &ds.Parameters); err != nil {
		return Dataset{}, false, fmt.Errorf("decode dataset parameters for %s: %w", testKey, err)
	}
	if err := json.Unmarshal([]byte(rowsJSON), &ds.Rows); err != nil {
		return Dataset{}, false, fmt.Errorf("decode dataset rows for %s: %w", testKey, err)
	}
	return ds, true, nil
}

// SaveTestDataset records a test's dataset, replacing whatever was cached.
//
// Save an empty Dataset to record that the test has none; that row is what
// stops the next open going back to Jira. The whole table is one row, so a
// dataset that shrank in Jira cannot leave stale rows behind.
func (r *Repository) SaveTestDataset(profileID, testKey, contextKey string, ds Dataset) error {
	params := ds.Parameters
	if params == nil {
		params = []DatasetParameter{}
	}
	rows := ds.Rows
	if rows == nil {
		rows = [][]string{}
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("encode dataset parameters for %s: %w", testKey, err)
	}
	rowsJSON, err := json.Marshal(rows)
	if err != nil {
		return fmt.Errorf("encode dataset rows for %s: %w", testKey, err)
	}
	if _, err := r.db.Exec(
		`INSERT INTO test_dataset
		   (profile_id, test_key, context_key, parameters_json, rows_json, fetched_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(profile_id, test_key, context_key) DO UPDATE SET
		   parameters_json = excluded.parameters_json,
		   rows_json       = excluded.rows_json,
		   fetched_at      = excluded.fetched_at`,
		profileID, testKey, contextKey, string(paramsJSON), string(rowsJSON),
		time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		return fmt.Errorf("save dataset for %s: %w", testKey, err)
	}
	return nil
}
