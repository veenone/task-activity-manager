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
	"strings"
)

const markerKey = "xtm_profiles_imported"

// Tables lists the shared tables in import order. Columns names every column
// the import copies for each; the drift test compares it with XTM's own
// table definitions.
var Tables = []string{"profiles", "connection", "app_setting"}

var Columns = map[string][]string{
	"profiles": {
		"id", "name", "jira_url", "project_key", "created_at", "scope_jql",
		"bug_issue_type", "bug_project_mode", "bug_project_key", "ca_cert",
		"allow_untrusted_tls", "backend", "cross_project_sources",
	},
	"connection": {
		"id", "workspace_id", "name", "backend", "url", "project_key", "scope_jql",
		"bug_issue_type", "bug_project_mode", "bug_project_key", "ca_cert",
		"allow_untrusted_tls", "role", "created_at",
	},
	"app_setting": {"key", "value"},
}

// copySQL builds the SELECT and INSERT OR IGNORE statements for one table from
// its column list.
func copySQL(table string) (selectSQL, insertSQL string, n int) {
	cols := Columns[table]
	list := strings.Join(cols, ", ")
	marks := strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", ")
	return "SELECT " + list + " FROM " + table,
		"INSERT OR IGNORE INTO " + table + " (" + list + ") VALUES (" + marks + ")",
		len(cols)
}

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

	for _, table := range Tables {
		selectSQL, insertSQL, n := copySQL(table)
		if err := copyRows(tx, src, selectSQL, insertSQL, n); err != nil {
			return fmt.Errorf("copy %s: %w", table, err)
		}
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
