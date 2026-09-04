// Package sharedmigrate moves XTM's profiles, connections, and settings out of
// its own database and into the shared profile database the suite's apps
// read. It runs once per shared file: after the copy it records a marker in
// the shared meta table and never copies again, so the shared file becomes
// the only place these rows are edited. The rows in XTM's own database are
// left in place as a backup and are no longer read.
package sharedmigrate

import (
	"database/sql"
	"errors"
	"fmt"
)

const markerKey = "xtm_profiles_imported"

// ImportFromStore copies profiles, connection, and app_setting rows from src
// (XTM's store) into dst (the shared database) unless dst already carries the
// import marker. Rows that already exist in dst are kept.
func ImportFromStore(src, dst *sql.DB) error {
	done, err := imported(dst)
	if err != nil {
		return err
	}
	if done {
		return nil
	}
	tx, err := dst.Begin()
	if err != nil {
		return fmt.Errorf("begin import: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := copyRows(tx, src,
		`SELECT id, name, jira_url, project_key, created_at, scope_jql, bug_issue_type, bug_project_mode, bug_project_key, ca_cert, allow_untrusted_tls, backend, cross_project_sources FROM profiles`,
		`INSERT OR IGNORE INTO profiles (id, name, jira_url, project_key, created_at, scope_jql, bug_issue_type, bug_project_mode, bug_project_key, ca_cert, allow_untrusted_tls, backend, cross_project_sources) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		13); err != nil {
		return fmt.Errorf("copy profiles: %w", err)
	}
	if err := copyRows(tx, src,
		`SELECT id, workspace_id, name, backend, url, project_key, scope_jql, bug_issue_type, bug_project_mode, bug_project_key, ca_cert, allow_untrusted_tls, role, created_at FROM connection`,
		`INSERT OR IGNORE INTO connection (id, workspace_id, name, backend, url, project_key, scope_jql, bug_issue_type, bug_project_mode, bug_project_key, ca_cert, allow_untrusted_tls, role, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		14); err != nil {
		return fmt.Errorf("copy connections: %w", err)
	}
	if err := copyRows(tx, src,
		`SELECT key, value FROM app_setting`,
		`INSERT OR IGNORE INTO app_setting (key, value) VALUES (?, ?)`,
		2); err != nil {
		return fmt.Errorf("copy settings: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO meta (key, value) VALUES (?, '1') ON CONFLICT(key) DO UPDATE SET value = '1'`,
		markerKey,
	); err != nil {
		return fmt.Errorf("record import marker: %w", err)
	}
	return tx.Commit()
}

func imported(dst *sql.DB) (bool, error) {
	var v string
	err := dst.QueryRow(`SELECT value FROM meta WHERE key = ?`, markerKey).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read import marker: %w", err)
	}
	return v == "1", nil
}

// copyRows streams every row of selectSQL from src into insertSQL on tx. cols
// is the column count shared by both statements.
func copyRows(tx *sql.Tx, src *sql.DB, selectSQL, insertSQL string, cols int) error {
	rows, err := src.Query(selectSQL)
	if err != nil {
		return err
	}
	defer rows.Close()
	stmt, err := tx.Prepare(insertSQL)
	if err != nil {
		return err
	}
	defer stmt.Close()
	vals := make([]any, cols)
	ptrs := make([]any, cols)
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		if _, err := stmt.Exec(vals...); err != nil {
			return err
		}
	}
	return rows.Err()
}
