package demo

import (
	"context"
	"strings"

	"agile-suite/tam/internal/backend"
)

var _ backend.FilterBackend = (*Backend)(nil)

// demoFilters are the saved filters the demo instance pretends its user
// has starred. They are curated rather than derived, the way the rest of
// this dataset is: a filter is something a person wrote, and two of them
// are enough to show what a dashboard is for.
var demoFilters = []backend.Filter{
	{ID: "10100", Name: "Everything in this project", JQL: "project = PLAT"},
	{ID: "10101", Name: "Open bugs", JQL: "project = PLAT AND type = Bug AND status != Done"},
}

// Filters is that list.
func (b *Backend) Filters(context.Context) ([]backend.Filter, error) {
	return append([]backend.Filter{}, demoFilters...), nil
}

// SearchByJQL answers the two shapes the demo filters are written in and
// nothing else: the whole project, and the project's open bugs. The demo
// has no JQL engine and should not pretend to one, so a filter it cannot
// read answers with the whole project rather than with a wrong subset,
// which is the honest failure for a dataset whose point is to show the
// view working.
func (b *Backend) SearchByJQL(_ context.Context, jql string, startAt, maxResults int) ([]backend.Issue, int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var all []backend.Issue
	bugsOnly := strings.Contains(strings.ToLower(jql), "type = bug")
	for _, iss := range b.issues() {
		if bugsOnly && (iss.Type != backend.TypeBug || iss.Status == "Done") {
			continue
		}
		all = append(all, iss)
	}
	total := len(all)
	if startAt >= total || maxResults <= 0 {
		return []backend.Issue{}, total, nil
	}
	end := startAt + maxResults
	if end > total {
		end = total
	}
	return append([]backend.Issue{}, all[startAt:end]...), total, nil
}
