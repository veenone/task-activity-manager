package demo

import (
	"context"
	"fmt"
	"strings"

	"agile-suite/tam/internal/backend"
)

var _ backend.FilterBackend = (*Backend)(nil)

// demoFilters are the saved filters the demo instance pretends its user can
// reach. They are curated rather than derived, the way the rest of this
// dataset is: a filter is something a person wrote.
//
// One of them is owned and the other two are shared, so the picker draws
// both of its groups here without a live Jira behind it. The owned one is
// the case #346 was about, a filter its author never starred, which on a
// real instance the favourites read cannot see.
var demoFilters = []backend.Filter{
	{ID: "10102", Name: "What I am working on", JQL: "project = PLAT AND assignee = currentUser()", Scope: backend.FilterScopeOwned},
	{ID: "10100", Name: "Everything in this project", JQL: "project = PLAT", Scope: backend.FilterScopeShared},
	{ID: "10101", Name: "Open bugs", JQL: "project = PLAT AND type = Bug AND status != Done", Scope: backend.FilterScopeShared},
}

// Filters is that list.
func (b *Backend) Filters(context.Context) ([]backend.Filter, error) {
	return append([]backend.Filter{}, demoFilters...), nil
}

// FilterByID resolves one of those three by id and refuses anything else,
// so the demo exercises both halves of the by-id box: the filter the
// picker could not have offered, and the id nobody can see.
func (b *Backend) FilterByID(_ context.Context, id string) (backend.Filter, error) {
	want := strings.TrimSpace(id)
	for _, f := range demoFilters {
		if f.ID == want {
			return f, nil
		}
	}
	return backend.Filter{}, fmt.Errorf("no filter with id %s, or this account cannot see it", want)
}

// JiraDashboards is empty, which is the honest answer. A demo profile has
// no Jira behind it, and the list exists only to open a dashboard in a
// browser, so a made-up row would be a link to nowhere.
func (b *Backend) JiraDashboards(context.Context) ([]backend.JiraDashboard, error) {
	return []backend.JiraDashboard{}, nil
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
