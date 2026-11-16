package testrepo

import (
	"fmt"

	"agile-suite/core/journal"
)

// queryColumn runs a query that selects one column and returns its values in
// row order. The slice is empty, never nil, when no row matches. what names
// the read in the error a failed query returns.
func queryColumn[T any](q journal.Querier, what, query string, args ...any) ([]T, error) {
	rows, err := q.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		var v T
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// keySet turns a list of keys into a set.
func keySet(keys []string) map[string]bool {
	set := make(map[string]bool, len(keys))
	for _, k := range keys {
		set[k] = true
	}
	return set
}
