package testrepo

import (
	"database/sql"
	"errors"
	"fmt"

	"agile-suite/xtm/internal/backend"
)

// fieldScope normalises a definition's scope. A backend that reports none
// means the test issue, which is what every backend meant before the scope
// existed, and a row cached before the column existed reads the same way
// (I1: the value crosses a trust boundary from the adapter).
func fieldScope(s string) string {
	if s == "" {
		return backend.FieldScopeIssue
	}
	return s
}

// CustomFieldDef mirrors a custom field the backend reports (FR-2.6).
//
// Scope is one of the backend.FieldScope* values and says what the field is
// attached to. Only a field on the test issue can be edited here, because a
// custom field edit commits as a Jira issue update; a Zephyr profile also has
// fields on an execution and on a step (#371 Phase 3). An empty scope is
// stored as the issue scope, which is what every definition written before the
// column existed meant.
type CustomFieldDef struct {
	FieldID string `json:"fieldId"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Scope   string `json:"scope"`
}

// CustomFieldValue is a custom field definition joined with a Test's value for
// it — what the detail panel renders and edits.
type CustomFieldValue struct {
	FieldID string `json:"fieldId"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Value   string `json:"value"`
	Scope   string `json:"scope"`
}

// UpsertCustomFields stores the custom field definitions for a profile (FR-2.6).
func (r *Repository) UpsertCustomFields(profileID string, defs []CustomFieldDef) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(
		`INSERT INTO custom_field (profile_id, field_id, name, type, scope)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(profile_id, field_id) DO UPDATE SET
		   name = excluded.name, type = excluded.type, scope = excluded.scope`)
	if err != nil {
		return fmt.Errorf("prepare upsert custom field: %w", err)
	}
	defer stmt.Close()

	for _, d := range defs {
		if _, err := stmt.Exec(profileID, d.FieldID, d.Name, d.Type, fieldScope(d.Scope)); err != nil {
			return fmt.Errorf("upsert custom field %s: %w", d.FieldID, err)
		}
	}
	return tx.Commit()
}

// ListCustomFieldDefs returns the custom field definitions for a profile.
func (r *Repository) ListCustomFieldDefs(profileID string) ([]CustomFieldDef, error) {
	rows, err := r.db.Query(
		`SELECT field_id, name, type, scope FROM custom_field WHERE profile_id = ? ORDER BY name`,
		profileID)
	if err != nil {
		return nil, fmt.Errorf("list custom field defs: %w", err)
	}
	defer rows.Close()

	out := []CustomFieldDef{}
	for rows.Next() {
		var d CustomFieldDef
		if err := rows.Scan(&d.FieldID, &d.Name, &d.Type, &d.Scope); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// SetTestCustomFields replaces a Test's cached custom field values from a freshly
// fetched map (FR-2.6).
func (r *Repository) SetTestCustomFields(profileID, testKey string, values map[string]string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(
		`DELETE FROM test_custom_field WHERE profile_id = ? AND test_key = ?`,
		profileID, testKey,
	); err != nil {
		return fmt.Errorf("clear custom field values: %w", err)
	}
	for fieldID, value := range values {
		if _, err := tx.Exec(
			`INSERT INTO test_custom_field (profile_id, test_key, field_id, value)
			 VALUES (?, ?, ?, ?)`,
			profileID, testKey, fieldID, value,
		); err != nil {
			return fmt.Errorf("insert custom field value: %w", err)
		}
	}
	return tx.Commit()
}

// ListTestCustomFields returns a Test's custom fields (definition + value),
// including fields the Test has no value for, ordered by name.
func (r *Repository) ListTestCustomFields(profileID, testKey string) ([]CustomFieldValue, error) {
	rows, err := r.db.Query(
		`SELECT d.field_id, d.name, d.type, COALESCE(v.value, ''), d.scope
		 FROM custom_field d
		 LEFT JOIN test_custom_field v
		   ON v.profile_id = d.profile_id AND v.field_id = d.field_id AND v.test_key = ?
		 WHERE d.profile_id = ?
		 ORDER BY d.name`,
		testKey, profileID)
	if err != nil {
		return nil, fmt.Errorf("list test custom fields: %w", err)
	}
	defer rows.Close()

	out := []CustomFieldValue{}
	for rows.Next() {
		var v CustomFieldValue
		if err := rows.Scan(&v.FieldID, &v.Name, &v.Type, &v.Value, &v.Scope); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// HasCustomFieldValues reports whether a Test has any cached custom field
// values — used to decide whether a lazy fetch is needed.
func (r *Repository) HasCustomFieldValues(profileID, testKey string) (bool, error) {
	var one int
	err := r.db.QueryRow(
		`SELECT 1 FROM test_custom_field WHERE profile_id = ? AND test_key = ? LIMIT 1`,
		profileID, testKey,
	).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check custom field values: %w", err)
	}
	return true, nil
}

// EditTestCustomField applies a local edit to one custom field of a Test
// (FR-2.6), coalescing it into a pending change keyed by "<testKey>:<fieldId>".
// Commit pushes it as an issue field update. Reverting drops the change.
func (r *Repository) EditTestCustomField(profileID, testKey, fieldID, newValue string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var defName, defScope string
	if err := tx.QueryRow(
		`SELECT name, scope FROM custom_field WHERE profile_id = ? AND field_id = ?`,
		profileID, fieldID,
	).Scan(&defName, &defScope); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("unknown custom field %q", fieldID)
		}
		return fmt.Errorf("read custom field def: %w", err)
	}
	// Only a field on the test issue can be edited here. Commit pushes a
	// custom field change as a Jira issue field update, so staging one against
	// a field that lives on an execution or a step would queue a change Jira
	// can only reject (#371 Phase 3). The UI renders those read-only; this is
	// the guard behind it.
	if fieldScope(defScope) != backend.FieldScopeIssue {
		return fmt.Errorf("%s is not a field on the test, so it cannot be edited here", defName)
	}

	var baseVersion string
	if err := tx.QueryRow(
		`SELECT updated_at FROM test_case WHERE profile_id = ? AND jira_key = ?`,
		profileID, testKey,
	).Scan(&baseVersion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("read test: %w", err)
	}

	var currentVal string
	err = tx.QueryRow(
		`SELECT value FROM test_custom_field
		 WHERE profile_id = ? AND test_key = ? AND field_id = ?`,
		profileID, testKey, fieldID,
	).Scan(&currentVal)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read custom field value: %w", err)
	}
	if currentVal == newValue {
		return nil
	}

	if _, err := tx.Exec(
		`INSERT INTO test_custom_field (profile_id, test_key, field_id, value)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(profile_id, test_key, field_id) DO UPDATE SET value = excluded.value`,
		profileID, testKey, fieldID, newValue,
	); err != nil {
		return fmt.Errorf("update custom field value: %w", err)
	}

	ek := stepEntityKey(testKey, fieldID)
	if err := upsertPendingChange(
		tx, profileID, entityCustomField, ek, "value", currentVal, newValue, baseVersion,
	); err != nil {
		return err
	}
	if err := writeAudit(
		tx, profileID, entityCustomField, ek,
		"edit-custom-field-local", "value", currentVal, newValue, "",
	); err != nil {
		return err
	}
	return tx.Commit()
}
