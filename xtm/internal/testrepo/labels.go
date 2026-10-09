package testrepo

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// maxLabelLen is Jira's limit on a single label.
const maxLabelLen = 255

// ListLabels returns the distinct labels across a profile's Tests with a count
// each, sorted by label. Jira labels are case-sensitive, so "Smoke" and
// "smoke" are separate entries. It backs the label picker's suggestions.
func (r *Repository) ListLabels(profileID string) ([]Bucket, error) {
	rows, err := r.db.Query(
		`SELECT labels FROM test_case WHERE profile_id = ? AND labels <> ''`,
		profileID)
	if err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var stored string
		if err := rows.Scan(&stored); err != nil {
			return nil, err
		}
		for _, l := range strings.Fields(stored) {
			counts[l]++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]Bucket, 0, len(counts))
	for l, n := range counts {
		out = append(out, Bucket{Label: l, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out, nil
}

// BulkEditLabels adds and removes labels on each given Test, queueing one
// "labels" field edit per Test whose labels change. Tests that would not
// change are reported as succeeded without a pending change. Each Test is
// edited on its own through EditTestField, as BulkEditTests does, so one
// failure does not block the others.
func (r *Repository) BulkEditLabels(profileID string, testKeys []string, add, remove []string) (BulkEditResult, error) {
	result := BulkEditResult{Succeeded: []string{}, Failed: []BulkFailure{}}
	if err := validateLabelEdit(add, remove); err != nil {
		return result, fmt.Errorf("bulk labels: %w", err)
	}

	for _, key := range testKeys {
		var current string
		err := r.db.QueryRow(
			`SELECT labels FROM test_case WHERE profile_id = ? AND jira_key = ?`,
			profileID, key,
		).Scan(&current)
		if errors.Is(err, sql.ErrNoRows) {
			result.Failed = append(result.Failed, BulkFailure{TestKey: key, Error: "not found"})
			continue
		}
		if err != nil {
			result.Failed = append(result.Failed, BulkFailure{TestKey: key, Error: err.Error()})
			continue
		}
		before := strings.Fields(current)
		next := strings.Join(addLabels(removeLabels(before, remove), add), " ")
		if next == strings.Join(before, " ") {
			result.Succeeded = append(result.Succeeded, key)
			continue
		}
		if err := r.EditTestField(profileID, key, "labels", next); err != nil {
			result.Failed = append(result.Failed, BulkFailure{TestKey: key, Error: err.Error()})
			continue
		}
		result.Succeeded = append(result.Succeeded, key)
	}
	return result, nil
}

// validateLabelEdit rejects an empty request, a label Jira would refuse, and
// a label that appears in both lists.
func validateLabelEdit(add, remove []string) error {
	if len(add) == 0 && len(remove) == 0 {
		return fmt.Errorf("nothing to add or remove")
	}
	adding := map[string]bool{}
	for _, l := range add {
		if err := validateLabel(l); err != nil {
			return err
		}
		adding[l] = true
	}
	for _, l := range remove {
		if err := validateLabel(l); err != nil {
			return err
		}
		if adding[l] {
			return fmt.Errorf("label %q is in both add and remove", l)
		}
	}
	return nil
}

func validateLabel(l string) error {
	if l == "" {
		return fmt.Errorf("label is empty")
	}
	if strings.IndexFunc(l, unicode.IsSpace) >= 0 {
		return fmt.Errorf("label %q contains whitespace", l)
	}
	if len([]rune(l)) > maxLabelLen {
		return fmt.Errorf("label %q is longer than %d characters", l, maxLabelLen)
	}
	return nil
}

// addLabels appends each label in add that labels does not already hold,
// keeping the existing order.
func addLabels(labels, add []string) []string {
	have := make(map[string]bool, len(labels))
	out := append([]string{}, labels...)
	for _, l := range labels {
		have[l] = true
	}
	for _, l := range add {
		if !have[l] {
			out = append(out, l)
			have[l] = true
		}
	}
	return out
}

// removeLabels drops every label in remove, keeping the order of the rest.
func removeLabels(labels, remove []string) []string {
	drop := make(map[string]bool, len(remove))
	for _, l := range remove {
		drop[l] = true
	}
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		if !drop[l] {
			out = append(out, l)
		}
	}
	return out
}
