package xray

import (
	"context"
	"time"

	"agile-suite/xtm/internal/backend"
)

var _ backend.CreatedIssueFinder = (*Adapter)(nil)

// FindCreatedIssues implements backend.CreatedIssueFinder with one JQL search.
func (a *Adapter) FindCreatedIssues(ctx context.Context, projectKey, kind, issueType string, since time.Time) ([]backend.IssueRef, error) {
	found, err := a.c.FindCreatedIssues(ctx, projectKey, kind, issueType, since)
	if err != nil {
		return nil, err
	}
	out := make([]backend.IssueRef, 0, len(found))
	for _, f := range found {
		out = append(out, backend.IssueRef{Key: f.Key, Summary: f.Summary})
	}
	return out, nil
}
