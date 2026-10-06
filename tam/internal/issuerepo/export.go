package issuerepo

import (
	"context"
	"fmt"

	"agile-suite/tam/internal/backend"
)

// ListIssuesForExport is every row the query matches, in the order the
// grid is showing them, with no paging.
//
// An export is of the filter and not of the page. The grid holds 25 rows
// and a planning session wants the whole backlog it narrowed to, so this
// shares ListIssues' filter and order and drops its limit alone. It does
// not expand families the way the grid does: a spreadsheet row is a row,
// and pulling in a parent nobody filtered for would put work in the file
// the filter excluded.
func (r *Repository) ListIssuesForExport(ctx context.Context, profileID string, q IssueQuery) ([]backend.Issue, error) {
	where, args := issueFilter(profileID, q)
	rows, err := r.db.QueryContext(ctx, `SELECT `+issueColumns+` FROM issue WHERE `+where+orderFor(q), args...)
	if err != nil {
		return nil, fmt.Errorf("list issues for export: %w", err)
	}
	defer rows.Close()
	out := []backend.Issue{}
	for rows.Next() {
		iss, err := scanIssue(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, iss)
	}
	return out, rows.Err()
}
