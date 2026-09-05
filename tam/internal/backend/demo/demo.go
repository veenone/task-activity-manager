// Package demo serves the offline dataset through the IssueBackend seam, so
// a profile whose Jira URL is "demo" exercises the same sync, cache, and
// views as a live one.
package demo

import (
	"context"
	"fmt"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/demo"
)

// Backend is the demo IssueBackend for one project key.
type Backend struct {
	project string
}

// New returns the demo backend for projectKey.
func New(projectKey string) *Backend {
	if projectKey == "" {
		projectKey = demo.ProjectKey
	}
	return &Backend{project: projectKey}
}

// TestConnection always succeeds with the demo user.
func (b *Backend) TestConnection(context.Context) (backend.User, error) {
	return backend.User{Name: "demo", DisplayName: "Demo User"}, nil
}

// IsDemo is true.
func (b *Backend) IsDemo() bool { return true }

// SearchIssuesPage pages the dataset filtered by type. The scope JQL and
// since are ignored, as in XTM's demo mode, so an incremental sync still
// returns everything.
func (b *Backend) SearchIssuesPage(_ context.Context, _, _, _ string, types []string, startAt, maxResults int) ([]backend.Issue, int, error) {
	want := map[string]bool{}
	for _, t := range types {
		want[t] = true
	}
	var all []backend.Issue
	for _, iss := range demo.Issues(b.project) {
		if len(want) == 0 || want[iss.Type] {
			all = append(all, iss)
		}
	}
	total := len(all)
	if startAt >= total || maxResults <= 0 {
		return []backend.Issue{}, total, nil
	}
	end := startAt + maxResults
	if end > total {
		end = total
	}
	return all[startAt:end], total, nil
}

// GetIssueDetail returns the curated or generated detail for key.
func (b *Backend) GetIssueDetail(_ context.Context, key string) (backend.IssueDetail, error) {
	d, ok := demo.Detail(b.project, key)
	if !ok {
		return backend.IssueDetail{}, fmt.Errorf("demo: no issue %s", key)
	}
	return d, nil
}

// IssueTypes lists the five types the dataset uses.
func (b *Backend) IssueTypes(context.Context, string) ([]backend.IssueType, error) {
	return []backend.IssueType{
		{ID: "1", Name: "Task"},
		{ID: "2", Name: "Epic"},
		{ID: "3", Name: "Story"},
		{ID: "4", Name: "Bug"},
		{ID: "5", Name: "Requirement"},
	}, nil
}
