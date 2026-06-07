package testrepo

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ImportPreview is what a freshly-parsed import file looks like before mapping
// (FR-10.5): its column headers and the number of data rows.
type ImportPreview struct {
	Headers  []string `json:"headers"`
	RowCount int      `json:"rowCount"`
}

// ImportMapping maps Test fields to spreadsheet column headers (FR-10.4). An
// empty value means the field is unmapped. Summary is required.
type ImportMapping struct {
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Labels      string `json:"labels"`
	Folder      string `json:"folder"`
}

// ImportError is one row that failed validation (FR-10.5).
type ImportError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// ImportResult reports an import (or dry-run) outcome (FR-10.5 / 10.6).
type ImportResult struct {
	Created int           `json:"created"`
	Skipped int           `json:"skipped"`
	Errors  []ImportError `json:"errors"`
}

// testCreatePayload is the JSON stored in a test_create pending row.
type testCreatePayload struct {
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Labels      string `json:"labels"`
	Folder      string `json:"folder"`
}

// ParseImportPreview reads the header row and counts data rows of a CSV file
// (FR-10.2 / 10.5).
func (r *Repository) ParseImportPreview(content string) (ImportPreview, error) {
	records, err := readCSV(content)
	if err != nil {
		return ImportPreview{}, err
	}
	if len(records) == 0 {
		return ImportPreview{}, fmt.Errorf("the file is empty")
	}
	return ImportPreview{Headers: records[0], RowCount: len(records) - 1}, nil
}

// ImportTests validates a CSV import against a column mapping and, unless
// dryRun, creates a local pending Test for each valid row (FR-10.2 / 10.4 /
// 10.5 / 10.6). Each created Test gets a temporary "NEW-N" key until commit
// assigns the real one. Invalid rows are reported and skipped, not fatal.
func (r *Repository) ImportTests(profileID, content string, mapping ImportMapping, dryRun bool) (ImportResult, error) {
	result := ImportResult{Errors: []ImportError{}}

	records, err := readCSV(content)
	if err != nil {
		return result, err
	}
	if len(records) < 2 {
		return result, fmt.Errorf("the file has no data rows")
	}
	header := records[0]
	col := func(name string) int {
		if name == "" {
			return -1
		}
		for i, h := range header {
			if strings.EqualFold(strings.TrimSpace(h), strings.TrimSpace(name)) {
				return i
			}
		}
		return -1
	}
	summaryIdx := col(mapping.Summary)
	if summaryIdx < 0 {
		return result, fmt.Errorf("the Summary field must be mapped to a column")
	}
	descIdx := col(mapping.Description)
	prioIdx := col(mapping.Priority)
	labelsIdx := col(mapping.Labels)
	folderIdx := col(mapping.Folder)

	get := func(row []string, idx int) string {
		if idx < 0 || idx >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[idx])
	}

	var tx *sql.Tx
	if !dryRun {
		tx, err = r.db.Begin()
		if err != nil {
			return result, fmt.Errorf("begin transaction: %w", err)
		}
		defer func() { _ = tx.Rollback() }()
	}

	for i := 1; i < len(records); i++ {
		rowNum := i + 1 // 1-based, counting the header row
		summary := get(records[i], summaryIdx)
		if summary == "" {
			result.Errors = append(result.Errors, ImportError{Row: rowNum, Message: "summary is empty"})
			result.Skipped++
			continue
		}
		if dryRun {
			result.Created++
			continue
		}

		payload := testCreatePayload{
			Summary:     summary,
			Description: get(records[i], descIdx),
			Priority:    get(records[i], prioIdx),
			Labels:      get(records[i], labelsIdx),
			Folder:      get(records[i], folderIdx),
		}
		if err := insertImportedTest(tx, profileID, payload); err != nil {
			return result, err
		}
		result.Created++
	}

	if !dryRun {
		if err := tx.Commit(); err != nil {
			return result, fmt.Errorf("commit import: %w", err)
		}
	}
	return result, nil
}

// insertImportedTest creates one pending Test from an import row.
func insertImportedTest(tx *sql.Tx, profileID string, p testCreatePayload) error {
	tempKey, err := nextTempTestKey(tx, profileID)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO test_case
		   (profile_id, jira_key, jira_id, summary, description, status, priority, labels, updated_at, folder_id)
		 VALUES (?, ?, '', ?, ?, '', ?, ?, '', ?)`,
		profileID, tempKey, p.Summary, p.Description, p.Priority, p.Labels, p.Folder,
	); err != nil {
		return fmt.Errorf("insert imported test: %w", err)
	}
	encoded, _ := json.Marshal(p)
	if err := upsertPendingChange(
		tx, profileID, entityTestCreate, tempKey, "test", "", string(encoded), "",
	); err != nil {
		return err
	}
	return writeAudit(
		tx, profileID, entityTestCreate, tempKey, "import-test-local", "test", "", p.Summary, "",
	)
}

// nextTempTestKey returns a Test key of the form "NEW-N" not already used in
// this profile.
func nextTempTestKey(tx *sql.Tx, profileID string) (string, error) {
	for n := 1; ; n++ {
		key := fmt.Sprintf("NEW-%d", n)
		var one int
		err := tx.QueryRow(
			`SELECT 1 FROM test_case WHERE profile_id = ? AND jira_key = ?`, profileID, key,
		).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return key, nil
		}
		if err != nil {
			return "", fmt.Errorf("probe temp test key: %w", err)
		}
	}
}

// RenameTest rewrites a Test's key across the cache, used by the commit path to
// swap a "NEW-N" placeholder for the real key Jira assigned. A no-op when
// newKey is empty or unchanged.
func (r *Repository) RenameTest(profileID, oldKey, newKey string) error {
	if newKey == "" || newKey == oldKey {
		return nil
	}
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, stmt := range []struct {
		table, keyCol string
	}{
		{"test_case", "jira_key"},
		{"test_step", "test_key"},
		{"test_precondition", "test_key"},
		{"test_container_test", "test_key"},
	} {
		if _, err := tx.Exec(
			fmt.Sprintf(`UPDATE %s SET %s = ? WHERE profile_id = ? AND %s = ?`,
				stmt.table, stmt.keyCol, stmt.keyCol),
			newKey, profileID, oldKey,
		); err != nil {
			return fmt.Errorf("rename test in %s: %w", stmt.table, err)
		}
	}
	return tx.Commit()
}

// readCSV parses CSV content leniently (variable field counts allowed).
func readCSV(content string) ([][]string, error) {
	reader := csv.NewReader(strings.NewReader(content))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse CSV: %w", err)
	}
	return records, nil
}

// ImportTemplateCSV returns a starter CSV with the supported columns (FR-10.3).
func ImportTemplateCSV() string {
	return "Summary,Description,Priority,Labels,Folder\n" +
		"Login with valid credentials,Verify a user can log in,High,smoke api,/Authentication/Login\n"
}
