package testrepo

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"agile-suite/core/journal"
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
	return saveTestDataset(r.db, profileID, testKey, contextKey, ds)
}

// saveTestDataset is SaveTestDataset against either the database or an open
// transaction, so a local edit can cache the table and journal it atomically.
func saveTestDataset(ex journal.Execer, profileID, testKey, contextKey string, ds Dataset) error {
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
	if _, err := ex.Exec(
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

// datasetField is the pending_change field name a dataset edit uses. There is
// only one, because the journal holds the whole table rather than a row or a
// cell: a dataset is small, the commit sends all of it anyway, and one row per
// cell would make the Pending changes dialog unreadable.
const datasetField = "dataset"

// EditTestDataset replaces a test's own dataset locally and queues it for
// commit (#277 W1). Jira is the system of record, so nothing is sent here.
//
// The before and after values are whole tables as JSON. Editing twice leaves
// one pending row still holding the table as it was synced, and editing back
// to that table drops the row, both from the journal's own coalescing.
func (r *Repository) EditTestDataset(profileID, testKey string, ds Dataset) error {
	before, _, err := r.GetTestDataset(profileID, testKey, "")
	if err != nil {
		return err
	}
	beforeJSON, err := encodeDataset(before)
	if err != nil {
		return err
	}
	afterJSON, err := encodeDataset(ds)
	if err != nil {
		return err
	}
	if beforeJSON == afterJSON {
		return nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := saveTestDataset(tx, profileID, testKey, "", ds); err != nil {
		return err
	}
	// Base version stays empty: a dataset carries no updated watermark of its
	// own, and the commit detects a remote change by reading the table back
	// and comparing it with the before value.
	if err := upsertPendingChange(
		tx, profileID, entityTestDataset, testKey, datasetField,
		beforeJSON, afterJSON, "",
	); err != nil {
		return err
	}
	if err := writeAudit(
		tx, profileID, entityTestDataset, testKey,
		"edit-dataset-local", datasetField, beforeJSON, afterJSON, "",
	); err != nil {
		return err
	}
	return tx.Commit()
}

// encodeDataset serialises a table for the journal. Nil slices are written as
// empty ones so a table that came back from the cache and one built in the UI
// encode identically, which is what makes the revert check work.
func encodeDataset(ds Dataset) (string, error) {
	if ds.Parameters == nil {
		ds.Parameters = []DatasetParameter{}
	}
	if ds.Rows == nil {
		ds.Rows = [][]string{}
	}
	b, err := json.Marshal(ds)
	if err != nil {
		return "", fmt.Errorf("encode dataset: %w", err)
	}
	return string(b), nil
}
