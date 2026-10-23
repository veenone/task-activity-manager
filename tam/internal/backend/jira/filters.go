package jira

import (
	"context"
	"errors"

	corejira "agile-suite/core/jira"

	"agile-suite/tam/internal/backend"
)

// Nothing calls these outside the Dashboards view, which reaches them
// through a type assertion, so drift here would fail that view against a
// real Jira with nothing in the build to say so. This line fails the
// build instead.
var _ backend.FilterBackend = (*Backend)(nil)

// Filters is the saved filters this user can pin, with the JQL each one
// stands for: the ones they own, then the ones they starred that somebody
// else wrote.
//
// Both lists are read because neither is the whole set. Jira does not star
// a filter because you created it, so the favourites leave out everything a
// person wrote and never pinned, which is what #346 was raised for. Which
// list a filter arrived in is also the scope, so nothing here has to ask
// Jira who the connected user is or compare owners.
//
// An instance with no owned-filter route answers the starred list alone
// with no scope on it, which is exactly what this view did before. A
// failure reading either list is still a failure.
func (b *Backend) Filters(ctx context.Context) ([]backend.Filter, error) {
	owned, err := b.c.OwnedFilters(ctx)
	scoped := true
	if errors.Is(err, corejira.ErrNoOwnedFilters) {
		owned, scoped = nil, false
	} else if err != nil {
		return nil, err
	}
	starred, err := b.c.FavouriteFilters(ctx)
	if err != nil {
		return nil, err
	}

	// Owned first, and the id is what dedupes: starring your own filter is
	// normal and must not list it twice. Owning it is the stronger fact of
	// the two, so the owned pass claims the id and the starred pass skips
	// whatever is already spoken for.
	out := make([]backend.Filter, 0, len(owned)+len(starred))
	seen := make(map[string]bool, len(owned)+len(starred))
	add := func(f corejira.RawFilter, scope string) {
		if seen[f.ID] {
			return
		}
		seen[f.ID] = true
		out = append(out, backend.Filter{ID: f.ID, Name: f.Name, JQL: f.JQL, Scope: scope})
	}
	for _, f := range owned {
		add(f, backend.FilterScopeOwned)
	}
	shared := backend.FilterScopeShared
	if !scoped {
		shared = ""
	}
	for _, f := range starred {
		add(f, shared)
	}
	return out, nil
}

// SearchByJQL runs a filter's own words and maps the hits onto rows.
//
// The JQL is sent unchanged. Everywhere else in TAM a search is scoped to
// the profile's project, because everywhere else the question is about
// that project; a saved filter is the user's own question and may well
// span projects, so narrowing it here would answer something they did not
// ask. Each hit resolves its own project's types for that reason too.
func (b *Backend) SearchByJQL(ctx context.Context, jql string, startAt, maxResults int) ([]backend.Issue, int, error) {
	ids := b.discover(ctx)
	fields := append(append([]string{}, baseFields...), ids.list()...)
	page, err := b.c.SearchIssues(ctx, jql, fields, nil, startAt, maxResults)
	if err != nil {
		return nil, 0, err
	}
	out := make([]backend.Issue, 0, len(page.Issues))
	for _, raw := range page.Issues {
		pt := b.typesOrEmpty(ctx, projectOf(raw.Key))
		out = append(out, parseIssue(raw, ids, b.requirementType, pt))
	}
	return out, page.Total, nil
}
