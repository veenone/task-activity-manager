package testrepo

import (
	"database/sql"
	"errors"
	"fmt"
)

// bulkEditField edits one field on each of the given Tests. next computes the
// i-th Test's new value from its current one; returning the current value
// reports the Test as succeeded with nothing queued, and an error fails that
// Test alone. Each Test is edited on its own through EditTestField, so one
// failure does not block the others.
func (r *Repository) bulkEditField(profileID, field string, testKeys []string, next func(i int, current string) (string, error)) (BulkEditResult, error) {
	result := BulkEditResult{Succeeded: []string{}, Failed: []BulkFailure{}}
	col, ok := editableFields[field]
	if !ok {
		return result, fmt.Errorf("field %q is not editable", field)
	}
	readSQL := fmt.Sprintf(`SELECT %s FROM test_case WHERE profile_id = ? AND jira_key = ?`, col)

	for i, key := range testKeys {
		var current string
		err := r.db.QueryRow(readSQL, profileID, key).Scan(&current)
		if errors.Is(err, sql.ErrNoRows) {
			err = errors.New("not found")
		}
		var newVal string
		if err == nil {
			newVal, err = next(i, current)
		}
		if err == nil && newVal != current {
			err = r.EditTestField(profileID, key, field, newVal)
		}
		if err != nil {
			result.Failed = append(result.Failed, BulkFailure{TestKey: key, Error: err.Error()})
			continue
		}
		result.Succeeded = append(result.Succeeded, key)
	}
	return result, nil
}
