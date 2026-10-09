package testrepo

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

const maxComponentLen = 255

// validateComponentName applies the rules Jira enforces on a component name,
// plus no newline, which the stored encoding uses as its separator.
func validateComponentName(n string) error {
	switch {
	case strings.TrimSpace(n) == "":
		return fmt.Errorf("component name is empty")
	case strings.ContainsAny(n, "\r\n"):
		return fmt.Errorf("component %q contains a line break", n)
	case len([]rune(n)) > maxComponentLen:
		return fmt.Errorf("component %q is longer than %d characters", n, maxComponentLen)
	}
	return nil
}

// cleanComponents validates, trims and dedupes names, keeping first-seen order.
func cleanComponents(names []string) ([]string, error) {
	out := make([]string, 0, len(names))
	seen := map[string]bool{}
	for _, n := range names {
		if err := validateComponentName(n); err != nil {
			return nil, err
		}
		n = strings.TrimSpace(n)
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out, nil
}

// SetTestComponents replaces one test's components, queueing a single
// pending edit. Setting the same list again queues nothing.
func (r *Repository) SetTestComponents(profileID, testKey string, names []string) error {
	clean, err := cleanComponents(names)
	if err != nil {
		return fmt.Errorf("set components on %s: %w", testKey, err)
	}
	var current string
	if err := r.db.QueryRow(
		`SELECT components FROM test_case WHERE profile_id = ? AND jira_key = ?`,
		profileID, testKey).Scan(&current); err != nil {
		return fmt.Errorf("set components on %s: %w", testKey, err)
	}
	next := encodeComponents(clean)
	if next == current {
		return nil
	}
	return r.EditTestField(profileID, testKey, "components", next)
}

// BulkEditComponents changes components across tests, one pending edit per
// test that changes. Without replace it removes then adds; with replace each
// test's components become add, and an empty add clears them.
func (r *Repository) BulkEditComponents(profileID string, testKeys, add, remove []string, replace bool) (BulkEditResult, error) {
	result := BulkEditResult{Succeeded: []string{}, Failed: []BulkFailure{}}
	addClean, err := cleanComponents(add)
	if err != nil {
		return result, fmt.Errorf("bulk components: %w", err)
	}
	removeClean, err := cleanComponents(remove)
	if err != nil {
		return result, fmt.Errorf("bulk components: %w", err)
	}
	switch {
	case replace && len(removeClean) > 0:
		return result, fmt.Errorf("bulk components: replace takes no remove list")
	case !replace && len(addClean) == 0 && len(removeClean) == 0:
		return result, fmt.Errorf("bulk components: nothing to add or remove")
	}
	adding := map[string]bool{}
	for _, n := range addClean {
		adding[n] = true
	}
	for _, n := range removeClean {
		if adding[n] {
			return result, fmt.Errorf("bulk components: %q is in both add and remove", n)
		}
	}

	for _, key := range testKeys {
		var current string
		err := r.db.QueryRow(
			`SELECT components FROM test_case WHERE profile_id = ? AND jira_key = ?`,
			profileID, key).Scan(&current)
		if errors.Is(err, sql.ErrNoRows) {
			result.Failed = append(result.Failed, BulkFailure{TestKey: key, Error: "not found"})
			continue
		}
		if err != nil {
			result.Failed = append(result.Failed, BulkFailure{TestKey: key, Error: err.Error()})
			continue
		}
		var next []string
		if replace {
			next = addClean
		} else {
			next = addLabels(removeLabels(decodeComponents(current), removeClean), addClean)
		}
		encoded := encodeComponents(next)
		if encoded == current {
			result.Succeeded = append(result.Succeeded, key)
			continue
		}
		if err := r.EditTestField(profileID, key, "components", encoded); err != nil {
			result.Failed = append(result.Failed, BulkFailure{TestKey: key, Error: err.Error()})
			continue
		}
		result.Succeeded = append(result.Succeeded, key)
	}
	return result, nil
}

// ListTestComponents returns each requested test's components, keyed by Jira
// key, for the Bulk Components preview. Unknown keys are omitted.
func (r *Repository) ListTestComponents(profileID string, testKeys []string) (map[string][]string, error) {
	out := make(map[string][]string, len(testKeys))
	for _, key := range testKeys {
		var stored string
		err := r.db.QueryRow(
			`SELECT components FROM test_case WHERE profile_id = ? AND jira_key = ?`,
			profileID, key).Scan(&stored)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("list test components %s: %w", key, err)
		}
		out[key] = decodeComponents(stored)
	}
	return out, nil
}
