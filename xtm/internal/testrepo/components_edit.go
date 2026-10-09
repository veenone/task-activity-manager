package testrepo

import (
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
