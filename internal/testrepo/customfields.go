package testrepo

import (
	"database/sql"
	"errors"
	"fmt"
)

// CustomFieldDef mirrors a Jira custom field on the Test issue type (FR-2.6).
type CustomFieldDef struct {
	FieldID string `json:"fieldId"`
	Name    string `json:"name"`
	Type    string `json:"type"`
}

// CustomFieldValue is a custom field definition joined with a Test's value for
// it — what the detail panel renders and edits.
type CustomFieldValue struct {
	FieldID string `json:"fieldId"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Value   string `json:"value"`
}

// UpsertCustomFields stores the custom field definitions for a profile (FR-2.6).
func (r *Repository) UpsertCustomFields(profileID string, defs []CustomFieldDef) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(
		`INSERT INTO custom_field (profile_id, field_id, name, type)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(profile_id, field_id) DO UPDATE SET
		   name = excluded.name, type = excluded.type`)
	if err != nil {
		return fmt.Errorf("prepare upsert custom field: %w", err)
	}
	defer stmt.Close()

	for _, d := range defs {
		if _, err := stmt.Exec(profileID, d.FieldID, d.Name, d.Type); err != nil {
			return fmt.Errorf("upsert custom field %s: %w", d.FieldID, err)
		}
	}
	return tx.Commit()
}

// ListCustomFieldDefs returns the custom field definitions for a profile.
func (r *Repository) ListCustomFieldDefs(profileID string) ([]CustomFieldDef, error) {
	rows, err := r.db.Query(
		`SELECT field_id, name, type FROM custom_field WHERE profile_id = ? ORDER BY name`,
		profileID)
	if err != nil {
		return nil, fmt.Errorf("list custom field defs: %w", err)
	}
	defer rows.Close()

	out := []CustomFieldDef{}
	for rows.Next() {
		var d CustomFieldDef
		if err := rows.Scan(&d.FieldID, &d.Name, &d.Type); err != nil {
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
		`SELECT d.field_id, d.name, d.type, COALESCE(v.value, '')
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
		if err := rows.Scan(&v.FieldID, &v.Name, &v.Type, &v.Value); err != nil {
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

	var defExists int
	if err := tx.QueryRow(
		`SELECT 1 FROM custom_field WHERE profile_id = ? AND field_id = ?`,
		profileID, fieldID,
	).Scan(&defExists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("unknown custom field %q", fieldID)
		}
		return fmt.Errorf("read custom field def: %w", err)
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
