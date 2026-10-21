package issuerepo

import (
	"context"
	"fmt"
)

// FilterOptions is what a view's filter bar offers for the two fields whose
// values are the project's own rather than a list TAM knows up front. Issue
// types come from ListProjectTypes, which asks Jira; these come from the
// cached rows, the way SprintRef does, so the bar never offers a status or a
// person the view cannot show.
//
// Neither list carries an empty entry. A blank option is unreadable beside
// the "All" one, and for the assignee the bar offers Unassigned itself.
type FilterOptions struct {
	Statuses  []string `json:"statuses"`
	Assignees []string `json:"assignees"`
}

// ListFilterOptions returns the distinct statuses and assignee display names
// this profile's cached issues carry, each sorted by name.
func (r *Repository) ListFilterOptions(ctx context.Context, profileID string) (FilterOptions, error) {
	statuses, err := r.distinct(ctx, profileID, "status")
	if err != nil {
		return FilterOptions{}, err
	}
	assignees, err := r.distinct(ctx, profileID, "assignee")
	if err != nil {
		return FilterOptions{}, err
	}
	return FilterOptions{Statuses: statuses, Assignees: assignees}, nil
}

// distinct reads one column's non-empty values for a profile. column is never
// user input: the two callers above name it literally, which is what lets it
// be interpolated where a host parameter cannot go.
func (r *Repository) distinct(ctx context.Context, profileID, column string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT `+column+` FROM issue WHERE profile_id = ? AND `+column+` <> '' ORDER BY `+column+` COLLATE NOCASE`,
		profileID)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", column, err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// inClause is the OR within one filter field: a placeholder list and the
// values to go with it. An empty values slice returns an empty clause, which
// the caller reads as "this field is not filtered".
//
// An empty string among the values is a real value, not an absence: for the
// assignee it is how the bar asks for unassigned work.
func inClause(column string, values []string) (string, []any) {
	if len(values) == 0 {
		return "", nil
	}
	marks := make([]byte, 0, len(values)*3)
	args := make([]any, 0, len(values))
	for i, v := range values {
		if i > 0 {
			marks = append(marks, ',', ' ')
		}
		marks = append(marks, '?')
		args = append(args, v)
	}
	return column + " IN (" + string(marks) + ")", args
}
